-- +goose Up
-- Markup rules turn a dealer price into a retail price. The most specific matching rule wins:
-- sku > brand_category > category > default (resolution lives in Go, internal/catalog/pricing.go).
CREATE TABLE catalog.markup_rules (
    id               bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    scope            text NOT NULL CHECK (scope IN ('default', 'category', 'brand_category', 'sku')),
    category         text,
    brand            text,
    sku              text,
    markup_bp        int NOT NULL CHECK (markup_bp >= 0),          -- 2500 = +25%
    min_margin_cents bigint NOT NULL DEFAULT 0 CHECK (min_margin_cents >= 0),
    rounding         text NOT NULL DEFAULT 'none' CHECK (rounding IN ('none', '99', '90')),
    priority         int NOT NULL DEFAULT 0,                        -- tie-breaker within a scope
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now(),

    -- Each scope fills exactly the fields it matches on, so a rule can't be half-specified.
    CONSTRAINT markup_rules_scope_fields CHECK (
        CASE scope
            WHEN 'default'        THEN category IS NULL     AND brand IS NULL     AND sku IS NULL
            WHEN 'category'       THEN category IS NOT NULL AND brand IS NULL     AND sku IS NULL
            WHEN 'brand_category' THEN category IS NOT NULL AND brand IS NOT NULL AND sku IS NULL
            WHEN 'sku'            THEN category IS NULL     AND brand IS NULL     AND sku IS NOT NULL
        END
    ),
    -- One rule per target. NULLS NOT DISTINCT (Postgres 15+) makes two 'default' rules
    -- (all NULLs) collide too, which a plain UNIQUE would allow.
    CONSTRAINT markup_rules_target_key UNIQUE NULLS NOT DISTINCT (scope, category, brand, sku)
);

-- +goose Down
DROP TABLE catalog.markup_rules;
