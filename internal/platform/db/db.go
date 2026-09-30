// Package db holds the Postgres plumbing: the connection pool, the DBTX interface that stores
// accept, the InTx transaction helper that services use, and the goose migration runner.
package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DBTX is satisfied by *pgxpool.Pool and pgx.Tx, so a store method works in or out of a
// transaction. Stores take a DBTX; services decide which one to hand them.
type DBTX interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

var (
	_ DBTX = (*pgxpool.Pool)(nil)
	_ DBTX = (pgx.Tx)(nil)
)

// Open creates a pool and checks that the database is reachable.
func Open(ctx context.Context, url string, maxConns int32) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("db: parse url: %w", err)
	}
	if maxConns > 0 {
		cfg.MaxConns = maxConns
	}
	cfg.MaxConnIdleTime = 5 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("db: create pool: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("db: ping: %w", err)
	}
	return pool, nil
}

// InTx runs fn in a transaction. It commits if fn returns nil and rolls back on an error,
// a panic (then re-panics) or context cancellation.
//
// fn must not call external systems (Vali, Speedy, Stripe, email): see AGENT_PLAN.md §1.2.
func InTx(ctx context.Context, pool *pgxpool.Pool, fn func(tx pgx.Tx) error) (err error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("db: begin: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			// Use a fresh context: ctx may be the reason we're here.
			_ = tx.Rollback(context.WithoutCancel(ctx))
			panic(p)
		}
		if err != nil {
			if rbErr := tx.Rollback(context.WithoutCancel(ctx)); rbErr != nil && !errors.Is(rbErr, pgx.ErrTxClosed) {
				err = errors.Join(err, fmt.Errorf("db: rollback: %w", rbErr))
			}
		}
	}()

	if err = fn(tx); err != nil {
		return err
	}
	// If ctx was cancelled, Commit fails and pgx rolls the transaction back.
	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("db: commit: %w", err)
	}
	return nil
}
