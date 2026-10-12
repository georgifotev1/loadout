-- +goose Up
-- The version of the terms the customer accepted at checkout (the date they took effect), so a
-- dispute can tell which text applied. NULL for orders placed before checkout asked.
ALTER TABLE orders.orders ADD COLUMN terms_version text;

-- +goose Down
ALTER TABLE orders.orders DROP COLUMN terms_version;
