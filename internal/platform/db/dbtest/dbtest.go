// Package dbtest gives integration tests a real, fully migrated Postgres (testcontainers).
//
// Each call starts its own container, so tests in different packages never share state. Within
// a package, start one per test function and clean up between subtests (TRUNCATE) rather than
// one container per subtest: a container takes a couple of seconds to start.
package dbtest

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/gfotev/loadout/internal/platform/db"
	"github.com/gfotev/loadout/internal/platform/jobs"
	"github.com/gfotev/loadout/migrations"
)

// Image matches deploy/compose so tests run against the same Postgres as local dev.
const Image = "postgres:17.11-alpine"

// New starts Postgres, applies every migration (ours and River's) and returns a pool. Skipped
// under -short.
func New(t testing.TB) *pgxpool.Pool {
	t.Helper()
	if testing.Short() {
		t.Skip("needs Docker (testcontainers)")
	}
	ctx := context.Background()

	ctr, err := postgres.Run(ctx, Image,
		postgres.WithDatabase("loadout_test"),
		postgres.WithUsername("loadout"),
		postgres.WithPassword("loadout"),
		postgres.BasicWaitStrategies(),
	)
	testcontainers.CleanupContainer(t, ctr)
	if err != nil {
		t.Fatalf("dbtest: start postgres: %v", err)
	}
	url, err := ctr.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("dbtest: connection string: %v", err)
	}
	// Same timeout as the app's default, so a test exercising a slow query sees what prod would.
	pool, err := db.Open(ctx, db.Options{URL: url, MaxConns: 8, StatementTimeout: 5 * time.Second})
	if err != nil {
		t.Fatalf("dbtest: %v", err)
	}
	t.Cleanup(pool.Close)

	m, err := db.NewMigrator(pool, migrations.FS)
	if err != nil {
		t.Fatalf("dbtest: %v", err)
	}
	defer m.Close()
	if err := m.Up(ctx, io.Discard); err != nil {
		t.Fatalf("dbtest: %v", err)
	}
	if err := jobs.MigrateUp(ctx, pool, io.Discard); err != nil {
		t.Fatalf("dbtest: %v", err)
	}
	return pool
}
