package db

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrLockHeld is returned by TryAdvisoryLock when another session holds the lock.
var ErrLockHeld = errors.New("db: advisory lock is held by another session")

// TryAdvisoryLock takes the session-level advisory lock key without waiting, on a connection
// it keeps out of the pool until release is called. It's for locks held through a long run
// (the supplier sync): the run's own work uses other connections and short transactions, so no
// transaction stays open for minutes.
//
// The lock belongs to the connection, not to a transaction, so release must always be called
// (defer it). If the process dies, Postgres drops the connection and the lock with it.
func TryAdvisoryLock(ctx context.Context, pool *pgxpool.Pool, key int64) (release func(), err error) {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("db: acquire connection for advisory lock: %w", err)
	}
	var locked bool
	if err := conn.QueryRow(ctx, `SELECT pg_try_advisory_lock($1)`, key).Scan(&locked); err != nil {
		conn.Release()
		return nil, fmt.Errorf("db: advisory lock %d: %w", key, err)
	}
	if !locked {
		conn.Release()
		return nil, ErrLockHeld
	}

	var once sync.Once
	release = func() {
		once.Do(func() {
			// Unlock even if ctx was cancelled: that's usually why we're releasing.
			ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
			defer cancel()
			var unlocked bool
			err := conn.QueryRow(ctx, `SELECT pg_advisory_unlock($1)`, key).Scan(&unlocked)
			if err != nil || !unlocked {
				// We can't tell whether the lock is gone, so don't hand the connection back to
				// the pool still holding it: closing the connection releases every session lock.
				_ = conn.Hijack().Close(ctx)
				return
			}
			conn.Release()
		})
	}
	return release, nil
}
