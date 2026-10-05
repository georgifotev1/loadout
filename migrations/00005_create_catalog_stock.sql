-- +goose Up
-- How many we can get from the supplier, one row per product. Kept apart from products for the
-- same reason as prices. If a supplier turns out to need per-warehouse stock, add a warehouse
-- column to the key.
CREATE TABLE catalog.stock (
    product_id uuid PRIMARY KEY REFERENCES catalog.products (id) ON DELETE CASCADE,
    qty        int NOT NULL CHECK (qty >= 0),
    updated_at timestamptz NOT NULL DEFAULT now()  -- when qty last changed
);

-- +goose Down
DROP TABLE catalog.stock;
