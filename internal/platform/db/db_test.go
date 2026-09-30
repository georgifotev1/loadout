package db_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/gfotev/loadout/internal/platform/db"
)

// newPool starts a throwaway Postgres container and returns a pool with a scratch table.
func newPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	if testing.Short() {
		t.Skip("needs Docker (testcontainers)")
	}
	ctx := context.Background()

	ctr, err := postgres.Run(ctx, "postgres:17.11-alpine",
		postgres.WithDatabase("loadout_test"),
		postgres.WithUsername("loadout"),
		postgres.WithPassword("loadout"),
		postgres.BasicWaitStrategies(),
	)
	testcontainers.CleanupContainer(t, ctr)
	if err != nil {
		t.Fatalf("start postgres: %v", err)
	}
	url, err := ctr.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	pool, err := db.Open(ctx, url, 4)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)

	if _, err := pool.Exec(ctx, `CREATE TABLE items (name text PRIMARY KEY)`); err != nil {
		t.Fatal(err)
	}
	return pool
}

func count(t *testing.T, pool *pgxpool.Pool) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM items`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func insert(ctx context.Context, q db.DBTX, name string) error {
	_, err := q.Exec(ctx, `INSERT INTO items (name) VALUES ($1)`, name)
	return err
}

func TestInTx(t *testing.T) {
	pool := newPool(t)
	ctx := context.Background()

	t.Run("commits when fn returns nil", func(t *testing.T) {
		t.Cleanup(func() { _, _ = pool.Exec(ctx, `TRUNCATE items`) })

		err := db.InTx(ctx, pool, func(tx pgx.Tx) error {
			if err := insert(ctx, tx, "a"); err != nil {
				return err
			}
			return insert(ctx, tx, "b")
		})
		if err != nil {
			t.Fatal(err)
		}
		if got := count(t, pool); got != 2 {
			t.Fatalf("rows = %d, want 2", got)
		}
	})

	t.Run("rolls back every write when fn fails on the last step", func(t *testing.T) {
		t.Cleanup(func() { _, _ = pool.Exec(ctx, `TRUNCATE items`) })
		errBoom := errors.New("boom")

		err := db.InTx(ctx, pool, func(tx pgx.Tx) error {
			if err := insert(ctx, tx, "a"); err != nil {
				return err
			}
			return errBoom
		})
		if !errors.Is(err, errBoom) {
			t.Fatalf("err = %v, want %v", err, errBoom)
		}
		if got := count(t, pool); got != 0 {
			t.Fatalf("rows = %d, want 0 (partial write leaked)", got)
		}
	})

	t.Run("rolls back and re-panics on panic", func(t *testing.T) {
		t.Cleanup(func() { _, _ = pool.Exec(ctx, `TRUNCATE items`) })

		func() {
			defer func() {
				if p := recover(); p != "kaboom" {
					t.Fatalf("recovered %v, want the original panic", p)
				}
			}()
			_ = db.InTx(ctx, pool, func(tx pgx.Tx) error {
				if err := insert(ctx, tx, "a"); err != nil {
					return err
				}
				panic("kaboom")
			})
		}()
		if got := count(t, pool); got != 0 {
			t.Fatalf("rows = %d, want 0", got)
		}
		// The connection must have gone back to the pool, not leaked with an open tx.
		if s := pool.Stat(); s.AcquiredConns() != 0 {
			t.Fatalf("acquired conns = %d, want 0", s.AcquiredConns())
		}
	})

	t.Run("rolls back when the context is cancelled", func(t *testing.T) {
		t.Cleanup(func() { _, _ = pool.Exec(ctx, `TRUNCATE items`) })
		cctx, cancel := context.WithCancel(ctx)

		err := db.InTx(cctx, pool, func(tx pgx.Tx) error {
			if err := insert(cctx, tx, "a"); err != nil {
				return err
			}
			cancel()
			return nil
		})
		if err == nil {
			t.Fatal("expected an error from a cancelled context")
		}
		if got := count(t, pool); got != 0 {
			t.Fatalf("rows = %d, want 0", got)
		}
	})
}
