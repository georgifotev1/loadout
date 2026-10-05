package catalog

import (
	"context"
	"errors"
	"time"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/gfotev/loadout/internal/platform/jobs"
)

const (
	syncInterval = 4 * time.Hour
	// syncJobTimeout caps one run; River cancels the job's context after it.
	syncJobTimeout = 30 * time.Minute
)

// SyncArgs is the job for one full supplier sync.
type SyncArgs struct{}

func (SyncArgs) Kind() string { return "supplier_sync" }

func (SyncArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{
		Queue: jobs.QueueSync,
		// No retries: a failed run is discarded (visible to admins) and the next scheduled one runs.
		MaxAttempts: 1,
		// At most one sync job waiting or running; inserting another meanwhile is a no-op.
		UniqueOpts: river.UniqueOpts{ByState: []rivertype.JobState{
			rivertype.JobStateAvailable,
			rivertype.JobStatePending,
			rivertype.JobStateRunning,
			rivertype.JobStateScheduled,
		}},
	}
}

// SyncWorker runs Sync. It's safe to run twice: Sync only writes what changed.
type SyncWorker struct {
	river.WorkerDefaults[SyncArgs]
	svc *Service
}

func NewSyncWorker(svc *Service) *SyncWorker { return &SyncWorker{svc: svc} }

func (w *SyncWorker) Timeout(*river.Job[SyncArgs]) time.Duration { return syncJobTimeout }

func (w *SyncWorker) Work(ctx context.Context, job *river.Job[SyncArgs]) error {
	_, err := w.svc.Sync(ctx)
	if errors.Is(err, ErrSyncInProgress) {
		return nil // `loadout sync` is running one right now
	}
	return err
}

// PeriodicSync schedules a sync every syncInterval.
func PeriodicSync() *river.PeriodicJob {
	return river.NewPeriodicJob(
		river.PeriodicInterval(syncInterval),
		func() (river.JobArgs, *river.InsertOpts) { return SyncArgs{}, nil },
		// River keeps the schedule in memory, so a deploy restarts the 4h countdown. RunOnStart
		// syncs on each start so frequent deploys can't keep pushing the sync back.
		&river.PeriodicJobOpts{ID: "supplier_sync", RunOnStart: true},
	)
}
