package orders

import (
	"github.com/google/uuid"
	"github.com/riverqueue/river"

	"github.com/gfotev/loadout/internal/platform/jobs"
)

// StepArgs is the job that carries out one command of an order's transition. Seq is the
// transition that asked for it, so the same command from a later transition is another job,
// and the worker can tell whether the order has moved on since.
//
// TODO(phase-7): the worker comes with the first command it can carry out (the order email,
// with checkout); until then nothing creates orders, so no step is queued.
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
