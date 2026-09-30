-- +goose Up
-- Shared extensions. Module schemas are created by each module's own first migration.
CREATE EXTENSION IF NOT EXISTS citext;  -- case-insensitive emails (auth)
CREATE EXTENSION IF NOT EXISTS pg_trgm; -- typo-tolerant product search (catalog)

-- +goose Down
DROP EXTENSION IF EXISTS pg_trgm;
DROP EXTENSION IF EXISTS citext;
