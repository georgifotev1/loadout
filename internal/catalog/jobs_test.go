package catalog

import (
	"context"
	"iter"
	"log/slog"
	"testing"
	"time"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/gfotev/loadout/internal/platform/jobs"
	"github.com/gfotev/loadout/internal/supplier"
)

// stallAfter passes through n products, signals reached, then blocks until ctx is cancelled:
// a sync still running when the process is told to stop.
type stallAfter struct {
	*supplier.Fake
	n       int
	reached chan struct{}
}

func (s *stallAfter) Products(ctx context.Context) iter.Seq2[supplier.Product, error] {
	return func(yield func(supplier.Product, error) bool) {
		i := 0
		for p, err := range s.Fake.Products(ctx) {
			if i == s.n {
				close(s.reached)
				<-ctx.Done()
				yield(supplier.Product{}, ctx.Err())
				return
			}
			i++
			if !yield(p, err) || err != nil {
				return
			}
		}
	}
}

func newSyncJobs(t *testing.T, env *syncEnv) *jobs.Client {
	t.Helper()
	workers := river.NewWorkers()
	river.AddWorker(workers, NewSyncWorker(env.svc))
	c, err := jobs.New(env.pool, jobs.Config{
		Logger:      slog.New(slog.DiscardHandler),
		Workers:     workers,
		Queues:      map[string]river.QueueConfig{jobs.QueueSync: {MaxWorkers: 1}},
		StopTimeout: 100 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestSyncJobIsUnique(t *testing.T) {
	env := newSyncEnv(t)
	c := newSyncJobs(t, env)
	ctx := context.Background()

	first, err := c.Insert(ctx, SyncArgs{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	second, err := c.Insert(ctx, SyncArgs{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !second.UniqueSkippedAsDuplicate || second.Job.ID != first.Job.ID {
		t.Errorf("second insert made job %d, want a duplicate of %d", second.Job.ID, first.Job.ID)
	}
}

// The process stops mid-sync; after a restart the same job finishes the sync, with no product
// lost or written twice.
func TestSyncJobSurvivesRestart(t *testing.T) {
	env := newSyncEnv(t)
	ctx := context.Background()
	stall := &stallAfter{Fake: env.src, n: 3, reached: make(chan struct{})}
	env.svc.supplier = stall

	// First process: picks up the job and is stopped while the sync is stuck after 3 products.
	first := newSyncJobs(t, env)
	res, err := first.Insert(ctx, SyncArgs{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	runCtx, stop := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() { done <- first.Run(runCtx) }()
	select {
	case <-stall.reached:
	case <-time.After(10 * time.Second):
		t.Fatal("sync never started")
	}
	stop()
	if err := <-done; err != nil {
		t.Fatalf("first Run: %v", err)
	}
	job, err := first.JobGet(ctx, res.Job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if job.State != rivertype.JobStateAvailable {
		t.Fatalf("job state after stop = %s, want available", job.State)
	}

	// Second process: works the same job to completion.
	env.svc.supplier = env.src
	second := newSyncJobs(t, env)
	runCtx, stop = context.WithCancel(ctx)
	defer stop()
	go func() { done <- second.Run(runCtx) }()
	deadline := time.Now().Add(10 * time.Second)
	for {
		job, err = second.JobGet(ctx, res.Job.ID)
		if err != nil {
			t.Fatal(err)
		}
		if job.State == rivertype.JobStateCompleted {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("job state = %s, want completed", job.State)
		}
		time.Sleep(50 * time.Millisecond)
	}
	stop()
	if err := <-done; err != nil {
		t.Fatalf("second Run: %v", err)
	}

	if n := env.count(t, `SELECT count(*) FROM catalog.products`); n != len(env.src.Items) {
		t.Errorf("%d products, want %d", n, len(env.src.Items))
	}
	// The interrupted run is recorded as failed; the rerun only wrote what the first didn't.
	if n := env.count(t, `SELECT count(*) FROM catalog.sync_runs WHERE status = 'failed' AND fetched = 3`); n != 1 {
		t.Errorf("%d failed runs after 3 products, want 1", n)
	}
	if n := env.count(t, `SELECT changed FROM catalog.sync_runs WHERE status = 'succeeded'`); n != len(env.src.Items)-3 {
		t.Errorf("rerun changed %d, want %d", n, len(env.src.Items)-3)
	}
}
