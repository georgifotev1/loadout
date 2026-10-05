package db_test

import (
	"context"
	"errors"
	"testing"

	"github.com/gfotev/loadout/internal/platform/db"
)

func TestTryAdvisoryLock(t *testing.T) {
	pool := newPool(t)
	ctx := context.Background()
	const key = 42

	release, err := db.TryAdvisoryLock(ctx, pool, key)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := db.TryAdvisoryLock(ctx, pool, key); !errors.Is(err, db.ErrLockHeld) {
		t.Fatalf("second lock: err = %v, want ErrLockHeld", err)
	}
	// Other keys are independent.
	releaseOther, err := db.TryAdvisoryLock(ctx, pool, key+1)
	if err != nil {
		t.Fatalf("other key: %v", err)
	}
	releaseOther()

	release()
	release() // safe to call twice

	again, err := db.TryAdvisoryLock(ctx, pool, key)
	if err != nil {
		t.Fatalf("after release: %v", err)
	}
	again()

	// Released connections went back to the pool without holding the lock.
	var held int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM pg_locks WHERE locktype = 'advisory'`).Scan(&held); err != nil {
		t.Fatal(err)
	}
	if held != 0 {
		t.Errorf("%d advisory locks still held", held)
	}
}

func TestTryAdvisoryLockReleasesAfterCancel(t *testing.T) {
	pool := newPool(t)
	ctx, cancel := context.WithCancel(context.Background())

	release, err := db.TryAdvisoryLock(ctx, pool, 7)
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	release()

	again, err := db.TryAdvisoryLock(context.Background(), pool, 7)
	if err != nil {
		t.Fatalf("lock after cancelled release: %v", err)
	}
	again()
}
