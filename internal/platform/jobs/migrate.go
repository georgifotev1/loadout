package jobs

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"
)

// River ships its schema as its own numbered migrations, tracked in river.river_migration rather
// than in goose's table. Upgrading River can add one; `loadout migrate up` applies it.

// MigrateUp creates the river schema if needed and applies River's pending migrations.
func MigrateUp(ctx context.Context, pool *pgxpool.Pool, w io.Writer) error {
	if _, err := pool.Exec(ctx, `CREATE SCHEMA IF NOT EXISTS `+pgx.Identifier{Schema}.Sanitize()); err != nil {
		return fmt.Errorf("jobs: create schema: %w", err)
	}
	m, err := newMigrator(pool)
	if err != nil {
		return err
	}
	res, err := m.Migrate(ctx, rivermigrate.DirectionUp, nil)
	if err != nil {
		return fmt.Errorf("jobs: migrate up: %w", err)
	}
	for _, v := range res.Versions {
		fmt.Fprintf(w, "up    river %03d_%s (%s)\n", v.Version, v.Name, v.Duration.Round(time.Millisecond))
	}
	if len(res.Versions) == 0 {
		fmt.Fprintln(w, "no pending river migrations")
	}
	return nil
}

// Status prints River's applied version and the latest one this binary knows.
func Status(ctx context.Context, pool *pgxpool.Pool, w io.Writer) error {
	var exists bool
	if err := pool.QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL`, Schema+".river_migration").Scan(&exists); err != nil {
		return fmt.Errorf("jobs: migrate status: %w", err)
	}
	m, err := newMigrator(pool)
	if err != nil {
		return err
	}
	all := m.AllVersions()
	latest := all[len(all)-1].Version

	applied := 0
	if exists {
		existing, err := m.ExistingVersions(ctx)
		if err != nil {
			return fmt.Errorf("jobs: migrate status: %w", err)
		}
		if len(existing) > 0 {
			applied = existing[len(existing)-1].Version
		}
	}
	fmt.Fprintf(w, "river schema: version %d of %d\n", applied, latest)
	return nil
}

func newMigrator(pool *pgxpool.Pool) (*rivermigrate.Migrator[pgx.Tx], error) {
	m, err := rivermigrate.New(riverpgxv5.New(pool), &rivermigrate.Config{
		Schema: Schema,
		// River logs every applied migration itself; we print our own lines instead.
		Logger: slog.New(slog.DiscardHandler),
	})
	if err != nil {
		return nil, fmt.Errorf("jobs: migrator: %w", err)
	}
	return m, nil
}
