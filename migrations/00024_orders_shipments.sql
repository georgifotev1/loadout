-- +goose Up
-- The courier's waybill for an order, made once the supplier confirms it. One per order: the
-- create_shipment job checks for it first, so a retry doesn't make a second. Tracking moves the
-- order on (picked up, delivered); the label is fetched from the courier when it's printed.
CREATE TABLE orders.shipments (
    order_id     uuid PRIMARY KEY REFERENCES orders.orders (id) ON DELETE CASCADE,
    carrier      text NOT NULL,
    number       text NOT NULL,
    -- The courier's page that follows it; '' if it has none.
    tracking_url text NOT NULL DEFAULT '',
    created_at   timestamptz NOT NULL DEFAULT now(),
    UNIQUE (carrier, number)
);

-- +goose Down
DROP TABLE orders.shipments;
