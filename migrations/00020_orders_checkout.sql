-- +goose Up
-- What checkout writes on an order: who placed it, where it goes, what it costs, and its lines
-- as they were sold. Nothing created orders before checkout, so there are no rows to fill.
ALTER TABLE orders.orders
    -- The order's number for people: on the order page, in emails, on the phone.
    ADD COLUMN number bigint GENERATED ALWAYS AS IDENTITY (START WITH 1001) UNIQUE,
    -- The secret in the order page's link, so a guest can open it. It only shows this order,
    -- whose details are in this row anyway, so it's stored as is: emails need it again.
    ADD COLUMN token text NOT NULL UNIQUE,
    -- The customer logged in at checkout; NULL for a guest. An auth user id, without a foreign
    -- key (one module, one schema).
    ADD COLUMN user_id uuid,
    ADD COLUMN email citext NOT NULL,
    ADD COLUMN name text NOT NULL,
    ADD COLUMN phone text NOT NULL,
    ADD COLUMN city text NOT NULL,
    ADD COLUMN postcode text NOT NULL,
    ADD COLUMN address text NOT NULL,
    -- Euro cents with VAT, as the customer agreed to them.
    ADD COLUMN subtotal_cents bigint NOT NULL CHECK (subtotal_cents >= 0),
    ADD COLUMN delivery_cents bigint NOT NULL CHECK (delivery_cents >= 0),
    ADD COLUMN total_cents bigint NOT NULL,
    ADD CONSTRAINT orders_total_check CHECK (total_cents = subtotal_cents + delivery_cents);

-- An account's order history: its own orders and, once its email is verified, the guest
-- orders placed with that email.
CREATE INDEX orders_user_id_idx ON orders.orders (user_id) WHERE user_id IS NOT NULL;
CREATE INDEX orders_email_idx ON orders.orders (email);

-- The products of an order as they were sold. product_id is a catalog id, without a foreign
-- key; the rest is a copy, so the order stays the same when the catalog changes.
CREATE TABLE orders.order_lines (
    order_id     uuid NOT NULL REFERENCES orders.orders (id) ON DELETE CASCADE,
    position     integer NOT NULL,  -- the order the lines are shown in
    product_id   uuid NOT NULL,
    provider     text NOT NULL,     -- the supplier to order it from, and its SKU there
    provider_sku text NOT NULL,
    slug         text NOT NULL,
    name         text NOT NULL,
    image        text NOT NULL,     -- '' if the product had none
    quantity     integer NOT NULL CHECK (quantity >= 1),
    -- One unit, euro cents with VAT: what we pay the supplier, and what the customer pays.
    dealer_cents bigint NOT NULL CHECK (dealer_cents >= 0),
    retail_cents bigint NOT NULL CHECK (retail_cents >= 0),
    vat_bp       integer NOT NULL CHECK (vat_bp BETWEEN 0 AND 10000),
    PRIMARY KEY (order_id, product_id),
    UNIQUE (order_id, position)
);

-- +goose Down
DROP TABLE orders.order_lines;
ALTER TABLE orders.orders
    DROP CONSTRAINT orders_total_check,
    DROP COLUMN total_cents,
    DROP COLUMN delivery_cents,
    DROP COLUMN subtotal_cents,
    DROP COLUMN address,
    DROP COLUMN postcode,
    DROP COLUMN city,
    DROP COLUMN phone,
    DROP COLUMN name,
    DROP COLUMN email,
    DROP COLUMN user_id,
    DROP COLUMN token,
    DROP COLUMN number;
