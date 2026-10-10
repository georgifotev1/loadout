package orders

import (
	"errors"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
)

// state is an order's state as far as Transition is concerned.
type state struct {
	status  Status
	payment Payment
	paid    bool
}

func (s state) String() string {
	paid := "unpaid"
	if s.paid {
		paid = "paid"
	}
	return fmt.Sprintf("%s/%s/%s", s.status, s.payment, paid)
}

var (
	awaitingCOD      = state{StatusAwaitingPayment, PaymentCOD, false}
	awaitingCard     = state{StatusAwaitingPayment, PaymentCard, false}
	pendingCOD       = state{StatusSupplierPending, PaymentCOD, false}
	pendingCard      = state{StatusSupplierPending, PaymentCard, true}
	readyCOD         = state{StatusReadyToShip, PaymentCOD, false}
	readyCard        = state{StatusReadyToShip, PaymentCard, true}
	shippedCOD       = state{StatusShipped, PaymentCOD, false}
	shippedCODPaid   = state{StatusShipped, PaymentCOD, true}
	shippedCard      = state{StatusShipped, PaymentCard, true}
	deliveredCOD     = state{StatusDelivered, PaymentCOD, false}
	deliveredCODPaid = state{StatusDelivered, PaymentCOD, true}
	deliveredCard    = state{StatusDelivered, PaymentCard, true}
	refundingCard    = state{StatusRefunding, PaymentCard, true}
	cancelledCOD     = state{StatusCancelled, PaymentCOD, false}
	cancelledCard    = state{StatusCancelled, PaymentCard, false}
	refundedCard     = state{StatusCancelled, PaymentCard, true}
)

// reachable is every state an order can be in. Card orders are paid from supplier_pending on;
// cash on delivery is paid at the earliest once shipped.
var reachable = []state{
	awaitingCOD, awaitingCard,
	pendingCOD, pendingCard,
	readyCOD, readyCard,
	shippedCOD, shippedCODPaid, shippedCard,
	deliveredCOD, deliveredCODPaid, deliveredCard,
	refundingCard,
	cancelledCOD, cancelledCard, refundedCard,
}

var allEvents = []Event{
	EventCODSelected,
	EventPaymentSucceeded,
	EventPaymentFailed,
	EventPaymentTimedOut,
	EventSupplierConfirmed,
	EventSupplierRejected,
	EventPickedUp,
	EventDelivered,
	EventRefundSucceeded,
	EventCancel,
}

type move struct {
	from state
	ev   Event
}

type result struct {
	to   state
	cmds []Command
}

// transitions is every event that changes an order, and what it does.
var transitions = map[move]result{
	{awaitingCOD, EventCODSelected}: {pendingCOD, []Command{CmdPlaceSupplierOrder, CmdEmailReceived}},
	{awaitingCOD, EventCancel}:      {cancelledCOD, []Command{CmdEmailCancelled}},

	{awaitingCard, EventPaymentSucceeded}: {pendingCard, []Command{CmdPlaceSupplierOrder, CmdEmailReceived}},
	{awaitingCard, EventPaymentFailed}:    {cancelledCard, nil},
	{awaitingCard, EventPaymentTimedOut}:  {cancelledCard, nil},
	{awaitingCard, EventCancel}:           {cancelledCard, []Command{CmdExpirePayment, CmdEmailCancelled}},

	{pendingCOD, EventSupplierConfirmed}: {readyCOD, []Command{CmdCreateShipment, CmdEmailConfirmed}},
	{pendingCOD, EventSupplierRejected}:  {cancelledCOD, []Command{CmdEmailCancelled}},
	{pendingCOD, EventCancel}:            {cancelledCOD, []Command{CmdEmailCancelled}},

	{pendingCard, EventSupplierConfirmed}: {readyCard, []Command{CmdCreateShipment, CmdEmailConfirmed}},
	{pendingCard, EventSupplierRejected}:  {refundingCard, []Command{CmdRefundPayment, CmdEmailCancelled}},
	{pendingCard, EventCancel}:            {refundingCard, []Command{CmdRefundPayment, CmdEmailCancelled}},

	{readyCOD, EventPickedUp}: {shippedCOD, []Command{CmdEmailShipped}},
	{readyCOD, EventCancel}:   {cancelledCOD, []Command{CmdEmailCancelled}},

	{readyCard, EventPickedUp}: {shippedCard, []Command{CmdEmailShipped}},
	{readyCard, EventCancel}:   {refundingCard, []Command{CmdRefundPayment, CmdEmailCancelled}},

	{shippedCOD, EventPaymentSucceeded}: {shippedCODPaid, nil},
	{shippedCOD, EventDelivered}:        {deliveredCOD, []Command{CmdEmailDelivered}},
	{shippedCODPaid, EventDelivered}:    {deliveredCODPaid, []Command{CmdEmailDelivered}},
	{shippedCard, EventDelivered}:       {deliveredCard, []Command{CmdEmailDelivered}},

	{deliveredCOD, EventPaymentSucceeded}: {deliveredCODPaid, nil},

	{refundingCard, EventRefundSucceeded}: {refundedCard, nil},

	// Paid after it timed out: the money goes back.
	{cancelledCard, EventPaymentSucceeded}: {refundingCard, []Command{CmdRefundPayment}},
}

// noops is every event an order has already moved past: it changes nothing, without an error.
var noops = map[state][]Event{
	pendingCOD:       {EventCODSelected},
	pendingCard:      {EventPaymentSucceeded, EventPaymentFailed, EventPaymentTimedOut},
	readyCOD:         {EventCODSelected, EventSupplierConfirmed},
	readyCard:        {EventPaymentSucceeded, EventPaymentFailed, EventPaymentTimedOut, EventSupplierConfirmed},
	shippedCOD:       {EventCODSelected, EventSupplierConfirmed, EventPickedUp},
	shippedCODPaid:   {EventCODSelected, EventPaymentSucceeded, EventSupplierConfirmed, EventPickedUp},
	shippedCard:      {EventPaymentSucceeded, EventPaymentFailed, EventPaymentTimedOut, EventSupplierConfirmed, EventPickedUp},
	deliveredCOD:     {EventCODSelected, EventSupplierConfirmed, EventPickedUp, EventDelivered},
	deliveredCODPaid: {EventCODSelected, EventPaymentSucceeded, EventSupplierConfirmed, EventPickedUp, EventDelivered},
	deliveredCard:    {EventPaymentSucceeded, EventPaymentFailed, EventPaymentTimedOut, EventSupplierConfirmed, EventPickedUp, EventDelivered},
	refundingCard:    {EventPaymentSucceeded, EventPaymentFailed, EventPaymentTimedOut, EventSupplierConfirmed, EventSupplierRejected, EventCancel},
	cancelledCOD:     {EventCODSelected, EventSupplierConfirmed, EventSupplierRejected, EventCancel},
	cancelledCard:    {EventPaymentFailed, EventPaymentTimedOut, EventSupplierConfirmed, EventSupplierRejected, EventCancel},
	refundedCard:     {EventPaymentSucceeded, EventPaymentFailed, EventPaymentTimedOut, EventSupplierConfirmed, EventSupplierRejected, EventRefundSucceeded, EventCancel},
}

func orderIn(s state, paidAt time.Time) Order {
	o := Order{ID: uuid.New(), Status: s.status, Payment: s.payment, Seq: 3}
	if s.paid {
		o.PaidAt = &paidAt
	}
	return o
}

// Every event in every reachable state: it's a transition, a no-op or invalid, and nothing else.
func TestTransition(t *testing.T) {
	earlier := time.Date(2026, 10, 7, 9, 0, 0, 0, time.UTC)
	now := earlier.Add(time.Hour)

	for _, from := range reachable {
		for _, ev := range allEvents {
			t.Run(from.String()+"/"+string(ev), func(t *testing.T) {
				o := orderIn(from, earlier)
				next, cmds, err := o.Transition(ev, now)

				want, moves := transitions[move{from, ev}]
				switch {
				case moves:
					if err != nil {
						t.Fatalf("err %v", err)
					}
					got := state{next.Status, next.Payment, next.PaidAt != nil}
					if got != want.to {
						t.Errorf("to %s, want %s", got, want.to)
					}
					if !slices.Equal(cmds, want.cmds) {
						t.Errorf("commands %v, want %v", cmds, want.cmds)
					}
					if next.Seq != o.Seq+1 {
						t.Errorf("seq %d, want %d", next.Seq, o.Seq+1)
					}
					// A payment is dated when it arrives; an earlier one keeps its date.
					switch {
					case !from.paid && want.to.paid && !next.PaidAt.Equal(now):
						t.Errorf("paid at %v, want %v", next.PaidAt, now)
					case from.paid && !next.PaidAt.Equal(earlier):
						t.Errorf("paid at %v, want %v", next.PaidAt, earlier)
					}

				case slices.Contains(noops[from], ev):
					if err != nil {
						t.Fatalf("err %v", err)
					}
					if next != o || cmds != nil {
						t.Errorf("a no-op gave %+v, commands %v", next, cmds)
					}

				default:
					if !errors.Is(err, ErrInvalidTransition) {
						t.Fatalf("err %v, want ErrInvalidTransition", err)
					}
					if next != o || cmds != nil {
						t.Errorf("an invalid event gave %+v, commands %v", next, cmds)
					}
				}
			})
		}
	}
}

// The tables only list reachable states: a typo there would test nothing.
func TestTransitionTablesUseReachableStates(t *testing.T) {
	for m, r := range transitions {
		if !slices.Contains(reachable, m.from) || !slices.Contains(reachable, r.to) {
			t.Errorf("%s --%s--> %s: not reachable", m.from, m.ev, r.to)
		}
	}
	for s := range noops {
		if !slices.Contains(reachable, s) {
			t.Errorf("no-ops for %s: not reachable", s)
		}
	}
}

func TestTransitionUnknownEvent(t *testing.T) {
	o := orderIn(awaitingCard, time.Time{})
	if _, _, err := o.Transition("refund_failed", time.Now()); !errors.Is(err, ErrInvalidTransition) {
		t.Errorf("err %v", err)
	}
}

// Walking the happy paths from checkout to the door, and a late card payment to its refund.
func TestTransitionPaths(t *testing.T) {
	now := time.Date(2026, 10, 7, 9, 0, 0, 0, time.UTC)
	tests := []struct {
		name   string
		from   state
		events []Event
		want   state
	}{
		{"cash on delivery", awaitingCOD, []Event{
			EventCODSelected, EventSupplierConfirmed, EventPickedUp, EventPaymentSucceeded, EventDelivered,
		}, deliveredCODPaid},
		{"card", awaitingCard, []Event{
			EventPaymentSucceeded, EventSupplierConfirmed, EventPickedUp, EventDelivered,
		}, deliveredCard},
		{"paid too late", awaitingCard, []Event{
			EventPaymentTimedOut, EventPaymentSucceeded, EventRefundSucceeded,
		}, refundedCard},
		{"rejected by the supplier", awaitingCard, []Event{
			EventPaymentSucceeded, EventSupplierRejected, EventRefundSucceeded,
		}, refundedCard},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := orderIn(tt.from, now)
			o.Seq = 0
			for _, ev := range tt.events {
				var err error
				if o, _, err = o.Transition(ev, now); err != nil {
					t.Fatalf("%s: %v", ev, err)
				}
			}
			if got := (state{o.Status, o.Payment, o.PaidAt != nil}); got != tt.want {
				t.Errorf("ended %s, want %s", got, tt.want)
			}
			if o.Seq != len(tt.events) {
				t.Errorf("seq %d, want %d", o.Seq, len(tt.events))
			}
		})
	}
}
