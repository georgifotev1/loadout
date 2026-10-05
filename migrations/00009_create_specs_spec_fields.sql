-- +goose Up
-- Every value we have for a product's canonical field, one row per source. The sync rewrites
-- the supplier rows; admin rows are only changed by an admin, and they win.
CREATE TABLE specs.spec_fields (
    product_id uuid NOT NULL REFERENCES specs.product_specs (product_id) ON DELETE CASCADE,
    field      text NOT NULL,   -- canonical name: socket, tdp_w, ...
    source     text NOT NULL CHECK (source IN ('supplier', 'admin')),
    value      jsonb NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (product_id, field, source)
);

-- +goose Down
DROP TABLE specs.spec_fields;
