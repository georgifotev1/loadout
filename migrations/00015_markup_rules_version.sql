-- +goose Up
-- Admins edit markup rules from the back office. version goes up on every edit, and an edit
-- names the version it started from, so a stale browser tab gets a conflict instead of
-- silently overwriting a newer change.
ALTER TABLE catalog.markup_rules ADD COLUMN version int NOT NULL DEFAULT 1;

-- Brands match case-insensitively when pricing ("GIGABYTE" = "Gigabyte"), so they must be
-- unique that way too: two rules that differ only in case would both match the same products.
-- A unique index, not a constraint, because a constraint can't hold an expression.
ALTER TABLE catalog.markup_rules DROP CONSTRAINT markup_rules_target_key;
CREATE UNIQUE INDEX markup_rules_target_key
    ON catalog.markup_rules (scope, category, lower(brand), sku) NULLS NOT DISTINCT;

-- +goose Down
DROP INDEX catalog.markup_rules_target_key;
ALTER TABLE catalog.markup_rules
    ADD CONSTRAINT markup_rules_target_key UNIQUE NULLS NOT DISTINCT (scope, category, brand, sku);
ALTER TABLE catalog.markup_rules DROP COLUMN version;
