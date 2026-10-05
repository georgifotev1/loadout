-- +goose Up
-- The specs module owns this schema. It keeps the canonical, typed specs of builder parts and
-- whether they are complete enough for the PC builder. No other module reads or writes it.
CREATE SCHEMA specs;

-- One row per builder product: its effective specs (admin value, else the supplier's).
--
-- product_id is a catalog product id, deliberately without a foreign key: a cross-schema FK would
-- tie the two modules' tables together. Catalog never hard-deletes products (it deactivates them).
-- category and product_name are copies, so the review queue never reads the catalog schema.
CREATE TABLE specs.product_specs (
    product_id       uuid PRIMARY KEY,
    category         text NOT NULL,
    product_name     text NOT NULL,
    specs            jsonb NOT NULL DEFAULT '{}',   -- field -> effective value
    missing          text[] NOT NULL DEFAULT '{}',  -- required fields without a value
    spec_status      text NOT NULL CHECK (spec_status IN ('incomplete', 'approved')),
    builder_eligible boolean GENERATED ALWAYS AS (spec_status = 'approved') STORED,
    updated_at       timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX product_specs_category_status_idx ON specs.product_specs (category, spec_status);
-- Facet filters in the builder and search: specs @> '{"socket": "AM5"}'.
CREATE INDEX product_specs_specs_idx           ON specs.product_specs USING gin (specs jsonb_path_ops);

-- +goose Down
DROP TABLE specs.product_specs;
DROP SCHEMA specs;
