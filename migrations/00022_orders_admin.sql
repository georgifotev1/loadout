-- +goose Up
-- Who applied a transition and why: an admin's email and note when they confirmed, rejected,
-- cancelled or marked an order refunded; '' for the system (checkout, Stripe, jobs). The email
-- is a copy, as the admin's account was at the time, without a link to auth.
ALTER TABLE orders.order_transitions
    ADD COLUMN actor text NOT NULL DEFAULT '',
    ADD COLUMN note text NOT NULL DEFAULT '';

-- When the order took its status, for "waiting since" and the stale flag. updated_at isn't
-- it: collecting cash on delivery updates a shipped order without changing its status.
ALTER TABLE orders.orders ADD COLUMN status_at timestamptz NOT NULL DEFAULT now();
UPDATE orders.orders o
SET status_at = coalesce(
    (SELECT max(t.at) FROM orders.order_transitions t
     WHERE t.order_id = o.id AND t.from_status <> t.to_status),
    o.created_at);

-- +goose Down
ALTER TABLE orders.orders DROP COLUMN status_at;
ALTER TABLE orders.order_transitions
    DROP COLUMN note,
    DROP COLUMN actor;
