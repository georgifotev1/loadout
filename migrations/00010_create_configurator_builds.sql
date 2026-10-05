-- +goose Up
-- The configurator module owns this schema: saved PC builds, shareable at /build/{slug}.
CREATE SCHEMA configurator;

-- A build as it was saved. parts is a JSON array of {slot, product_id, slug, retail_cents}, in
-- slot order, one entry per part bought (a kit bought twice is there twice). retail_cents is the
-- price when it was saved, to tell the shopper what changed since. product_id is a catalog id,
-- without a foreign key (see specs.product_specs).
-- TODO(phase-7): owner_id once customers have accounts.
CREATE TABLE configurator.builds (
    slug       text PRIMARY KEY,
    parts      jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE configurator.builds;
DROP SCHEMA configurator;
