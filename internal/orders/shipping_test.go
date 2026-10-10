package orders

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/gfotev/loadout/internal/carrier"
)

// countingCarrier is the mock courier, counting the waybills it makes and what it was asked.
type countingCarrier struct {
	*carrier.Mock
	requests []carrier.Request
}

func (c *countingCarrier) CreateShipment(ctx context.Context, r carrier.Request) (carrier.Shipment, error) {
	c.requests = append(c.requests, r)
	s, err := c.Mock.CreateShipment(ctx, r)
	s.TrackingURL = "https://example.bg/t/" + s.Number
	return s, err
}

func TestShipping(t *testing.T) {
	s, _ := newCheckoutService(t)
	ctx := context.Background()
	now := s.now()
	c := &countingCarrier{Mock: carrier.NewMock()}
	c.Now = func() time.Time { return now }
	m := &fakeMailer{}
	step := NewStepWorker(s, m, nil, c)
	track := NewTrackWorker(s, c)

	work := func(id uuid.UUID, cmd Command) {
		t.Helper()
		job := &river.Job[StepArgs]{JobRow: &rivertype.JobRow{}, Args: StepArgs{OrderID: id, Command: cmd, Seq: 2}}
		if err := step.Work(ctx, job); err != nil {
			t.Fatal(err)
		}
	}
	poll := func(after time.Duration) {
		t.Helper()
		now = s.now().Add(after)
		if err := track.Work(ctx, &river.Job[TrackArgs]{JobRow: &rivertype.JobRow{}}); err != nil {
			t.Fatal(err)
		}
	}
	confirmed := func(p Payment) uuid.UUID {
		t.Helper()
		in := checkoutInput(400_00 + DeliveryCents)
		in.Payment = p
		placed, err := s.Checkout(ctx, guestCart(t, s, AddLine{Slug: "cpu", Quantity: 2}), in)
		if err != nil {
			t.Fatal(err)
		}
		if p == PaymentCard {
			if err := s.Apply(ctx, placed.ID, EventPaymentSucceeded); err != nil {
				t.Fatal(err)
			}
		}
		if err := s.Apply(ctx, placed.ID, EventSupplierConfirmed); err != nil {
			t.Fatal(err)
		}
		return placed.ID
	}

	cod := confirmed(PaymentCOD)
	work(cod, CmdCreateShipment)
	work(cod, CmdCreateShipment) // a retry makes no second waybill
	if len(c.requests) != 1 {
		t.Fatalf("%d waybills", len(c.requests))
	}
	if r := c.requests[0]; r.CODCents != 400_00+DeliveryCents || r.DeclaredCents != 400_00 || r.Name != "Иван Иванов" ||
		r.City != "София" || r.Contents != "Компютърни компоненти" {
		t.Errorf("request %+v", r)
	}
	sh, err := s.Shipment(ctx, cod)
	if err != nil || sh.Carrier != "mock" || sh.Number == "" {
		t.Fatalf("shipment %+v, %v", sh, err)
	}

	card := confirmed(PaymentCard)
	work(card, CmdCreateShipment)
	if r := c.requests[1]; r.CODCents != 0 {
		t.Errorf("a paid order's COD is %d", r.CODCents)
	}

	cancelled := confirmed(PaymentCOD)
	if err := s.Apply(ctx, cancelled, EventCancel); err != nil {
		t.Fatal(err)
	}
	work(cancelled, CmdCreateShipment)
	if len(c.requests) != 2 {
		t.Error("a cancelled order got a waybill")
	}
	if _, err := s.Shipment(ctx, cancelled); !errors.Is(err, ErrNotFound) {
		t.Errorf("cancelled order's shipment: %v", err)
	}

	t.Run("the order follows the parcel", func(t *testing.T) {
		poll(time.Minute)
		if o := loadOrder(t, s, cod); o.Status != StatusReadyToShip {
			t.Fatalf("before pickup: %s", o.Status)
		}
		poll(2 * time.Minute)
		if o := loadOrder(t, s, cod); o.Status != StatusShipped || o.PaidAt != nil {
			t.Fatalf("picked up: %s, paid %v", o.Status, o.PaidAt)
		}
		poll(5 * time.Minute)
		if o := loadOrder(t, s, cod); o.Status != StatusDelivered || o.PaidAt == nil {
			t.Errorf("delivered COD: %s, paid %v", o.Status, o.PaidAt)
		}
		if o := loadOrder(t, s, card); o.Status != StatusDelivered || o.PaidAt == nil {
			t.Errorf("delivered card: %s, paid %v", o.Status, o.PaidAt)
		}
		var events []Event
		for _, r := range transitionRows(t, s, cod) {
			events = append(events, r.Event)
		}
		want := []Event{EventCODSelected, EventSupplierConfirmed, EventPickedUp, EventDelivered, EventPaymentSucceeded}
		if len(events) != len(want) {
			t.Fatalf("events %v, want %v", events, want)
		}
		for i := range want {
			if events[i] != want[i] {
				t.Errorf("events %v, want %v", events, want)
			}
		}
		poll(time.Hour) // delivered orders aren't asked about again
	})

	t.Run("the shipped email has the waybill", func(t *testing.T) {
		work(cod, CmdEmailShipped)
		e := m.sent[len(m.sent)-1]
		if m.templates[len(m.templates)-1] != "shipped" || e.TrackingNumber != sh.Number ||
			e.TrackingURL != "https://example.bg/t/"+sh.Number {
			t.Errorf("email %+v", e)
		}
	})
}
