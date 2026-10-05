-- +goose Up
-- A starting default markup so every synced product gets a retail price: +15%, at least €5
-- margin, rounded up to a .90 ending. Placeholder numbers: change them with SQL until the admin
-- markup page exists (Phase 6). DO NOTHING keeps a default rule that is already there.
INSERT INTO catalog.markup_rules (scope, markup_bp, min_margin_cents, rounding)
VALUES ('default', 1500, 500, '90')
ON CONFLICT ON CONSTRAINT markup_rules_target_key DO NOTHING;

-- +goose Down
-- Also removes a default rule edited after the seed; prices that used it keep their amounts.
DELETE FROM catalog.markup_rules WHERE scope = 'default';
