// Package payments takes card payments through Stripe Checkout: the payment page for an order,
// Stripe's webhook, what became of a payment, and refunds. It changes orders only through
// orders.ApplyTx; cash on delivery needs nothing from it.
package payments

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stripe/stripe-go/v86"

	"github.com/gfotev/loadout/internal/orders"
	"github.com/gfotev/loadout/internal/platform/db"
)

// ErrNotPayable is returned by Pay for an order that can't be paid by card (any more): cash on
// delivery, paid already, cancelled, or too close to the end of its payment window.
var ErrNotPayable = errors.New("payments: the order can't be paid")

// ErrBadSignature is returned by HandleWebhook for a request that isn't from Stripe, or one
// signed for another API version.
var ErrBadSignature = errors.New("payments: invalid webhook signature")

// minSessionLife is the least time Stripe lets a Checkout Session live.
const minSessionLife = 30 * time.Minute

// integrationID tags our Checkout Sessions in Stripe's Dashboard.
const integrationID = "loadout_hosted_checkout_vkqmrtzd"

// Orders is what payments needs from the orders module: *orders.Service. Declared here, by the
// consumer.
type Orders interface {
	ApplyTx(ctx context.Context, tx pgx.Tx, id uuid.UUID, ev orders.Event) error
}

// Config configures NewService.
type Config struct {
	// WebhookSecret verifies the webhook's signatures (whsec_…).
	WebhookSecret string
	// SiteURL is the storefront's origin: Stripe sends the customer back to the order page there.
	SiteURL string
	Logger  *slog.Logger
}

// Service is the payments module's public API.
type Service struct {
	pool   *pgxpool.Pool
	stripe *stripe.Client
	orders Orders
	cfg    Config
	now    func() time.Time
}

func NewService(pool *pgxpool.Pool, sc *stripe.Client, o Orders, cfg Config) *Service {
	return &Service{pool: pool, stripe: sc, orders: o, cfg: cfg, now: time.Now}
}

// NewStripeClient returns a Stripe client for key, best a restricted key (rk_…) that can only
// write Checkout Sessions and Refunds. apiURL is "" for Stripe's API; tests pass a fake. The
// SDK's own logging is off: errors come back to the caller, who logs them.
func NewStripeClient(key, apiURL string) *stripe.Client {
	cfg := &stripe.BackendConfig{LeveledLogger: &stripe.LeveledLogger{Level: stripe.LevelNull}}
	if apiURL != "" {
		cfg.URL = stripe.String(apiURL)
	}
	return stripe.NewClient(key, stripe.WithBackends(stripe.NewBackendsWithConfig(cfg)))
}

// Pay returns the address of Stripe's payment page for a card order waiting for its payment.
// The first call creates it; later ones (the customer came back from the page without paying)
// return the same page while it's open. If Stripe says the page was paid meanwhile (its webhook
// is late), the payment is applied and the order isn't payable any more.
//
// The Stripe call happens outside any transaction, with the order id as its idempotency key:
// if the process dies before the page is stored, the next call gets the same page back from
// Stripe instead of a second one.
func (s *Service) Pay(ctx context.Context, o orders.OrderDetail) (string, error) {
	if o.Payment != orders.PaymentCard || o.Status != orders.StatusAwaitingPayment {
		return "", ErrNotPayable
	}
	now := s.now()
	c, found, err := s.checkout(ctx, s.pool, o.ID)
	if err != nil {
		return "", err
	}
	if found {
		if !now.Before(c.expiresAt) {
			return "", ErrNotPayable
		}
		cs, err := s.settle(ctx, o.ID, c.sessionID)
		if err != nil {
			return "", err
		}
		if cs.Status != stripe.CheckoutSessionStatusOpen {
			return "", ErrNotPayable
		}
		return c.url, nil
	}

	// The window is the order's, not this call's, so a retry sends Stripe the same request.
	expiresAt := o.CreatedAt.Add(orders.PaymentWindow).Truncate(time.Second)
	if expiresAt.Sub(now) < minSessionLife+time.Minute {
		return "", ErrNotPayable
	}
	cs, err := s.stripe.V1CheckoutSessions.Create(ctx, s.sessionParams(o, expiresAt))
	if err != nil {
		return "", fmt.Errorf("payments: create the checkout session of order %s: %w", o.ID, err)
	}
	query := `
		INSERT INTO payments.checkouts (order_id, session_id, url, expires_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (order_id) DO NOTHING`
	if _, err := s.pool.Exec(ctx, query, o.ID, cs.ID, cs.URL, time.Unix(cs.ExpiresAt, 0)); err != nil {
		return "", fmt.Errorf("payments: store the checkout session of order %s: %w", o.ID, err)
	}
	return cs.URL, nil
}

func (s *Service) sessionParams(o orders.OrderDetail, expiresAt time.Time) *stripe.CheckoutSessionCreateParams {
	item := func(name string, cents int64, qty int) *stripe.CheckoutSessionCreateLineItemParams {
		return &stripe.CheckoutSessionCreateLineItemParams{
			PriceData: &stripe.CheckoutSessionCreateLineItemPriceDataParams{
				Currency:    stripe.String(string(stripe.CurrencyEUR)),
				UnitAmount:  stripe.Int64(cents),
				ProductData: &stripe.CheckoutSessionCreateLineItemPriceDataProductDataParams{Name: stripe.String(name)},
			},
			Quantity: stripe.Int64(int64(qty)),
		}
	}
	// Prices include VAT, as on the order; Stripe Tax isn't used.
	items := make([]*stripe.CheckoutSessionCreateLineItemParams, 0, len(o.Lines)+1)
	for _, l := range o.Lines {
		items = append(items, item(l.Name, int64(l.UnitCents), l.Quantity))
	}
	if o.DeliveryCents > 0 {
		items = append(items, item("Доставка", int64(o.DeliveryCents), 1))
	}

	page := s.cfg.SiteURL + "/order/" + o.ID.String() + "?" + url.Values{"token": {o.Token}}.Encode()
	id := o.ID.String()
	p := &stripe.CheckoutSessionCreateParams{
		Mode:                  stripe.String(string(stripe.CheckoutSessionModePayment)),
		LineItems:             items,
		ClientReferenceID:     stripe.String(id),
		CustomerEmail:         stripe.String(o.Email),
		Locale:                stripe.String("bg"),
		ExpiresAt:             stripe.Int64(expiresAt.Unix()),
		SuccessURL:            stripe.String(page + "&paid=1"),
		CancelURL:             stripe.String(page),
		IntegrationIdentifier: stripe.String(integrationID),
		PaymentIntentData: &stripe.CheckoutSessionCreatePaymentIntentDataParams{
			Description: stripe.String(fmt.Sprintf("Поръчка № %d", o.Number)),
		},
	}
	p.AddMetadata("order_id", id)
	p.PaymentIntentData.AddMetadata("order_id", id)
	p.SetIdempotencyKey("checkout-" + id)
	return p
}

// Sync asks Stripe about a card order waiting for its payment and applies the payment if its
// page was paid, without waiting for the webhook. The order page calls it when the customer
// comes back from paying. Stripe's answer decides, never the customer's browser, and applying a
// payment twice is a no-op, so it's safe to call any time.
func (s *Service) Sync(ctx context.Context, o orders.OrderDetail) error {
	if o.Payment != orders.PaymentCard || o.Status != orders.StatusAwaitingPayment {
		return nil
	}
	c, found, err := s.checkout(ctx, s.pool, o.ID)
	if err != nil || !found {
		return err
	}
	_, err = s.settle(ctx, o.ID, c.sessionID)
	return err
}

// settle retrieves an order's checkout session and applies its payment if it's paid.
func (s *Service) settle(ctx context.Context, orderID uuid.UUID, sessionID string) (*stripe.CheckoutSession, error) {
	cs, err := s.stripe.V1CheckoutSessions.Retrieve(ctx, sessionID, nil)
	if err != nil {
		return nil, fmt.Errorf("payments: retrieve the checkout session of order %s: %w", orderID, err)
	}
	if cs.PaymentStatus != stripe.CheckoutSessionPaymentStatusPaid {
		return cs, nil
	}
	err = db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		return s.orders.ApplyTx(ctx, tx, orderID, orders.EventPaymentSucceeded)
	})
	if err != nil {
		return nil, err
	}
	return cs, nil
}

// Outcome asks Stripe what became of an order's payment, for the order's timeout job. A page
// still open is expired first, so the customer can't pay after the order is cancelled.
func (s *Service) Outcome(ctx context.Context, orderID uuid.UUID) (orders.Event, error) {
	c, found, err := s.checkout(ctx, s.pool, orderID)
	if err != nil {
		return "", err
	}
	if !found {
		return orders.EventPaymentTimedOut, nil // the customer never got a payment page
	}
	cs, err := s.expireOpen(ctx, orderID, c.sessionID)
	if err != nil {
		return "", err
	}
	switch {
	case cs.Status == stripe.CheckoutSessionStatusExpired:
		return orders.EventPaymentTimedOut, nil
	case cs.PaymentStatus == stripe.CheckoutSessionPaymentStatusPaid:
		return orders.EventPaymentSucceeded, nil
	}
	return "", nil // complete but unpaid: a bank debit on its way; its webhook settles it
}

// Expire closes an order's payment page if it's still open, for an order cancelled before it
// was paid. A customer who completes the page first is refunded: the payment's webhook finds
// the order cancelled.
func (s *Service) Expire(ctx context.Context, orderID uuid.UUID) error {
	c, found, err := s.checkout(ctx, s.pool, orderID)
	if err != nil || !found {
		return err
	}
	_, err = s.expireOpen(ctx, orderID, c.sessionID)
	return err
}

// expireOpen retrieves a checkout session and expires it if it's open. It returns the session
// as it is after.
func (s *Service) expireOpen(ctx context.Context, orderID uuid.UUID, sessionID string) (*stripe.CheckoutSession, error) {
	cs, err := s.stripe.V1CheckoutSessions.Retrieve(ctx, sessionID, nil)
	if err != nil {
		return nil, fmt.Errorf("payments: retrieve the checkout session of order %s: %w", orderID, err)
	}
	if cs.Status == stripe.CheckoutSessionStatusOpen {
		// Fails if the customer completes the page meanwhile; the retry sees that.
		if cs, err = s.stripe.V1CheckoutSessions.Expire(ctx, sessionID, nil); err != nil {
			return nil, fmt.Errorf("payments: expire the checkout session of order %s: %w", orderID, err)
		}
	}
	return cs, nil
}

// Refund gives an order's payment back, all of it, with the order id as the idempotency key. It
// reports whether the money is back already; a pending refund's webhook settles it later.
func (s *Service) Refund(ctx context.Context, orderID uuid.UUID) (bool, error) {
	c, found, err := s.checkout(ctx, s.pool, orderID)
	if err != nil {
		return false, err
	}
	if !found {
		return false, fmt.Errorf("payments: refund order %s: no checkout session", orderID)
	}
	cs, err := s.stripe.V1CheckoutSessions.Retrieve(ctx, c.sessionID, nil)
	if err != nil {
		return false, fmt.Errorf("payments: retrieve the checkout session of order %s: %w", orderID, err)
	}
	if cs.PaymentIntent == nil {
		return false, fmt.Errorf("payments: refund order %s: the checkout session has no payment", orderID)
	}
	p := &stripe.RefundCreateParams{PaymentIntent: stripe.String(cs.PaymentIntent.ID)}
	p.AddMetadata("order_id", orderID.String())
	p.SetIdempotencyKey("refund-" + orderID.String())
	r, err := s.stripe.V1Refunds.Create(ctx, p)
	if err != nil {
		return false, fmt.Errorf("payments: refund order %s: %w", orderID, err)
	}
	switch r.Status {
	case stripe.RefundStatusSucceeded:
		return true, nil
	case stripe.RefundStatusFailed, stripe.RefundStatusCanceled:
		return false, fmt.Errorf("payments: refund %s of order %s is %s", r.ID, orderID, r.Status)
	}
	return false, nil
}

type checkout struct {
	sessionID string
	url       string
	expiresAt time.Time
}

func (s *Service) checkout(ctx context.Context, q db.DBTX, orderID uuid.UUID) (checkout, bool, error) {
	query := `SELECT session_id, url, expires_at FROM payments.checkouts WHERE order_id = $1`

	var c checkout
	err := q.QueryRow(ctx, query, orderID).Scan(
		&c.sessionID,
		&c.url,
		&c.expiresAt,
	)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return checkout{}, false, nil
	case err != nil:
		return checkout{}, false, fmt.Errorf("payments: checkout of order %s: %w", orderID, err)
	}
	return c, true, nil
}
