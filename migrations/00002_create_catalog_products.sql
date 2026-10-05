-- +goose Up
-- The catalog module owns this schema. No other module reads or writes it.
--
-- Columns hold only what every supplier gives us (internal/supplier.Product). Add a column when
-- a real supplier provides the data and a feature needs it.
CREATE SCHEMA catalog;

CREATE TABLE catalog.products (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    provider        text NOT NULL,         -- the supplier the product comes from: 'vali'
    provider_sku    text NOT NULL,         -- the supplier's own SKU, unique only within that supplier
    name            text NOT NULL,
    brand           text NOT NULL,
    slug            text NOT NULL UNIQUE,  -- public URL, never changed by sync once set
    category        text NOT NULL,         -- canonical: cpu, motherboard, ..., peripheral.*
    attributes      jsonb NOT NULL DEFAULT '{}',  -- the supplier's spec rows, for display
    images          jsonb NOT NULL DEFAULT '[]',

    -- Change detection: two hashes so a price or stock change doesn't look like a content change
    -- (which would re-save specs) and vice versa.
    content_hash    text NOT NULL,         -- name, brand, category, attributes, images, specs
    commercial_hash text NOT NULL,         -- cost, stock

    missing_runs    int NOT NULL DEFAULT 0 CHECK (missing_runs >= 0),
    active          boolean NOT NULL DEFAULT true,

    -- 'simple' because Postgres has no built-in Bulgarian stemmer. The two-argument
    -- to_tsvector is IMMUTABLE, which a generated column requires.
    search          tsvector GENERATED ALWAYS AS (
                        setweight(to_tsvector('simple', name), 'A') ||
                        setweight(to_tsvector('simple', brand || ' ' || provider_sku), 'B')
                    ) STORED,

    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),

    -- The natural key: sync upserts ON CONFLICT on it. Two suppliers can use the same SKU.
    CONSTRAINT products_provider_sku_key UNIQUE (provider, provider_sku)
);

CREATE INDEX products_search_idx          ON catalog.products USING gin (search);
CREATE INDEX products_name_trgm_idx       ON catalog.products USING gin (name gin_trgm_ops);
CREATE INDEX products_category_active_idx ON catalog.products (category, active);

-- +goose Down
DROP TABLE catalog.products;
DROP SCHEMA catalog;
