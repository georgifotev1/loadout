package db

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"io/fs"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

// Migrator runs goose migrations from an fs.FS (the app passes the embedded /migrations).
// River's own migrations are separate: see jobs.MigrateUp.
type Migrator struct {
	sqlDB    *sql.DB
	provider *goose.Provider
}

// NewMigrator wraps the pool in a database/sql handle, which is what goose works with.
func NewMigrator(pool *pgxpool.Pool, fsys fs.FS) (*Migrator, error) {
	sqlDB := stdlib.OpenDBFromPool(pool)
	p, err := goose.NewProvider(goose.DialectPostgres, sqlDB, fsys)
	if err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("db: goose provider: %w", err)
	}
	return &Migrator{sqlDB: sqlDB, provider: p}, nil
}

func (m *Migrator) Close() error { return m.sqlDB.Close() }

// Up applies all pending migrations.
func (m *Migrator) Up(ctx context.Context, w io.Writer) error {
	results, err := m.provider.Up(ctx)
	for _, r := range results {
		fmt.Fprintf(w, "up    %s (%s)\n", r.Source.Path, r.Duration.Round(time.Millisecond))
	}
	if err != nil {
		return fmt.Errorf("db: migrate up: %w", err)
	}
	if len(results) == 0 {
		fmt.Fprintln(w, "no pending migrations")
	}
	return nil
}

// Down rolls back the most recent migration.
func (m *Migrator) Down(ctx context.Context, w io.Writer) error {
	r, err := m.provider.Down(ctx)
	if r != nil {
		fmt.Fprintf(w, "down  %s (%s)\n", r.Source.Path, r.Duration.Round(time.Millisecond))
	}
	if err != nil {
		return fmt.Errorf("db: migrate down: %w", err)
	}
	return nil
}

// Status prints every migration and whether it has been applied.
func (m *Migrator) Status(ctx context.Context, w io.Writer) error {
	statuses, err := m.provider.Status(ctx)
	if err != nil {
		return fmt.Errorf("db: migrate status: %w", err)
	}
	for _, s := range statuses {
		applied := "pending"
		if !s.AppliedAt.IsZero() {
			applied = s.AppliedAt.Format("2006-01-02 15:04:05")
		}
		fmt.Fprintf(w, "%-20s %s\n", applied, s.Source.Path)
	}
	return nil
}
