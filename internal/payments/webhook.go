package payments

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stripe/stripe-go/v86"

	"github.com/gfotev/loadout/internal/orders"
	"github.com/gfotev/loadout/internal/platform/db"
)

// HandleWebhook verifies a Stripe event and applies it to its order. Stripe sends each event at
// least once and in no particular order, so the event's id is inserted in the transaction that
// applies it: a second delivery finds it and changes nothing, and an event the order has moved
// past is a no-op in the state machine.
//
// The endpoint in Stripe's Dashboard sends: checkout.session.completed,
// checkout.session.async_payment_succeeded, checkout.session.async_payment_failed,
// checkout.session.expired, refund.created, refund.updated and refund.failed.
//
// An error other than ErrBadSignature is worth a retry: Stripe sends the event again later.
func (s *Service) HandleWebhook(ctx context.Context, payload []byte, signature string) error {
	ev, err := s.stripe.ConstructEvent(payload, signature, s.cfg.WebhookSecret)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrBadSignature, err)
	}
	orderID, oev, err := s.orderEvent(ev)
	if err != nil || oev == "" {
		return err
	}
	log := s.cfg.Logger.With("stripe_event", ev.ID, "type", ev.Type, "order_id", orderID)

	return db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		query := `INSERT INTO payments.stripe_events (event_id, type) VALUES ($1, $2) ON CONFLICT DO NOTHING`
		tag, err := tx.Exec(ctx, query, ev.ID, ev.Type)
		if err != nil {
			return fmt.Errorf("payments: record stripe event %s: %w", ev.ID, err)
		}
		if tag.RowsAffected() == 0 {
			return nil // handled already
		}
		err = s.orders.ApplyTx(ctx, tx, orderID, oev)
		switch {
		case errors.Is(err, orders.ErrNotFound):
			// Another environment's order on the same Stripe account; nothing to do here.
			log.Warn("stripe event for an unknown order")
			return nil
		case errors.Is(err, orders.ErrInvalidTransition):
			// Sending it again won't help: it needs a person.
			log.Error("stripe event doesn't fit the order", "err", err)
			return nil
		}
		return err
	})
}

// orderEvent maps a Stripe event to the order event it means, or "" for one that means nothing
// to an order (yet).
func (s *Service) orderEvent(ev stripe.Event) (uuid.UUID, orders.Event, error) {
	switch ev.Type {
	case stripe.EventTypeCheckoutSessionCompleted,
		stripe.EventTypeCheckoutSessionAsyncPaymentSucceeded,
		stripe.EventTypeCheckoutSessionAsyncPaymentFailed,
		stripe.EventTypeCheckoutSessionExpired:
		var cs stripe.CheckoutSession
		if err := json.Unmarshal(ev.Data.Raw, &cs); err != nil {
			return uuid.Nil, "", fmt.Errorf("payments: stripe event %s: %w", ev.ID, err)
		}
		id, err := uuid.Parse(cs.ClientReferenceID)
		if err != nil {
			s.cfg.Logger.Warn("stripe checkout session without an order", "stripe_event", ev.ID, "session", cs.ID)
			return uuid.Nil, "", nil
		}
		switch ev.Type {
		case stripe.EventTypeCheckoutSessionAsyncPaymentFailed:
			return id, orders.EventPaymentFailed, nil
		case stripe.EventTypeCheckoutSessionExpired:
			return id, orders.EventPaymentTimedOut, nil
		}
		// Completed but unpaid is a bank debit on its way: its async event follows.
		if cs.PaymentStatus != stripe.CheckoutSessionPaymentStatusPaid {
			return id, "", nil
		}
		return id, orders.EventPaymentSucceeded, nil

	case stripe.EventTypeRefundCreated, stripe.EventTypeRefundUpdated, stripe.EventTypeRefundFailed:
		var r stripe.Refund
		if err := json.Unmarshal(ev.Data.Raw, &r); err != nil {
			return uuid.Nil, "", fmt.Errorf("payments: stripe event %s: %w", ev.ID, err)
		}
		id, err := uuid.Parse(r.Metadata["order_id"])
		if err != nil {
			return uuid.Nil, "", nil // a refund made in the Dashboard
		}
		switch r.Status {
		case stripe.RefundStatusSucceeded:
			return id, orders.EventRefundSucceeded, nil
		case stripe.RefundStatusFailed, stripe.RefundStatusCanceled:
			// The order stays refunding, in the admin's "needs action" list: they return the
			// money another way and mark it refunded.
			s.cfg.Logger.Error("stripe refund failed", "stripe_event", ev.ID, "refund", r.ID, "order_id", id, "status", r.Status)
		}
		return id, "", nil
	}
	return uuid.Nil, "", nil
}
