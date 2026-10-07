package orders

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/riverqueue/river"

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
}

// StepWorker carries out StepArgs jobs. A job can run more than once (a retry after the work
// was done but before the job was marked so), so an email can go out twice; that's the price of
// never sending one from inside a transaction.
type StepWorker struct {
	river.WorkerDefaults[StepArgs]
	svc    *Service
	mailer Mailer
}

func NewStepWorker(svc *Service, m Mailer) *StepWorker {
	return &StepWorker{svc: svc, mailer: m}
}

func (w *StepWorker) Work(ctx context.Context, job *river.Job[StepArgs]) error {
	switch job.Args.Command {
	case CmdEmailReceived:
		o, err := w.svc.store.order(ctx, w.svc.pool, job.Args.OrderID)
		if errors.Is(err, ErrNotFound) {
			return nil // the order is gone
		}
		if err != nil {
			return err
		}
		return w.mailer.OrderReceived(ctx, orderEmail(o))
	}
	// TODO(phase-7): the supplier order, shipment, refund and the other emails come with
	// fulfillment (step 6), Stripe (step 5) and Speedy (step 7). Until then their jobs are
	// cancelled, so they show in River's UI without failing again and again.
	return river.JobCancel(fmt.Errorf("orders: %s isn't carried out yet", job.Args.Command))
}

func orderEmail(o OrderDetail) notify.Order {
	lines := make([]notify.OrderLine, len(o.Lines))
	for i, l := range o.Lines {
		lines[i] = notify.OrderLine{Name: l.Name, Quantity: l.Quantity, LineCents: l.TotalCents}
	}
	return notify.Order{
		ID:             o.ID,
		Number:         o.Number,
		Token:          o.Token,
		Email:          o.Email,
		Name:           o.Name,
		Phone:          o.Phone,
		Address:        o.Address + ", " + o.Postcode + " " + o.City,
		Lines:          lines,
		SubtotalCents:  o.SubtotalCents,
		DeliveryCents:  o.DeliveryCents,
		TotalCents:     o.TotalCents,
		CashOnDelivery: o.Payment == PaymentCOD,
	}
}
