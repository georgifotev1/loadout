package orders

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/gfotev/loadout/internal/catalog"
	"github.com/gfotev/loadout/internal/money"
	"github.com/gfotev/loadout/internal/notify"
)

// newCheckoutService is a service with the cart's fake catalog that queues jobs for real.
func newCheckoutService(t *testing.T) (*Service, *fakeCatalog) {
	t.Helper()
	s := newOrderService(t)
	cat := &fakeCatalog{parts: []catalog.Part{
		part("cpu", 200_00, 5),
		part("ram", 80_00, 5),
	}}
	s.catalog = cat
	return s, cat
}

// guestCart starts a guest cart with lines and returns its owner.
func guestCart(t *testing.T, s *Service, lines ...AddLine) CartOwner {
	t.Helper()
	_, token, err := s.AddToCart(context.Background(), GuestCart(""), lines, false)
	if err != nil {
		t.Fatal(err)
	}
	return GuestCart(token)
}

func checkoutInput(total money.Cents) CheckoutInput {
	return CheckoutInput{
		Name:               " Иван Иванов ",
		Email:              "Ivan@Example.bg",
		Phone:              "+359 888 123-456",
		Delivery:           DeliveryAddress,
		City:               "София",
		Postcode:           "1000",
		Address:            "ул. Граф Игнатиев 1, ет. 2",
		Payment:            PaymentCOD,
		ExpectedTotalCents: total,
		AcceptTerms:        true,
	}
}

// addressTotal is the quote's total with delivery to an address.
func addressTotal(q Quote) money.Cents {
	for _, d := range q.Deliveries {
		if d.Delivery == DeliveryAddress {
			return d.TotalCents
		}
	}
	panic("no delivery to an address")
}

func countOrders(t *testing.T, s *Service) int {
	t.Helper()
	var n int
	if err := s.pool.QueryRow(context.Background(), `SELECT count(*) FROM orders.orders`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestCheckout(t *testing.T) {
	s, _ := newCheckoutService(t)
	ctx := context.Background()
	owner := guestCart(t, s, AddLine{Slug: "cpu", Quantity: 2})
	if _, _, err := s.AddToCart(ctx, owner, []AddLine{{Slug: "ram", Quantity: 1}}, false); err != nil {
		t.Fatal(err)
	}

	q, err := s.Quote(ctx, owner)
	if err != nil {
		t.Fatal(err)
	}
	if !q.Orderable || addressTotal(q) != 480_00+AddressDeliveryCents {
		t.Fatalf("quote %+v", q)
	}

	placed, err := s.Checkout(ctx, owner, checkoutInput(addressTotal(q)))
	if err != nil {
		t.Fatal(err)
	}
	if placed.Number < 1001 || placed.Token == "" {
		t.Errorf("placed %+v", placed)
	}

	o, err := s.Order(ctx, placed.ID, Viewer{Token: placed.Token})
	if err != nil {
		t.Fatal(err)
	}
	if o.Status != StatusSupplierPending || o.Payment != PaymentCOD || o.UserID != nil ||
		o.Name != "Иван Иванов" || o.Email != "Ivan@Example.bg" ||
		o.SubtotalCents != 480_00 || o.DeliveryCents != AddressDeliveryCents || o.TotalCents != addressTotal(q) {
		t.Errorf("order %+v", o)
	}
	// In the cart's order: the last added first.
	if len(o.Lines) != 2 ||
		o.Lines[0].Slug != "ram" || o.Lines[0].TotalCents != 80_00 ||
		o.Lines[1].Slug != "cpu" || o.Lines[1].Quantity != 2 || o.Lines[1].UnitCents != 200_00 || o.Lines[1].TotalCents != 400_00 {
		t.Errorf("lines %+v", o.Lines)
	}

	t.Run("the lines keep what we pay the supplier", func(t *testing.T) {
		var sku string
		var dealer money.Cents
		var vat int
		err := s.pool.QueryRow(ctx, `
			SELECT provider_sku, dealer_cents, vat_bp FROM orders.order_lines
			WHERE order_id = $1 AND slug = 'cpu'`, placed.ID).Scan(&sku, &dealer, &vat)
		if err != nil {
			t.Fatal(err)
		}
		if sku != "CPU" || dealer != 160_00 || vat != money.VATStandardBP {
			t.Errorf("sku %q, dealer %d, vat %d", sku, dealer, vat)
		}
	})

	t.Run("the order keeps the terms the customer accepted", func(t *testing.T) {
		var version string
		err := s.pool.QueryRow(ctx, `SELECT terms_version FROM orders.orders WHERE id = $1`, placed.ID).Scan(&version)
		if err != nil {
			t.Fatal(err)
		}
		if version != TermsVersion {
			t.Errorf("terms_version %q, want %q", version, TermsVersion)
		}
	})

	t.Run("the cart is gone", func(t *testing.T) {
		c, err := s.Cart(ctx, owner)
		if err != nil {
			t.Fatal(err)
		}
		if len(c.Lines) != 0 {
			t.Errorf("cart %+v", c)
		}
	})

	t.Run("cod_selected queued the first steps", func(t *testing.T) {
		want := []StepArgs{
			{placed.ID, CmdPlaceSupplierOrder, 1},
			{placed.ID, CmdEmailReceived, 1},
		}
		if got := queuedSteps(t, s, placed.ID); !slices.Equal(got, want) {
			t.Errorf("steps %+v, want %+v", got, want)
		}
	})

	t.Run("a logged-in customer's order is theirs", func(t *testing.T) {
		user := uuid.New()
		owner := UserCart(user)
		if _, _, err := s.AddToCart(ctx, owner, []AddLine{{Slug: "ram", Quantity: 1}}, false); err != nil {
			t.Fatal(err)
		}
		in := checkoutInput(80_00 + AddressDeliveryCents)
		in.UserID = user
		placed, err := s.Checkout(ctx, owner, in)
		if err != nil {
			t.Fatal(err)
		}
		o, err := s.Order(ctx, placed.ID, Viewer{UserID: user})
		if err != nil {
			t.Fatal(err)
		}
		if o.UserID == nil || *o.UserID != user {
			t.Errorf("user %v", o.UserID)
		}
	})
}

// A card order waits for its payment, with a job to settle it once the payment window is over.
func TestCheckoutByCard(t *testing.T) {
	s, _ := newCheckoutService(t)
	ctx := context.Background()
	owner := guestCart(t, s, AddLine{Slug: "cpu", Quantity: 1})
	in := checkoutInput(200_00 + AddressDeliveryCents)
	in.Payment = PaymentCard

	placed, err := s.Checkout(ctx, owner, in)
	if err != nil {
		t.Fatal(err)
	}
	o, err := s.Order(ctx, placed.ID, Viewer{Token: placed.Token})
	if err != nil {
		t.Fatal(err)
	}
	if o.Status != StatusAwaitingPayment || o.Payment != PaymentCard || o.PaidAt != nil {
		t.Errorf("order %+v", o)
	}
	if steps := queuedSteps(t, s, placed.ID); len(steps) != 0 {
		t.Errorf("steps %+v, want none before the payment", steps)
	}
	if c, err := s.Cart(ctx, owner); err != nil || len(c.Lines) != 0 {
		t.Errorf("cart %+v, %v; want it gone", c, err)
	}

	var at time.Time
	err = s.pool.QueryRow(ctx, `
		SELECT scheduled_at FROM river.river_job
		WHERE kind = 'order_payment_timeout' AND args->>'order_id' = $1`, placed.ID.String()).Scan(&at)
	if err != nil {
		t.Fatal(err)
	}
	if want := s.now().Add(PaymentWindow + paymentTimeoutGrace); !at.Equal(want) {
		t.Errorf("timeout at %v, want %v", at, want)
	}
}

func TestCheckoutRefuses(t *testing.T) {
	s, cat := newCheckoutService(t)
	ctx := context.Background()
	total := 200_00 + AddressDeliveryCents

	t.Run("bad fields", func(t *testing.T) {
		owner := guestCart(t, s, AddLine{Slug: "cpu", Quantity: 1})
		in := CheckoutInput{
			Email:    "ivan",
			Phone:    "0888",
			Delivery: DeliveryAddress,
			City:     "  ",
			Postcode: "10000",
			Address:  "x",
			Payment:  "cash",
		}
		_, err := s.Checkout(ctx, owner, in)
		want := []string{
			"name: required",
			"email: invalid_email",
			"phone: invalid_phone",
			"city: required",
			"postcode: invalid_postcode",
			"payment: unsupported",
			"accept_terms: required",
		}
		if got := fieldCodes(t, err); !slices.Equal(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})

	t.Run("bad delivery", func(t *testing.T) {
		owner := guestCart(t, s, AddLine{Slug: "cpu", Quantity: 1})
		for _, tt := range []struct {
			delivery Delivery
			want     string
		}{
			{"", "delivery_method: unsupported"},
			{"pigeon", "delivery_method: unsupported"},
			{DeliveryOffice, "office_id: required"},
			{DeliveryAPT, "office_id: required"},
		} {
			in := checkoutInput(200_00 + tt.delivery.Cents())
			in.Delivery = tt.delivery
			_, err := s.Checkout(ctx, owner, in)
			if got := fieldCodes(t, err); !slices.Equal(got, []string{tt.want}) {
				t.Errorf("%q: got %v, want %v", tt.delivery, got, tt.want)
			}
		}
	})

	for _, tt := range []struct {
		name  string
		setup func(t *testing.T) (CartOwner, money.Cents)
		want  string
	}{
		{"no cart", func(*testing.T) (CartOwner, money.Cents) { return GuestCart(""), total }, "cart: empty"},
		{"an unknown cart", func(*testing.T) (CartOwner, money.Cents) { return GuestCart("gone"), total }, "cart: empty"},
		{"a product no longer for sale", func(t *testing.T) (CartOwner, money.Cents) {
			cat.parts = append(cat.parts, part("old", 10_00, 1))
			owner := guestCart(t, s, AddLine{Slug: "cpu", Quantity: 1}, AddLine{Slug: "old", Quantity: 1})
			cat.remove("old")
			return owner, total
		}, "cart: unavailable"},
		{"a product out of stock", func(t *testing.T) (CartOwner, money.Cents) {
			owner := guestCart(t, s, AddLine{Slug: "ram", Quantity: 1})
			cat.update("ram", func(p *catalog.Part) { p.StockQty = 0 })
			t.Cleanup(func() { cat.update("ram", func(p *catalog.Part) { p.StockQty = 5 }) })
			return owner, 80_00 + AddressDeliveryCents
		}, "cart: unavailable"},
		{"a price that moved since the customer looked", func(t *testing.T) (CartOwner, money.Cents) {
			owner := guestCart(t, s, AddLine{Slug: "cpu", Quantity: 1})
			cat.update("cpu", func(p *catalog.Part) { p.RetailCents = 210_00 })
			t.Cleanup(func() { cat.update("cpu", func(p *catalog.Part) { p.RetailCents = 200_00 }) })
			return owner, total
		}, "expected_total_cents: changed"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			owner, expected := tt.setup(t)
			before, err := s.Cart(ctx, owner)
			if err != nil {
				t.Fatal(err)
			}
			_, err = s.Checkout(ctx, owner, checkoutInput(expected))
			if got := fieldCodes(t, err); !slices.Equal(got, []string{tt.want}) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
			if after, err := s.Cart(ctx, owner); err != nil || len(after.Lines) != len(before.Lines) {
				t.Errorf("cart changed: %+v, %v", after, err)
			}
		})
	}
	if n := countOrders(t, s); n != 0 {
		t.Errorf("%d orders, want none", n)
	}
}

// A double click places one order: the second checkout waits for the cart's lock, then finds
// the cart gone.
func TestCheckoutConcurrently(t *testing.T) {
	s, _ := newCheckoutService(t)
	ctx := context.Background()
	owner := guestCart(t, s, AddLine{Slug: "cpu", Quantity: 1})

	errs := make([]error, 2)
	var wg sync.WaitGroup
	for i := range errs {
		wg.Go(func() { _, errs[i] = s.Checkout(ctx, owner, checkoutInput(200_00+AddressDeliveryCents)) })
	}
	wg.Wait()

	var failed int
	for _, err := range errs {
		if err != nil {
			if got := fieldCodes(t, err); !slices.Equal(got, []string{"cart: empty"}) {
				t.Errorf("got %v", got)
			}
			failed++
		}
	}
	if failed != 1 || countOrders(t, s) != 1 {
		t.Errorf("%d failed, %d orders; want one of each", failed, countOrders(t, s))
	}
}

// The order, its lines, the cart's deletion and the order's steps commit together.
func TestCheckoutRollsBack(t *testing.T) {
	s, _ := newCheckoutService(t)
	ctx := context.Background()
	owner := guestCart(t, s, AddLine{Slug: "cpu", Quantity: 1})
	s.jobs = &failingInserter{jobs: s.jobs, ok: 1}

	if _, err := s.Checkout(ctx, owner, checkoutInput(200_00+AddressDeliveryCents)); !errors.Is(err, errInsert) {
		t.Fatalf("err %v, want the insert error", err)
	}
	if n := countOrders(t, s); n != 0 {
		t.Errorf("%d orders, want none", n)
	}
	if c, err := s.Cart(ctx, owner); err != nil || c.Count != 1 {
		t.Errorf("cart %+v, %v; want it as it was", c, err)
	}
}

func TestQuoteNotOrderable(t *testing.T) {
	s, cat := newCheckoutService(t)
	ctx := context.Background()

	if q, err := s.Quote(ctx, GuestCart("")); err != nil || q.Orderable {
		t.Errorf("empty cart: %+v, %v", q, err)
	}
	owner := guestCart(t, s, AddLine{Slug: "cpu", Quantity: 1}, AddLine{Slug: "ram", Quantity: 1})
	cat.update("ram", func(p *catalog.Part) { p.StockQty = 0 })
	if q, err := s.Quote(ctx, owner); err != nil || q.Orderable {
		t.Errorf("out of stock: %+v, %v", q, err)
	}
}

func TestOrderAccess(t *testing.T) {
	s, _ := newCheckoutService(t)
	ctx := context.Background()
	user, other := uuid.New(), uuid.New()

	guest, err := s.Checkout(ctx, guestCart(t, s, AddLine{Slug: "cpu", Quantity: 1}), checkoutInput(200_00+AddressDeliveryCents))
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.AddToCart(ctx, UserCart(user), []AddLine{{Slug: "ram", Quantity: 3}}, false); err != nil {
		t.Fatal(err)
	}
	in := checkoutInput(240_00 + AddressDeliveryCents)
	in.UserID, in.Email = user, "someone@else.bg"
	own, err := s.Checkout(ctx, UserCart(user), in)
	if err != nil {
		t.Fatal(err)
	}

	for _, tt := range []struct {
		name string
		id   uuid.UUID
		v    Viewer
		ok   bool
	}{
		{"the link's token", guest.ID, Viewer{Token: guest.Token}, true},
		{"another order's token", guest.ID, Viewer{Token: own.Token}, false},
		{"no one", guest.ID, Viewer{}, false},
		{"the customer who placed it", own.ID, Viewer{UserID: user}, true},
		{"another customer", own.ID, Viewer{UserID: other}, false},
		{"the guest order's verified email, in any case", guest.ID, Viewer{UserID: other, VerifiedEmail: "ivan@example.BG"}, true},
		{"another verified email", guest.ID, Viewer{UserID: other, VerifiedEmail: "petar@example.bg"}, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := s.Order(ctx, tt.id, tt.v)
			if tt.ok && err != nil {
				t.Errorf("err %v", err)
			}
			if !tt.ok && !errors.Is(err, ErrNotFound) {
				t.Errorf("err %v, want ErrNotFound", err)
			}
		})
	}

	t.Run("history", func(t *testing.T) {
		ids := func(os []OrderSummary) []uuid.UUID {
			var out []uuid.UUID
			for _, o := range os {
				out = append(out, o.ID)
			}
			return out
		}
		got, err := s.CustomerOrders(ctx, user, "")
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(ids(got), []uuid.UUID{own.ID}) || got[0].Count != 3 || got[0].TotalCents != 240_00+AddressDeliveryCents {
			t.Errorf("unverified: %+v", got)
		}
		got, err = s.CustomerOrders(ctx, user, "IVAN@example.bg")
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 2 || !slices.Contains(ids(got), guest.ID) {
			t.Errorf("verified: %+v", got)
		}
		if got, err := s.CustomerOrders(ctx, other, ""); err != nil || len(got) != 0 {
			t.Errorf("another customer: %+v, %v", got, err)
		}
	})
}

// fakeMailer records the emails sent, as "template" and the order.
type fakeMailer struct {
	sent      []notify.Order
	templates []string
}

func (f *fakeMailer) record(tpl string, o notify.Order) error {
	f.sent = append(f.sent, o)
	f.templates = append(f.templates, tpl)
	return nil
}

func (f *fakeMailer) OrderReceived(_ context.Context, o notify.Order) error {
	return f.record("received", o)
}

func (f *fakeMailer) OrderConfirmed(_ context.Context, o notify.Order) error {
	return f.record("confirmed", o)
}

func (f *fakeMailer) OrderCancelled(_ context.Context, o notify.Order) error {
	return f.record("cancelled", o)
}

func (f *fakeMailer) OrderShipped(_ context.Context, o notify.Order) error {
	return f.record("shipped", o)
}

func (f *fakeMailer) OrderDelivered(_ context.Context, o notify.Order) error {
	return f.record("delivered", o)
}

func TestStepWorker(t *testing.T) {
	s, _ := newCheckoutService(t)
	ctx := context.Background()
	placed, err := s.Checkout(ctx, guestCart(t, s, AddLine{Slug: "cpu", Quantity: 2}), checkoutInput(400_00+AddressDeliveryCents))
	if err != nil {
		t.Fatal(err)
	}
	m := &fakeMailer{}
	w := NewStepWorker(s, m, nil, nil)
	work := func(cmd Command) error {
		return w.Work(ctx, &river.Job[StepArgs]{JobRow: &rivertype.JobRow{}, Args: StepArgs{OrderID: placed.ID, Command: cmd, Seq: 1}})
	}

	if err := work(CmdEmailReceived); err != nil {
		t.Fatal(err)
	}
	if len(m.sent) != 1 {
		t.Fatalf("sent %d emails", len(m.sent))
	}
	e := m.sent[0]
	if e.ID != placed.ID || e.Number != placed.Number || e.Token != placed.Token || e.Email != "Ivan@Example.bg" ||
		e.Address != "ул. Граф Игнатиев 1, ет. 2, 1000 София" || !e.CashOnDelivery ||
		len(e.Lines) != 1 || e.Lines[0].LineCents != 400_00 || e.TotalCents != 400_00+AddressDeliveryCents {
		t.Errorf("email %+v", e)
	}

	t.Run("the supplier order is placed by hand", func(t *testing.T) {
		if err := work(CmdPlaceSupplierOrder); err != nil {
			t.Error(err)
		}
	})

	t.Run("an unknown command is cancelled", func(t *testing.T) {
		var cancel *rivertype.JobCancelError
		if err := work(Command("teleport")); !errors.As(err, &cancel) {
			t.Errorf("err %v, want a cancel", err)
		}
	})

	t.Run("confirmed and cancelled emails", func(t *testing.T) {
		if err := work(CmdEmailConfirmed); err != nil {
			t.Fatal(err)
		}
		if err := s.Apply(ctx, placed.ID, EventCancel); err != nil {
			t.Fatal(err)
		}
		if err := work(CmdEmailCancelled); err != nil {
			t.Fatal(err)
		}
		// A confirmation that comes after the cancellation isn't sent.
		if err := work(CmdEmailConfirmed); err != nil {
			t.Fatal(err)
		}
		if want := []string{"received", "confirmed", "cancelled"}; !slices.Equal(m.templates, want) {
			t.Errorf("sent %v, want %v", m.templates, want)
		}
	})

	t.Run("an order that's gone sends nothing", func(t *testing.T) {
		job := &river.Job[StepArgs]{JobRow: &rivertype.JobRow{}, Args: StepArgs{OrderID: uuid.New(), Command: CmdEmailReceived, Seq: 1}}
		n := len(m.sent)
		if err := w.Work(ctx, job); err != nil || len(m.sent) != n {
			t.Errorf("err %v, sent %d", err, len(m.sent)-n)
		}
	})
}

// fakePayments answers for Stripe: outcome is what became of every payment, refunded whether a
// refund is done at once.
type fakePayments struct {
	outcome  Event
	refunded bool
	err      error
	refunds  []uuid.UUID
	expired  []uuid.UUID
}

func (f *fakePayments) Outcome(context.Context, uuid.UUID) (Event, error) {
	return f.outcome, f.err
}

func (f *fakePayments) Refund(_ context.Context, id uuid.UUID) (bool, error) {
	f.refunds = append(f.refunds, id)
	return f.refunded, f.err
}

func (f *fakePayments) Expire(_ context.Context, id uuid.UUID) error {
	f.expired = append(f.expired, id)
	return f.err
}

func TestExpirePaymentStep(t *testing.T) {
	s := newOrderService(t)
	ctx := context.Background()
	id := newOrder(t, s, PaymentCard)
	job := &river.Job[StepArgs]{JobRow: &rivertype.JobRow{}, Args: StepArgs{OrderID: id, Command: CmdExpirePayment, Seq: 1}}

	p := &fakePayments{}
	if err := NewStepWorker(s, &fakeMailer{}, p, nil).Work(ctx, job); err != nil || !slices.Equal(p.expired, []uuid.UUID{id}) {
		t.Errorf("err %v, expired %v", err, p.expired)
	}
	if err := NewStepWorker(s, &fakeMailer{}, nil, nil).Work(ctx, job); err != nil {
		t.Errorf("without card payments: %v", err)
	}
}

func TestRefundStep(t *testing.T) {
	s := newOrderService(t)
	ctx := context.Background()
	work := func(w *StepWorker, id uuid.UUID) error {
		return w.Work(ctx, &river.Job[StepArgs]{JobRow: &rivertype.JobRow{}, Args: StepArgs{OrderID: id, Command: CmdRefundPayment, Seq: 2}})
	}
	// refunding is a card order paid, then cancelled.
	refunding := func(t *testing.T) uuid.UUID {
		t.Helper()
		id := newOrder(t, s, PaymentCard)
		for _, ev := range []Event{EventPaymentSucceeded, EventCancel} {
			if err := s.Apply(ctx, id, ev); err != nil {
				t.Fatal(err)
			}
		}
		return id
	}

	t.Run("a refund done at once cancels the order", func(t *testing.T) {
		id := refunding(t)
		p := &fakePayments{refunded: true}
		if err := work(NewStepWorker(s, &fakeMailer{}, p, nil), id); err != nil {
			t.Fatal(err)
		}
		if o := loadOrder(t, s, id); o.Status != StatusCancelled || o.PaidAt == nil || len(p.refunds) != 1 {
			t.Errorf("order %+v, refunds %v", o, p.refunds)
		}

		t.Run("and isn't asked for again", func(t *testing.T) {
			if err := work(NewStepWorker(s, &fakeMailer{}, p, nil), id); err != nil || len(p.refunds) != 1 {
				t.Errorf("err %v, refunds %v", err, p.refunds)
			}
		})
	})

	t.Run("a pending refund waits for its webhook", func(t *testing.T) {
		id := refunding(t)
		if err := work(NewStepWorker(s, &fakeMailer{}, &fakePayments{}, nil), id); err != nil {
			t.Fatal(err)
		}
		if o := loadOrder(t, s, id); o.Status != StatusRefunding {
			t.Errorf("status %s", o.Status)
		}
	})

	t.Run("an error is retried", func(t *testing.T) {
		id := refunding(t)
		errStripe := errors.New("stripe is down")
		if err := work(NewStepWorker(s, &fakeMailer{}, &fakePayments{err: errStripe}, nil), id); !errors.Is(err, errStripe) {
			t.Errorf("err %v", err)
		}
		if err := work(NewStepWorker(s, &fakeMailer{}, nil, nil), id); err == nil {
			t.Error("no error without card payments")
		}
	})
}

func TestPaymentTimeoutWorker(t *testing.T) {
	s := newOrderService(t)
	ctx := context.Background()
	work := func(p Payments, id uuid.UUID) error {
		w := NewPaymentTimeoutWorker(s, p)
		return w.Work(ctx, &river.Job[PaymentTimeoutArgs]{JobRow: &rivertype.JobRow{}, Args: PaymentTimeoutArgs{OrderID: id}})
	}

	for _, tt := range []struct {
		name     string
		payments Payments
		want     Status
	}{
		{"timed out", &fakePayments{outcome: EventPaymentTimedOut}, StatusCancelled},
		{"paid, but the webhook never came", &fakePayments{outcome: EventPaymentSucceeded}, StatusSupplierPending},
		{"failed", &fakePayments{outcome: EventPaymentFailed}, StatusCancelled},
		{"a bank debit on its way", &fakePayments{}, StatusAwaitingPayment},
		{"no card payments set up", nil, StatusCancelled},
	} {
		t.Run(tt.name, func(t *testing.T) {
			id := newOrder(t, s, PaymentCard)
			if err := work(tt.payments, id); err != nil {
				t.Fatal(err)
			}
			if o := loadOrder(t, s, id); o.Status != tt.want {
				t.Errorf("status %s, want %s", o.Status, tt.want)
			}
		})
	}

	t.Run("an order settled already isn't asked about", func(t *testing.T) {
		id := newOrder(t, s, PaymentCard)
		if err := s.Apply(ctx, id, EventPaymentSucceeded); err != nil {
			t.Fatal(err)
		}
		p := &fakePayments{err: errors.New("must not be called")}
		if err := work(p, id); err != nil {
			t.Error(err)
		}
	})

	t.Run("an order that's gone", func(t *testing.T) {
		if err := work(&fakePayments{}, uuid.New()); err != nil {
			t.Error(err)
		}
	})
}
