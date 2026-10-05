// Package jobs is the River plumbing: the client that inserts and works background jobs, its
// start/stop lifecycle, and River's own migrations.
//
// Modules define their job args and workers next to the code they belong to; this package only
// wires them into one client. Jobs caused by a DB change are inserted with InsertTx in that
// change's transaction, so the change and its job commit (or roll back) together.
package jobs

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
)

// Schema holds River's tables. Like every module, River gets its own Postgres schema instead of
// sharing public.
const Schema = "river"

// QueueSync runs supplier syncs, one at a time.
const QueueSync = "sync"

// hardStopTimeout is how long jobs get to return after their context is cancelled at shutdown.
const hardStopTimeout = time.Second

// Config configures New.
type Config struct {
	Logger *slog.Logger
	// Workers and Queues are nil for an insert-only client: it can enqueue jobs but not work them.
	Workers      *river.Workers
	Queues       map[string]river.QueueConfig
	PeriodicJobs []*river.PeriodicJob
	// StopTimeout is how long running jobs get to finish at shutdown before their contexts are
	// cancelled. Shutdown takes at most StopTimeout plus one second.
	StopTimeout time.Duration
}

// Client is a River client bound to pgx. Services insert through it (InsertTx); serve runs it.
type Client struct {
	*river.Client[pgx.Tx]
	logger      *slog.Logger
	stopTimeout time.Duration
	working     bool
}

// New creates a client. It doesn't start working jobs: that's Run.
func New(pool *pgxpool.Pool, cfg Config) (*Client, error) {
	rc, err := river.NewClient(riverpgxv5.New(pool), &river.Config{
		Logger:       cfg.Logger,
		Schema:       Schema,
		Workers:      cfg.Workers,
		Queues:       cfg.Queues,
		PeriodicJobs: cfg.PeriodicJobs,
	})
	if err != nil {
		return nil, fmt.Errorf("jobs: new client: %w", err)
	}
	return &Client{
		Client:      rc,
		logger:      cfg.Logger,
		stopTimeout: cfg.StopTimeout,
		working:     len(cfg.Queues) > 0,
	}, nil
}

// Working reports whether the client works jobs (Run does something) or only inserts them.
func (c *Client) Working() bool { return c.working }

// Run works jobs until ctx is cancelled, then stops in two steps:
//
//  1. Soft stop: fetch no new jobs and wait up to StopTimeout for running ones to finish.
//  2. Hard stop: cancel the running jobs' contexts. River puts those jobs back in the queue
//     (without counting the attempt), and the next worker runs them again from the start.
//     That's why workers must be idempotent: a job can run more than once.
//
// If the process dies before either step finishes (SIGKILL, crash), the job stays "running" in
// the database until River's rescuer puts it back in the queue: the same retry, only later.
func (c *Client) Run(ctx context.Context) error {
	if !c.working {
		return errors.New("jobs: Run on an insert-only client (no queues configured)")
	}
	// River stops on its own when the Start context is cancelled. We want to choose how it stops,
	// so start it on a context that is never cancelled and call Stop ourselves.
	if err := c.Start(context.WithoutCancel(ctx)); err != nil {
		return fmt.Errorf("jobs: start: %w", err)
	}
	c.logger.Info("job workers started")

	select {
	case <-ctx.Done():
	case <-c.Stopped():
		return errors.New("jobs: client stopped unexpectedly")
	}

	c.logger.Info("job workers stopping", "timeout", c.stopTimeout)
	softCtx, cancel := context.WithTimeout(context.Background(), c.stopTimeout)
	err := c.Stop(softCtx)
	cancel()
	if errors.Is(err, context.DeadlineExceeded) {
		c.logger.Warn("jobs still running after stop timeout; cancelling them")
		hardCtx, cancel := context.WithTimeout(context.Background(), hardStopTimeout)
		defer cancel()
		err = c.StopAndCancel(hardCtx)
	}
	if err != nil {
		return fmt.Errorf("jobs: stop: %w", err)
	}
	c.logger.Info("job workers stopped")
	return nil
}
