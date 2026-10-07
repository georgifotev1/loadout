-- +goose Up
-- A logged-in customer's cart belongs to their account (user_id), so it follows them to other
-- devices; a guest's belongs to the cart cookie (token_hash). Logging in merges the guest cart
-- into the account's. user_id is an auth user id, without a foreign key (one module, one schema).
ALTER TABLE orders.carts
    ADD COLUMN user_id uuid UNIQUE,
    ALTER COLUMN token_hash DROP NOT NULL,
    ADD CONSTRAINT carts_owner_check CHECK ((token_hash IS NULL) <> (user_id IS NULL));

-- +goose Down
-- Account carts have no token to keep, so they go.
DELETE FROM orders.carts WHERE token_hash IS NULL;
ALTER TABLE orders.carts
    DROP CONSTRAINT carts_owner_check,
    ALTER COLUMN token_hash SET NOT NULL,
    DROP COLUMN user_id;
