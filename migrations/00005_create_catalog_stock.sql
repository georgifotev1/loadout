-- +goose Up
-- Stock per Vali warehouse. A product's rows are the warehouses in its latest payload: a
-- warehouse that disappears is deleted, so sum(qty) is always the current total.
CREATE TABLE catalog.stock (
    product_id  uuid NOT NULL REFERENCES catalog.products (id) ON DELETE CASCADE,
    warehouse   text NOT NULL,
    qty         int NOT NULL CHECK (qty >= 0),
    observed_at timestamptz NOT NULL DEFAULT now(),  -- when this qty was first seen
    PRIMARY KEY (product_id, warehouse)
);

-- +goose Down
DROP TABLE catalog.stock;
