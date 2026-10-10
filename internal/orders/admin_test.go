package orders

import (
	"context"
	"errors"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/gfotev/loadout/internal/money"
)

// codOrder is a cash on delivery order waiting for the supplier.
func codOrder(t *testing.T, s *Service) uuid.UUID {
	t.Helper()
	id := newOrder(t, s, PaymentCOD)
	if err := s.Apply(context.Background(), id, EventCODSelected); err != nil {
		t.Fatal(err)
	}
	return id
}

func TestAdminApply(t *testing.T) {
	s := newOrderService(t)
	ctx := context.Background()
	id := codOrder(t, s)

	by := Actor{Email: "admin@loadout.bg", Note: "  Потвърдено по телефона  "}
	if err := s.AdminApply(ctx, id, EventSupplierConfirmed, by); err != nil {
		t.Fatal(err)
	}
	if o := loadOrder(t, s, id); o.Status != StatusReadyToShip {
		t.Errorf("status %s", o.Status)
	}
	ts, err := s.store.transitions(ctx, s.pool, id)
	if err != nil {
		t.Fatal(err)
	}
	if len(ts) != 2 || ts[0].Actor != (Actor{}) || ts[1].Actor != (Actor{Email: "admin@loadout.bg", Note: "Потвърдено по телефона"}) {
		t.Errorf("transitions %+v", ts)
	}

	t.Run("a second click is a no-op", func(t *testing.T) {
		if err := s.AdminApply(ctx, id, EventSupplierConfirmed, by); err != nil {
			t.Error(err)
		}
		if o := loadOrder(t, s, id); o.Seq != 2 {
			t.Errorf("seq %d", o.Seq)
		}
	})

	t.Run("an event that doesn't fit", func(t *testing.T) {
		if err := s.AdminApply(ctx, id, EventSupplierRejected, by); !errors.Is(err, ErrInvalidTransition) {
			t.Errorf("err %v", err)
		}
	})

	t.Run("refused", func(t *testing.T) {
		long := Actor{Note: string(make([]rune, MaxNoteLen+1))}
		for _, tt := range []struct {
			ev   Event
			by   Actor
			want string
		}{
			{EventPickedUp, by, "event: unknown"},
			{EventPaymentSucceeded, by, "event: unknown"},
			{EventRefundSucceeded, Actor{Note: "   "}, "note: required"},
			{EventCancel, long, "note: too_long"},
		} {
			got := fieldCodes(t, s.AdminApply(ctx, id, tt.ev, tt.by))
			if !slices.Equal(got, []string{tt.want}) {
				t.Errorf("%s: %v, want %s", tt.ev, got, tt.want)
			}
		}
	})

	t.Run("an order that doesn't exist", func(t *testing.T) {
		if err := s.AdminApply(ctx, uuid.New(), EventCancel, by); !errors.Is(err, ErrNotFound) {
			t.Errorf("err %v", err)
		}
	})
}

func TestStatusAt(t *testing.T) {
	s := newOrderService(t)
	ctx := context.Background()
	at := func(id uuid.UUID) time.Time {
		t.Helper()
		since, _, err := s.store.statusSince(ctx, s.pool, id)
		if err != nil {
			t.Fatal(err)
		}
		return since
	}
	start := s.now()
	id := codOrder(t, s)
	if got := at(id); !got.Equal(start) {
		t.Errorf("after checkout %v, want %v", got, start)
	}

	shipped := start.Add(time.Hour)
	s.now = func() time.Time { return shipped }
	for _, ev := range []Event{EventSupplierConfirmed, EventPickedUp} {
		if err := s.Apply(ctx, id, ev); err != nil {
			t.Fatal(err)
		}
	}
	// Collecting the cash doesn't change the status.
	s.now = func() time.Time { return shipped.Add(time.Hour) }
	if err := s.Apply(ctx, id, EventPaymentSucceeded); err != nil {
		t.Fatal(err)
	}
	if got := at(id); !got.Equal(shipped) {
		t.Errorf("after the cash %v, want %v", got, shipped)
	}
}

func TestAdminOrders(t *testing.T) {
	s := newOrderService(t)
	ctx := context.Background()
	now := s.now()

	pending := codOrder(t, s)
	stale := codOrder(t, s)
	if _, err := s.pool.Exec(ctx, `UPDATE orders.orders SET status_at = $2 WHERE id = $1`, stale, now.Add(-StaleAfter-time.Minute)); err != nil {
		t.Fatal(err)
	}
	awaiting := newOrder(t, s, PaymentCard)
	refunding := newOrder(t, s, PaymentCard)
	for _, ev := range []Event{EventPaymentSucceeded, EventCancel} {
		if err := s.Apply(ctx, refunding, ev); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.pool.Exec(ctx, `UPDATE orders.orders SET name = 'Мария Петрова', email = 'maria@example.bg' WHERE id = $1`, awaiting); err != nil {
		t.Fatal(err)
	}
	var number int64
	if err := s.pool.QueryRow(ctx, `SELECT number FROM orders.orders WHERE id = $1`, pending).Scan(&number); err != nil {
		t.Fatal(err)
	}

	list := func(f AdminOrderFilter) ([]uuid.UUID, AdminOrderPage) {
		t.Helper()
		f.Page, f.PageSize = 1, 10
		p, err := s.AdminOrders(ctx, f)
		if err != nil {
			t.Fatal(err)
		}
		ids := make([]uuid.UUID, len(p.Orders))
		for i, o := range p.Orders {
			ids[i] = o.ID
		}
		return ids, p
	}

	for _, tt := range []struct {
		name string
		f    AdminOrderFilter
		want []uuid.UUID
	}{
		{"all, newest first", AdminOrderFilter{}, []uuid.UUID{refunding, awaiting, stale, pending}},
		{"needs action, longest waiting first", AdminOrderFilter{NeedsAction: true}, []uuid.UUID{stale, pending, refunding}},
		{"by status", AdminOrderFilter{Status: StatusAwaitingPayment}, []uuid.UUID{awaiting}},
		{"by number", AdminOrderFilter{Text: strconv.FormatInt(number, 10)}, []uuid.UUID{pending}},
		{"by name", AdminOrderFilter{Text: "мария"}, []uuid.UUID{awaiting}},
		{"by email", AdminOrderFilter{Text: "MARIA@"}, []uuid.UUID{awaiting}},
		{"by phone", AdminOrderFilter{Text: "0888123", Status: StatusRefunding}, []uuid.UUID{refunding}},
		{"no match", AdminOrderFilter{Text: "%"}, []uuid.UUID{}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, p := list(tt.f)
			if !slices.Equal(got, tt.want) || p.Total != len(tt.want) {
				t.Errorf("got %v (total %d), want %v", got, p.Total, tt.want)
			}
		})
	}

	t.Run("stale", func(t *testing.T) {
		_, p := list(AdminOrderFilter{NeedsAction: true})
		for _, o := range p.Orders {
			if o.Stale != (o.ID == stale) {
				t.Errorf("order %s stale %v", o.ID, o.Stale)
			}
		}
	})

	t.Run("counts", func(t *testing.T) {
		c, err := s.AdminCounts(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if c != (AdminCounts{SupplierPending: 2, Stale: 1, Refunding: 1}) {
			t.Errorf("counts %+v", c)
		}
	})

	t.Run("refused", func(t *testing.T) {
		_, err := s.AdminOrders(ctx, AdminOrderFilter{Status: "lost", Page: 0, PageSize: 1000})
		want := []string{"status: unknown", "page: out_of_range", "page_size: out_of_range"}
		if got := fieldCodes(t, err); !slices.Equal(got, want) {
			t.Errorf("%v, want %v", got, want)
		}
	})
}

func TestAdminOrder(t *testing.T) {
	s, _ := newCheckoutService(t)
	ctx := context.Background()
	placed, err := s.Checkout(ctx, guestCart(t, s, AddLine{Slug: "cpu", Quantity: 2}, AddLine{Slug: "ram", Quantity: 1}), checkoutInput(480_00+DeliveryCents))
	if err != nil {
		t.Fatal(err)
	}

	d, err := s.AdminOrder(ctx, placed.ID)
	if err != nil {
		t.Fatal(err)
	}
	if d.Number != placed.Number || d.Status != StatusSupplierPending || len(d.OrderDetail.Lines) != 0 || len(d.Lines) != 2 {
		t.Fatalf("order %+v", d)
	}
	// The fake catalog's dealer price is 80% of retail.
	if d.DealerCents != 384_00 || d.MarginCents != 96_00 {
		t.Errorf("dealer %d, margin %d", d.DealerCents, d.MarginCents)
	}
	var cpu AdminOrderLine
	for _, l := range d.Lines {
		if l.Slug == "cpu" {
			cpu = l
		}
	}
	if cpu.Provider != "vali" || cpu.ProviderSKU != "CPU" || cpu.DealerCents != 160_00 || cpu.UnitCents != 200_00 || cpu.TotalCents != 400_00 || cpu.VATbp != money.VATStandardBP {
		t.Errorf("cpu line %+v", cpu)
	}
	if len(d.Transitions) != 1 || d.Transitions[0].Event != EventCODSelected || d.Transitions[0].To != StatusSupplierPending {
		t.Errorf("transitions %+v", d.Transitions)
	}
	if want := []Event{EventSupplierConfirmed, EventSupplierRejected, EventCancel}; !slices.Equal(d.Actions, want) {
		t.Errorf("actions %v, want %v", d.Actions, want)
	}
	if d.Stale || !d.StatusSince.Equal(s.now()) {
		t.Errorf("stale %v, since %v", d.Stale, d.StatusSince)
	}

	t.Run("a refunding order can be marked refunded", func(t *testing.T) {
		id := newOrder(t, s, PaymentCard)
		for _, ev := range []Event{EventPaymentSucceeded, EventCancel} {
			if err := s.Apply(ctx, id, ev); err != nil {
				t.Fatal(err)
			}
		}
		d, err := s.AdminOrder(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(d.Actions, []Event{EventRefundSucceeded}) {
			t.Errorf("actions %v", d.Actions)
		}
	})

	t.Run("not found", func(t *testing.T) {
		if _, err := s.AdminOrder(ctx, uuid.New()); !errors.Is(err, ErrNotFound) {
			t.Errorf("err %v", err)
		}
	})
}
