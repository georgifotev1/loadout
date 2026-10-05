package configurator

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/gfotev/loadout/internal/platform/db"
)

type store struct{}

// insertBuild stores a build under slug. It reports false, and stores nothing, if the slug is
// taken.
func (store) insertBuild(ctx context.Context, q db.DBTX, slug string, parts []SavedPart) (SavedBuild, bool, error) {
	query := `
		INSERT INTO configurator.builds (slug, parts)
		VALUES ($1, $2)
		ON CONFLICT (slug) DO NOTHING
		RETURNING created_at`

	b := SavedBuild{Slug: slug, Parts: parts}
	err := q.QueryRow(ctx, query, slug, parts).Scan(&b.CreatedAt)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return SavedBuild{}, false, nil
	case err != nil:
		return SavedBuild{}, false, fmt.Errorf("configurator: insert build: %w", err)
	}
	return b, true, nil
}

func (store) build(ctx context.Context, q db.DBTX, slug string) (SavedBuild, error) {
	query := `SELECT slug, parts, created_at FROM configurator.builds WHERE slug = $1`

	var b SavedBuild
	err := q.QueryRow(ctx, query, slug).Scan(
		&b.Slug,
		&b.Parts,
		&b.CreatedAt,
	)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return SavedBuild{}, ErrNotFound
	case err != nil:
		return SavedBuild{}, fmt.Errorf("configurator: build %q: %w", slug, err)
	}
	return b, nil
}
