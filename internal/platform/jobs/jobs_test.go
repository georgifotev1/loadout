package jobs_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/gfotev/loadout/internal/platform/db"
	"github.com/gfotev/loadout/internal/platform/db/dbtest"
	"github.com/gfotev/loadout/internal/platform/jobs"
)

type blockArgs struct{}

func (blockArgs) Kind() string { return "test_block" }

// blockWorker signals when it starts, then waits for release or for its context to be cancelled.
type blockWorker struct {
	river.WorkerDefaults[blockArgs]
	started chan struct{}
	release chan struct{}
}

func (w *blockWorker) Work(ctx context.Context, job *river.Job[blockArgs]) error {
	w.started <- struct{}{}
	select {
	case <-w.release:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func newWorkingClient(t *testing.T, pool *pgxpool.Pool, w *blockWorker, stopTimeout time.Duration) *jobs.Client {
	t.Helper()
	workers := river.NewWorkers()
	river.AddWorker(workers, w)
	c, err := jobs.New(pool, jobs.Config{
		Logger:      slog.New(slog.DiscardHandler),
		Workers:     workers,
		Queues:      map[string]river.QueueConfig{river.QueueDefault: {MaxWorkers: 1}},
		StopTimeout: stopTimeout,
	})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// runInBackground starts c.Run and returns a func that cancels it and returns Run's error.
func runInBackground(c *jobs.Client) (stop func() error) {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- c.Run(ctx) }()
	return func() error {
		cancel()
		return <-done
	}
}

func waitStarted(t *testing.T, w *blockWorker) {
	t.Helper()
	select {
	case <-w.started:
	case <-time.After(10 * time.Second):
		t.Fatal("job never started")
	}
}

func TestMigrateUpIsIdempotent(t *testing.T) {
	pool := dbtest.New(t) // already ran MigrateUp once
	ctx := context.Background()

	var out bytes.Buffer
	if err := jobs.MigrateUp(ctx, pool, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "no pending river migrations") {
		t.Errorf("second MigrateUp output = %q", out.String())
	}

	out.Reset()
	if err := jobs.Status(ctx, pool, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out.String(), "river schema: version ") {
		t.Errorf("Status output = %q", out.String())
	}

	var inRiverSchema bool
	if err := pool.QueryRow(ctx, `SELECT to_regclass('river.river_job') IS NOT NULL`).Scan(&inRiverSchema); err != nil {
		t.Fatal(err)
	}
	if !inRiverSchema {
		t.Error("river_job is not in the river schema")
	}
}

// A job inserted with InsertTx exists only if the transaction commits: no dual write.
func TestInsertTxRollsBackWithTheTransaction(t *testing.T) {
	pool := dbtest.New(t)
	ctx := context.Background()
	c, err := jobs.New(pool, jobs.Config{Logger: slog.New(slog.DiscardHandler)})
	if err != nil {
		t.Fatal(err)
	}
	if c.Working() {
		t.Fatal("client without queues reports Working")
	}

	errBoom := errors.New("boom")
	err = db.InTx(ctx, pool, func(tx pgx.Tx) error {
		if _, err := c.InsertTx(ctx, tx, blockArgs{}, nil); err != nil {
			return err
		}
		return errBoom // a later step of the same use case fails
	})
	if !errors.Is(err, errBoom) {
		t.Fatalf("InTx err = %v, want boom", err)
	}
	if n := countJobs(t, pool); n != 0 {
		t.Fatalf("%d jobs after rollback, want 0", n)
	}

	err = db.InTx(ctx, pool, func(tx pgx.Tx) error {
		_, err := c.InsertTx(ctx, tx, blockArgs{}, nil)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if n := countJobs(t, pool); n != 1 {
		t.Fatalf("%d jobs after commit, want 1", n)
	}
}

func TestRunOnInsertOnlyClientFails(t *testing.T) {
	pool := dbtest.New(t)
	c, err := jobs.New(pool, jobs.Config{Logger: slog.New(slog.DiscardHandler)})
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Run(context.Background()); err == nil {
		t.Fatal("Run on an insert-only client returned nil")
	}
}

func TestRunStops(t *testing.T) {
	pool := dbtest.New(t)
	ctx := context.Background()

	// A job that finishes within StopTimeout completes normally.
	t.Run("soft stop lets the running job finish", func(t *testing.T) {
		w := &blockWorker{started: make(chan struct{}, 1), release: make(chan struct{})}
		c := newWorkingClient(t, pool, w, 10*time.Second)
		stop := runInBackground(c)

		res, err := c.Insert(ctx, blockArgs{}, nil)
		if err != nil {
			t.Fatal(err)
		}
		waitStarted(t, w)

		go func() {
			time.Sleep(200 * time.Millisecond) // shutdown has begun and is waiting on the job
			close(w.release)
		}()
		if err := stop(); err != nil {
			t.Fatalf("Run: %v", err)
		}
		if got := jobState(t, c, res.Job.ID); got != rivertype.JobStateCompleted {
			t.Errorf("job state = %s, want completed", got)
		}
	})

	// A job still running after StopTimeout is cancelled and put back in the queue, so it runs
	// again from the start on the next worker: at-least-once.
	t.Run("hard stop cancels the job and puts it back", func(t *testing.T) {
		w := &blockWorker{started: make(chan struct{}, 1), release: make(chan struct{})}
		c := newWorkingClient(t, pool, w, 100*time.Millisecond)
		stop := runInBackground(c)

		res, err := c.Insert(ctx, blockArgs{}, nil)
		if err != nil {
			t.Fatal(err)
		}
		waitStarted(t, w)

		if err := stop(); err != nil {
			t.Fatalf("Run: %v", err)
		}
		job, err := c.JobGet(ctx, res.Job.ID)
		if err != nil {
			t.Fatal(err)
		}
		// River doesn't count a shutdown cancellation as a failed attempt.
		if job.State != rivertype.JobStateAvailable || job.Attempt != 0 {
			t.Errorf("job state = %s attempt %d, want available attempt 0", job.State, job.Attempt)
		}
	})
}

func countJobs(t *testing.T, pool *pgxpool.Pool) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM river.river_job`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func jobState(t *testing.T, c *jobs.Client, id int64) rivertype.JobState {
	t.Helper()
	job, err := c.JobGet(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	return job.State
}
