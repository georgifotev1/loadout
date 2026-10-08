package orders

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// ErrInvalidTransition is returned for an event the order can't take in its state, such as a
// shipment for a cancelled order. It points at a bug or a mix-up that a person has to look at.
var ErrInvalidTransition = errors.New("orders: invalid transition")

// Status is where an order is in its lifecycle:
//
//	awaiting_payment ── payment_succeeded, cod_selected ──► supplier_pending ── supplier_confirmed ──► ready_to_ship
//	  │ payment_failed, payment_timed_out                     │ supplier_rejected, cancel               │ shipment_created
//	  ▼                                                       ▼                                         ▼
//	cancelled ◄── unpaid ─────────────────────────────── (stopped) ── paid ──► refunding               shipped ── delivered ──► delivered
//	  ▲                                                                           │ refund_succeeded
//	  └───────────────────────────────────────────────────────────────────────────┘
//
// An admin can cancel any order that hasn't shipped.
// Whether the order is paid is kept apart (Order.PaidAt): cash on delivery is paid only when the
// courier collects it, while the order is shipped or delivered.
type Status string

const (
	StatusAwaitingPayment Status = "awaiting_payment"
	StatusSupplierPending Status = "supplier_pending" // ordered from the supplier, waiting for their confirmation
	StatusReadyToShip     Status = "ready_to_ship"
	StatusShipped         Status = "shipped"
	StatusDelivered       Status = "delivered"
	StatusRefunding       Status = "refunding"
	StatusCancelled       Status = "cancelled" // a paid cancelled order has been refunded
)

// Payment is how the customer pays.
type Payment string

const (
	PaymentCOD  Payment = "cod" // cash on delivery, collected by the courier
	PaymentCard Payment = "card"
)

// Event is something that happened to an order: a payment, the supplier's answer, a courier's
// update or an admin's action.
type Event string

const (
	EventCODSelected       Event = "cod_selected" // checkout with cash on delivery: nothing to wait for
	EventPaymentSucceeded  Event = "payment_succeeded"
	EventPaymentFailed     Event = "payment_failed"
	EventPaymentTimedOut   Event = "payment_timed_out"
	EventSupplierConfirmed Event = "supplier_confirmed"
	EventSupplierRejected  Event = "supplier_rejected"
	EventShipmentCreated   Event = "shipment_created"
	EventDelivered         Event = "delivered"
	EventRefundSucceeded   Event = "refund_succeeded"
	EventCancel            Event = "cancel" // by an admin
)

// Command is work a transition asks for. Apply queues one order_step job per command, in the
// transaction that saves the transition.
type Command string

const (
	CmdPlaceSupplierOrder Command = "place_supplier_order"
	CmdCreateShipment     Command = "create_shipment"
	CmdRefundPayment      Command = "refund_payment"
	CmdExpirePayment      Command = "expire_payment" // close the payment page of an order cancelled unpaid
	CmdEmailReceived      Command = "email_received"
	CmdEmailConfirmed     Command = "email_confirmed"
	CmdEmailShipped       Command = "email_shipped"
	CmdEmailDelivered     Command = "email_delivered"
	CmdEmailCancelled     Command = "email_cancelled"
)

// Order is the part of an order the state machine works on.
type Order struct {
	ID      uuid.UUID
	Status  Status
	Payment Payment
	PaidAt  *time.Time
	// Seq counts the transitions the order has made. Transition adds one when it changes the
	// order, so an unchanged Seq means the event was a no-op.
	Seq int
}

func (o Order) paid() bool { return o.PaidAt != nil }

// Transition applies ev and returns the order after it and the commands it causes. It's pure:
// Apply loads, locks and saves around it.
//
// An event the order has already moved past (a webhook delivered twice, a supplier confirming
// an order that is cancelled meanwhile) is a no-op: the order comes back unchanged with no
// commands and no error. An event that makes no sense in the order's state is
// ErrInvalidTransition.
func (o Order) Transition(ev Event, now time.Time) (Order, []Command, error) {
	next := o
	var cmds []Command
	to := func(s Status, c ...Command) {
		next.Status = s
		cmds = c
	}
	// cancel stops the order. A paid one waits for its refund first.
	cancel := func(c ...Command) {
		if o.paid() {
			to(StatusRefunding, append([]Command{CmdRefundPayment}, c...)...)
			return
		}
		to(StatusCancelled, c...)
	}

	switch ev {
	case EventCODSelected:
		switch {
		case o.Payment != PaymentCOD:
			return o.invalid(ev)
		case o.Status == StatusAwaitingPayment:
			to(StatusSupplierPending, CmdPlaceSupplierOrder, CmdEmailReceived)
		default:
			return o, nil, nil
		}

	case EventPaymentSucceeded:
		if o.paid() {
			return o, nil, nil
		}
		switch {
		case o.Payment == PaymentCard && o.Status == StatusAwaitingPayment:
			to(StatusSupplierPending, CmdPlaceSupplierOrder, CmdEmailReceived)
		case o.Payment == PaymentCard && o.Status == StatusCancelled:
			// Paid after the order timed out or was cancelled: give the money back.
			to(StatusRefunding, CmdRefundPayment)
		case o.Payment == PaymentCOD && (o.Status == StatusShipped || o.Status == StatusDelivered):
			// The courier collected the cash; the order goes on as it was.
		default:
			return o.invalid(ev)
		}
		next.PaidAt = &now

	case EventPaymentFailed, EventPaymentTimedOut:
		switch {
		case o.Payment != PaymentCard:
			return o.invalid(ev)
		case o.Status == StatusAwaitingPayment:
			// The customer never paid, so there's no one to tell.
			to(StatusCancelled)
		default:
			return o, nil, nil
		}

	case EventSupplierConfirmed:
		switch o.Status {
		case StatusSupplierPending:
			to(StatusReadyToShip, CmdCreateShipment, CmdEmailConfirmed)
		case StatusReadyToShip, StatusShipped, StatusDelivered, StatusRefunding, StatusCancelled:
			// Cancelled meanwhile: what to do with the supplier's parts is the admin's call.
			return o, nil, nil
		default:
			return o.invalid(ev)
		}

	case EventSupplierRejected:
		switch o.Status {
		case StatusSupplierPending:
			cancel(CmdEmailCancelled)
		case StatusRefunding, StatusCancelled:
			return o, nil, nil
		default:
			return o.invalid(ev)
		}

	case EventShipmentCreated:
		switch o.Status {
		case StatusReadyToShip:
			to(StatusShipped, CmdEmailShipped)
		case StatusShipped, StatusDelivered:
			return o, nil, nil
		default:
			return o.invalid(ev)
		}

	case EventDelivered:
		switch o.Status {
		case StatusShipped:
			to(StatusDelivered, CmdEmailDelivered)
		case StatusDelivered:
			return o, nil, nil
		default:
			return o.invalid(ev)
		}

	case EventRefundSucceeded:
		switch {
		case o.Status == StatusRefunding:
			to(StatusCancelled)
		case o.Status == StatusCancelled && o.paid():
			return o, nil, nil
		default:
			return o.invalid(ev)
		}

	case EventCancel:
		switch o.Status {
		case StatusAwaitingPayment:
			if o.Payment == PaymentCard {
				// The customer could still pay on the open page; one paying anyway is refunded.
				to(StatusCancelled, CmdExpirePayment, CmdEmailCancelled)
			} else {
				to(StatusCancelled, CmdEmailCancelled)
			}
		case StatusSupplierPending, StatusReadyToShip:
			cancel(CmdEmailCancelled)
		case StatusRefunding, StatusCancelled:
			return o, nil, nil
		default:
			// A shipped order comes back as a return, not a cancellation.
			return o.invalid(ev)
		}

	default:
		return o.invalid(ev)
	}

	next.Seq++
	return next, cmds, nil
}

func (o Order) invalid(ev Event) (Order, []Command, error) {
	return o, nil, fmt.Errorf("%w: %s on a %s %s order", ErrInvalidTransition, ev, o.Status, o.Payment)
}
