-- +goose Up
-- An order's place in its lifecycle. Only orders.Apply changes it: it locks the row, runs the
-- state machine and writes the new state with an order_transitions row in one transaction.
-- What was ordered, who by and where it goes come with checkout.
CREATE TABLE orders.orders (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    status     text NOT NULL CHECK (status IN (
                   'awaiting_payment', 'supplier_pending', 'ready_to_ship', 'shipped',
                   'delivered', 'refunding', 'cancelled')),
    payment    text NOT NULL CHECK (payment IN ('cod', 'card')),
    -- When the money arrived: the card payment, or the courier collecting cash on delivery.
    paid_at    timestamptz,
    -- How many transitions the order has been through; the latest order_transitions.seq.
    seq        integer NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

-- Every transition an order made, in order: the audit log and the admin's timeline.
CREATE TABLE orders.order_transitions (
    order_id    uuid NOT NULL REFERENCES orders.orders (id) ON DELETE CASCADE,
    seq         integer NOT NULL,
    event       text NOT NULL,
    from_status text NOT NULL,
    to_status   text NOT NULL,
    at          timestamptz NOT NULL,
    PRIMARY KEY (order_id, seq)
);

-- +goose Down
DROP TABLE orders.order_transitions;
DROP TABLE orders.orders;
