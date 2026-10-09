package orders

import (
	"context"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/gfotev/loadout/internal/money"
	"github.com/gfotev/loadout/internal/platform/db"
	"github.com/gfotev/loadout/internal/platform/validate"
)

// StaleAfter is how long an order may wait for the supplier's confirmation before the admin
// pages flag it.
const StaleAfter = 24 * time.Hour

const (
	MaxNoteLen       = 500
	maxAdminTextLen  = 200
	maxAdminPageSize = 100
)

// adminEvents are the events an admin applies by hand. The supplier's answer is theirs while
// orders are placed with the supplier by hand. refund_succeeded is for a refund Stripe couldn't
// make, after the admin returned the money another way.
// TODO(vali-api): supplier_confirmed and supplier_rejected come from the supplier's API once it
// takes orders; the admin keeps cancel.
var adminEvents = []Event{
	EventSupplierConfirmed,
	EventSupplierRejected,
	EventCancel,
	EventRefundSucceeded,
}

// Actor is who applied a transition: an admin's email and their note. The zero Actor is the
// system.
type Actor struct {
	Email string
	Note  string
}

// AdminApply applies an admin's event to an order, with their email and note on the transition.
// An event the order has moved past (a second click) is a no-op, as for any other caller.
func (s *Service) AdminApply(ctx context.Context, id uuid.UUID, ev Event, by Actor) error {
	by.Note = strings.TrimSpace(by.Note)

	var v validate.Validator
	v.Check(slices.Contains(adminEvents, ev), "event", "unknown")
	// The money went back outside Stripe: how is worth writing down.
	v.Check(ev != EventRefundSucceeded || by.Note != "", "note", "required")
	v.Check(utf8.RuneCountInString(by.Note) <= MaxNoteLen, "note", "too_long")
	if err := v.Err(); err != nil {
		return err
	}
	return db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		return s.applyTx(ctx, tx, id, ev, by)
	})
}

// adminActions are the admin events that would change the order now.
func (o Order) adminActions(now time.Time) []Event {
	var out []Event
	for _, ev := range adminEvents {
		if next, _, err := o.Transition(ev, now); err == nil && next.Seq != o.Seq {
			out = append(out, ev)
		}
	}
	return out
}

// needsAction are the statuses in which an order waits for an admin: the supplier's answer, or
// a refund that needs them if Stripe can't make it.
var needsAction = []Status{StatusSupplierPending, StatusRefunding}

// AdminOrderFilter selects orders for the admin's list.
type AdminOrderFilter struct {
	// Text is an order number, or part of the customer's name, email or phone.
	Text        string
	Status      Status // "" for any
	NeedsAction bool   // only orders waiting for an admin, the longest waiting first
	Page        int
	PageSize    int
}

// AdminOrderSummary is an order in the admin's list.
type AdminOrderSummary struct {
	ID          uuid.UUID
	Number      int64
	Status      Status
	Payment     Payment
	PaidAt      *time.Time
	CreatedAt   time.Time
	StatusSince time.Time
	Stale       bool
	Name        string
	Email       string
	Count       int
	TotalCents  money.Cents
}

type AdminOrderPage struct {
	Orders []AdminOrderSummary
	Total  int
}

// AdminOrders returns one page of orders, the newest first, or with NeedsAction the longest
// waiting first.
func (s *Service) AdminOrders(ctx context.Context, f AdminOrderFilter) (AdminOrderPage, error) {
	f.Text = strings.TrimSpace(f.Text)

	var v validate.Validator
	v.Check(utf8.RuneCountInString(f.Text) <= maxAdminTextLen, "q", "too_long")
	v.Check(f.Status == "" || slices.Contains(statuses, f.Status), "status", "unknown")
	v.Check(f.Page >= 1, "page", "out_of_range")
	v.Check(f.PageSize >= 1 && f.PageSize <= maxAdminPageSize, "page_size", "out_of_range")
	if err := v.Err(); err != nil {
		return AdminOrderPage{}, err
	}

	staleBefore := s.now().Add(-StaleAfter)
	list, total, err := s.store.adminOrders(ctx, s.pool, f, staleBefore)
	if err != nil {
		return AdminOrderPage{}, err
	}
	return AdminOrderPage{Orders: list, Total: total}, nil
}

// AdminOrderLine is an order line with what we pay the supplier for it.
type AdminOrderLine struct {
	OrderLine
	Provider    string
	ProviderSKU string
	DealerCents money.Cents // one unit, VAT included
	VATbp       int
}

// Transition is one change in an order's history.
type Transition struct {
	Seq   int
	Event Event
	From  Status
	To    Status
	At    time.Time
	Actor Actor
}

// AdminOrderDetail is an order as the admin sees it. Its Lines carry the supplier's side;
// OrderDetail.Lines is left empty.
type AdminOrderDetail struct {
	OrderDetail
	Lines       []AdminOrderLine
	DealerCents money.Cents // what the lines cost us, VAT included
	MarginCents money.Cents // the lines' subtotal minus DealerCents; delivery and assembly left out
	StatusSince time.Time
	Stale       bool
	Transitions []Transition // oldest first
	Actions     []Event      // what the admin can apply now
}

// AdminOrder returns any order with its supplier prices, history and the admin's actions.
func (s *Service) AdminOrder(ctx context.Context, id uuid.UUID) (AdminOrderDetail, error) {
	var d AdminOrderDetail
	var seq int
	err := db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		// One snapshot, so the lines and history match the order.
		var err error
		if d.OrderDetail, err = s.store.order(ctx, tx, id); err != nil {
			return err
		}
		if d.Lines, err = s.store.adminLines(ctx, tx, id); err != nil {
			return err
		}
		if d.Transitions, err = s.store.transitions(ctx, tx, id); err != nil {
			return err
		}
		d.StatusSince, seq, err = s.store.statusSince(ctx, tx, id)
		return err
	})
	if err != nil {
		return AdminOrderDetail{}, err
	}
	d.OrderDetail.Lines = nil
	for _, l := range d.Lines {
		d.DealerCents += l.DealerCents * money.Cents(l.Quantity)
	}
	d.MarginCents = d.SubtotalCents - d.DealerCents

	now := s.now()
	d.Stale = d.Status == StatusSupplierPending && now.Sub(d.StatusSince) > StaleAfter
	o := Order{ID: d.ID, Status: d.Status, Payment: d.Payment, PaidAt: d.PaidAt, Seq: seq}
	d.Actions = o.adminActions(now)
	return d, nil
}

// AdminCounts are the orders waiting for an admin, for the dashboard and the nav.
type AdminCounts struct {
	SupplierPending int // waiting for the supplier's answer
	Stale           int // of them, waiting longer than StaleAfter
	Refunding       int
}

func (s *Service) AdminCounts(ctx context.Context) (AdminCounts, error) {
	return s.store.adminCounts(ctx, s.pool, s.now().Add(-StaleAfter))
}

var statuses = []Status{
	StatusAwaitingPayment,
	StatusSupplierPending,
	StatusReadyToShip,
	StatusShipped,
	StatusDelivered,
	StatusRefunding,
	StatusCancelled,
}
