package migrations_test

import (
	"context"
	"io"
	"io/fs"
	"testing"

	"github.com/gfotev/loadout/internal/platform/db"
	"github.com/gfotev/loadout/internal/platform/db/dbtest"
	"github.com/gfotev/loadout/migrations"
)

// Every migration must be reversible: roll all of them back, then apply them again.
func TestMigrationsRoundTrip(t *testing.T) {
	pool := dbtest.New(t) // fully migrated
	ctx := context.Background()

	files, err := fs.Glob(migrations.FS, "*.sql")
	if err != nil {
		t.Fatal(err)
	}
	m, err := db.NewMigrator(pool, migrations.FS)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()

	for range files {
		if err := m.Down(ctx, io.Discard); err != nil {
			t.Fatalf("down: %v", err)
		}
	}
	var schemas int
	err = pool.QueryRow(ctx, `SELECT count(*) FROM pg_namespace WHERE nspname IN ('catalog', 'specs', 'configurator', 'auth')`).Scan(&schemas)
	if err != nil {
		t.Fatal(err)
	}
	if schemas != 0 {
		t.Errorf("%d module schemas left after rolling everything back", schemas)
	}
	if err := m.Up(ctx, io.Discard); err != nil {
		t.Fatalf("up again: %v", err)
	}
}
