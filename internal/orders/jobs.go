package orders

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/riverqueue/river"

	"github.com/gfotev/loadout/internal/carrier"
	"github.com/gfotev/loadout/internal/notify"
	"github.com/gfotev/loadout/internal/platform/jobs"
)

// StepArgs is the job that carries out one command of an order's transition. Seq is the
// transition that asked for it, so the same command from a later transition is another job,
// and the worker can tell whether the order has moved on since.
type StepArgs struct {
	OrderID uuid.UUID `json:"order_id"`
	Command Command   `json:"command"`
	Seq     int       `json:"seq"`
}

func (StepArgs) Kind() string { return "order_step" }

func (StepArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{
		Queue:       jobs.QueueOrders,
		MaxAttempts: 5,
		UniqueOpts:  river.UniqueOpts{ByArgs: true},
	}
}

// Mailer sends the order emails. Declared here, by the consumer: *notify.Service.
type Mailer interface {
	OrderReceived(ctx context.Context, o notify.Order) error
	OrderConfirmed(ctx context.Context, o notify.Order) error
	OrderCancelled(ctx context.Context, o notify.Order) error
	OrderShipped(ctx context.Context, o notify.Order) error
	OrderDelivered(ctx context.Context, o notify.Order) error
}

// Payments is the card payment provider as orders needs it: *payments.Service. Declared here,
// by the consumer.
type Payments interface {
	// Outcome is what became of an order's card payment: EventPaymentSucceeded,
	// EventPaymentFailed or EventPaymentTimedOut, or "" while a payment method that takes days
	// (a bank debit) is still on its way.
	Outcome(ctx context.Context, orderID uuid.UUID) (Event, error)
	// Refund gives an order's card payment back, all of it. It reports whether the money is
	// back already; if not, the provider's webhook applies EventRefundSucceeded later.
	Refund(ctx context.Context, orderID uuid.UUID) (bool, error)
	// Expire closes an order's payment page if it's still open, so it can't be paid any more.
	Expire(ctx context.Context, orderID uuid.UUID) error
}

// StepWorker carries out StepArgs jobs. A job can run more than once (a retry after the work
// was done but before the job was marked so), so an email can go out twice; that's the price of
// never sending one from inside a transaction. Calls to the outside use idempotency keys.
type StepWorker struct {
	river.WorkerDefaults[StepArgs]
	svc      *Service
	mailer   Mailer
	payments Payments
	carrier  carrier.Carrier
}

// NewStepWorker returns the order_step worker. payments is nil when card payments aren't set up;
// refunds fail then, and wait in River's UI for a person. c makes the orders' waybills.
func NewStepWorker(svc *Service, m Mailer, p Payments, c carrier.Carrier) *StepWorker {
	return &StepWorker{svc: svc, mailer: m, payments: p, carrier: c}
}

func (w *StepWorker) Work(ctx context.Context, job *river.Job[StepArgs]) error {
	switch job.Args.Command {
	case CmdPlaceSupplierOrder:
		// Placed by hand: the admin orders the lines from the supplier and applies the
		// supplier's answer on the order's admin page.
		// TODO(vali-api): place the order through the supplier's API, once it takes orders.
		return nil

	case CmdCreateShipment:
		return w.createShipment(ctx, job.Args.OrderID)

	case CmdEmailReceived, CmdEmailConfirmed, CmdEmailCancelled, CmdEmailShipped, CmdEmailDelivered:
		o, err := w.svc.store.order(ctx, w.svc.pool, job.Args.OrderID)
		if errors.Is(err, ErrNotFound) {
			return nil // the order is gone
		}
		if err != nil {
			return err
		}
		switch job.Args.Command {
		case CmdEmailReceived:
			return w.mailer.OrderReceived(ctx, orderEmail(o))
		case CmdEmailConfirmed:
			if o.Status == StatusRefunding || o.Status == StatusCancelled {
				return nil // cancelled meanwhile: that email says so
			}
			return w.mailer.OrderConfirmed(ctx, orderEmail(o))
		case CmdEmailShipped:
			return w.mailer.OrderShipped(ctx, orderEmail(o))
		case CmdEmailDelivered:
			return w.mailer.OrderDelivered(ctx, orderEmail(o))
		default:
			return w.mailer.OrderCancelled(ctx, orderEmail(o))
		}

	case CmdExpirePayment:
		if w.payments == nil {
			return nil // no card payments, no payment page
		}
		return w.payments.Expire(ctx, job.Args.OrderID)

	case CmdRefundPayment:
		o, err := w.svc.store.order(ctx, w.svc.pool, job.Args.OrderID)
		if errors.Is(err, ErrNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		if o.Status != StatusRefunding {
			return nil // refunded already
		}
		if w.payments == nil {
			return fmt.Errorf("orders: refund order %s: card payments aren't set up", o.ID)
		}
		done, err := w.payments.Refund(ctx, o.ID)
		if err != nil || !done {
			return err
		}
		return w.svc.Apply(ctx, o.ID, EventRefundSucceeded)
	}
	// A command this worker doesn't know is a bug: cancelled, so it shows in River's UI without
	// failing again and again.
	return river.JobCancel(fmt.Errorf("orders: unknown command %s", job.Args.Command))
}

// PaymentTimeoutArgs is the job that settles a card order still waiting for its payment once
// PaymentWindow is over. Stripe's events normally settle it first; this catches the ones that
// never came, and the order whose payment page was never created.
type PaymentTimeoutArgs struct {
	OrderID uuid.UUID `json:"order_id"`
}

func (PaymentTimeoutArgs) Kind() string { return "order_payment_timeout" }

func (PaymentTimeoutArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{
		Queue:       jobs.QueueOrders,
		MaxAttempts: 5,
		UniqueOpts:  river.UniqueOpts{ByArgs: true},
	}
}

// PaymentTimeoutWorker carries out PaymentTimeoutArgs jobs: it asks the payment provider what
// became of the payment and applies that.
type PaymentTimeoutWorker struct {
	river.WorkerDefaults[PaymentTimeoutArgs]
	svc      *Service
	payments Payments
}

// NewPaymentTimeoutWorker returns the order_payment_timeout worker. payments is nil when card
// payments aren't set up: a card order then can't have been paid, so it times out.
func NewPaymentTimeoutWorker(svc *Service, p Payments) *PaymentTimeoutWorker {
	return &PaymentTimeoutWorker{svc: svc, payments: p}
}

func (w *PaymentTimeoutWorker) Work(ctx context.Context, job *river.Job[PaymentTimeoutArgs]) error {
	o, err := w.svc.store.order(ctx, w.svc.pool, job.Args.OrderID)
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if o.Status != StatusAwaitingPayment {
		return nil // settled already
	}
	ev := EventPaymentTimedOut
	if w.payments != nil {
		if ev, err = w.payments.Outcome(ctx, o.ID); err != nil || ev == "" {
			return err
		}
	}
	return w.svc.Apply(ctx, o.ID, ev)
}

func orderEmail(o OrderDetail) notify.Order {
	lines := make([]notify.OrderLine, len(o.Lines))
	for i, l := range o.Lines {
		lines[i] = notify.OrderLine{Name: l.Name, Quantity: l.Quantity, LineCents: l.TotalCents, InBuild: l.InBuild}
	}
	e := notify.Order{
		ID:             o.ID,
		Number:         o.Number,
		Token:          o.Token,
		Email:          o.Email,
		Name:           o.Name,
		Phone:          o.Phone,
		Address:        deliveryAddress(o),
		Lines:          lines,
		SubtotalCents:  o.SubtotalCents,
		DeliveryCents:  o.DeliveryCents,
		AssemblyCents:  o.AssemblyCents,
		BuildCents:     o.BuildCents,
		TotalCents:     o.TotalCents,
		Assembly:       o.Assembly,
		CashOnDelivery: o.Payment == PaymentCOD,
		PaidByCard:     o.Payment == PaymentCard && o.PaidAt != nil,
	}
	if o.Shipment != nil {
		e.TrackingNumber = o.Shipment.Number
		e.TrackingURL = o.Shipment.TrackingURL
	}
	return e
}

// deliveryAddress is where the parcel goes, on one line: the street, postcode and city, after
// the office's name for an office or a locker.
func deliveryAddress(o OrderDetail) string {
	addr := o.Address + ", " + o.Postcode + " " + o.City
	switch o.Delivery {
	case DeliveryOffice:
		return "офис „" + o.OfficeName + "“, " + addr
	case DeliveryAPT:
		return "автомат „" + o.OfficeName + "“, " + addr
	}
	return addr
}
