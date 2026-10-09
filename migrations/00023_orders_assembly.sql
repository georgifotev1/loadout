-- +goose Up
-- Assembly: the customer can have the supplier assemble the PC their order's parts make, for a
-- fee. in_build marks the lines that go into it; the rest (peripherals) ship as they are.

-- The shopper asked for assembly (in the builder, the cart or at checkout). Checkout assembles
-- only if the cart's parts still make one complete, compatible PC.
ALTER TABLE orders.carts ADD COLUMN assembly boolean NOT NULL DEFAULT false;

ALTER TABLE orders.orders
    ADD COLUMN assembly boolean NOT NULL DEFAULT false,
    -- The fee, euro cents with VAT, as the customer agreed to it.
    ADD COLUMN assembly_cents bigint NOT NULL DEFAULT 0 CHECK (assembly_cents >= 0),
    ADD CONSTRAINT orders_assembly_check CHECK (assembly OR assembly_cents = 0),
    DROP CONSTRAINT orders_total_check,
    ADD CONSTRAINT orders_total_check CHECK (total_cents = subtotal_cents + delivery_cents + assembly_cents);

ALTER TABLE orders.order_lines ADD COLUMN in_build boolean NOT NULL DEFAULT false;

-- +goose Down
-- Assembled orders lose their fee from the total check, so the down migration refuses them.
-- +goose StatementBegin
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM orders.orders WHERE assembly) THEN
        RAISE EXCEPTION 'orders with assembly exist';
    END IF;
END $$;
-- +goose StatementEnd
ALTER TABLE orders.order_lines DROP COLUMN in_build;
ALTER TABLE orders.carts DROP COLUMN assembly;
ALTER TABLE orders.orders
    DROP CONSTRAINT orders_total_check,
    DROP CONSTRAINT orders_assembly_check,
    DROP COLUMN assembly_cents,
    DROP COLUMN assembly,
    ADD CONSTRAINT orders_total_check CHECK (total_cents = subtotal_cents + delivery_cents);
