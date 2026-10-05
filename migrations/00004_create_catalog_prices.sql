-- +goose Up
-- One current price per product. Kept apart from products because it changes on almost every
-- sync while the content rarely does. Order lines snapshot these values (Phase 7).
CREATE TABLE catalog.prices (
    product_id   uuid PRIMARY KEY REFERENCES catalog.products (id) ON DELETE CASCADE,
    dealer_cents bigint NOT NULL CHECK (dealer_cents >= 0),  -- what we pay the supplier, VAT included
    retail_cents bigint NOT NULL CHECK (retail_cents >= 0),  -- what the customer pays, VAT included
    vat_bp       int NOT NULL CHECK (vat_bp BETWEEN 0 AND 10000),
    -- The rule that produced retail_cents. SET NULL so deleting a rule doesn't delete prices;
    -- the next price recompute picks a new one.
    rule_id      bigint REFERENCES catalog.markup_rules (id) ON DELETE SET NULL,
    computed_at  timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX prices_rule_id_idx ON catalog.prices (rule_id);

-- +goose Down
DROP TABLE catalog.prices;
