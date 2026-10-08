package orders

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/gfotev/loadout/internal/platform/db"
)

// Apply applies an event to an order in its own transaction. It's the only way an order changes.
func (s *Service) Apply(ctx context.Context, id uuid.UUID, ev Event) error {
	return db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		return s.ApplyTx(ctx, tx, id, ev)
	})
}

// ApplyTx applies an event in the caller's transaction, for callers whose own writes must commit
// with it (a webhook's event id, an admin task, a job's result). The order row stays locked until
// that transaction ends, so concurrent events on one order take turns.
//
// In the transaction it saves the new state, a transition row and one order_step job per
// command: all of them or none. An event the order has already moved past changes nothing.
func (s *Service) ApplyTx(ctx context.Context, tx pgx.Tx, id uuid.UUID, ev Event) error {
	return s.applyTx(ctx, tx, id, ev, Actor{})
}

func (s *Service) applyTx(ctx context.Context, tx pgx.Tx, id uuid.UUID, ev Event, by Actor) error {
	o, err := s.store.lockOrder(ctx, tx, id)
	if err != nil {
		return err
	}
	now := s.now()
	next, cmds, err := o.Transition(ev, now)
	if err != nil {
		return err
	}
	if next.Seq == o.Seq {
		return nil
	}
	if err := s.store.saveTransition(ctx, tx, o, next, ev, by, now); err != nil {
		return err
	}
	for _, c := range cmds {
		args := StepArgs{OrderID: id, Command: c, Seq: next.Seq}
		if _, err := s.jobs.InsertTx(ctx, tx, args, nil); err != nil {
			return fmt.Errorf("orders: enqueue %s for order %s: %w", c, id, err)
		}
	}
	return nil
}
