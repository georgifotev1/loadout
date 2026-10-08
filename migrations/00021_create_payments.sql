-- +goose Up
-- The payments module owns this schema: what we keep of card payments through Stripe.
CREATE SCHEMA payments;

-- The Stripe Checkout Session an order is paid through. An order has at most one: it's created
-- with the order id as the idempotency key, and when it expires the order is cancelled.
-- order_id is an orders id, without a foreign key (one module, one schema).
CREATE TABLE payments.checkouts (
    order_id   uuid PRIMARY KEY,
    session_id text NOT NULL UNIQUE,
    -- Stripe's payment page, where the customer goes to pay, until expires_at.
    url        text NOT NULL,
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

-- The Stripe events already handled. Stripe sends an event at least once; its id is inserted in
-- the transaction that applies it to the order, so a second delivery changes nothing.
-- TODO(phase-10): delete rows older than Stripe's retry window (3 days).
CREATE TABLE payments.stripe_events (
    event_id    text PRIMARY KEY,
    type        text NOT NULL,
    received_at timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE payments.stripe_events;
DROP TABLE payments.checkouts;
DROP SCHEMA payments;
