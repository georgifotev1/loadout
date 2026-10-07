-- +goose Up
-- The orders module owns this schema: carts now, orders later in Phase 7.
CREATE SCHEMA orders;

-- A shopper's cart, found by the token in its cookie. Only the token's SHA-256 is stored, like
-- auth.sessions. A cart is created by the first add to cart, so browsing alone creates nothing.
-- TODO(phase-10): delete carts left untouched for a long time.
CREATE TABLE orders.carts (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    token_hash bytea NOT NULL UNIQUE,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

-- One line per product. product_id is a catalog id, without a foreign key (see
-- specs.product_specs). slug and name are as they were when the product was added, to show a
-- line whose product is no longer for sale.
CREATE TABLE orders.cart_lines (
    cart_id    uuid NOT NULL REFERENCES orders.carts (id) ON DELETE CASCADE,
    product_id uuid NOT NULL,
    slug       text NOT NULL,
    name       text NOT NULL,
    quantity   integer NOT NULL CHECK (quantity BETWEEN 1 AND 10),
    added_at   timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (cart_id, product_id)
);

-- +goose Down
DROP TABLE orders.cart_lines;
DROP TABLE orders.carts;
DROP SCHEMA orders;
