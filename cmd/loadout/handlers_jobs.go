package main

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/gfotev/loadout/internal/catalog"
	"github.com/gfotev/loadout/internal/platform/validate"
)

const (
	defaultDiscardedLimit = 50
	maxDiscardedLimit     = 100
)

type syncTriggeredJSON struct {
	JobID int64 `json:"job_id"`
	// AlreadyQueued is true when a sync was already waiting or running; no new job was added.
	AlreadyQueued bool `json:"already_queued"`
}

// POST /api/admin/sync
func (app *application) triggerSync(w http.ResponseWriter, r *http.Request) {
	res, err := app.jobs.Insert(r.Context(), catalog.SyncArgs{}, nil)
	if err != nil {
		app.handleError(w, r, err)
		return
	}
	app.writeJSON(w, r, http.StatusAccepted, syncTriggeredJSON{
		JobID:         res.Job.ID,
		AlreadyQueued: res.UniqueSkippedAsDuplicate,
	})
}

type discardedJobJSON struct {
	ID          int64           `json:"id"`
	Kind        string          `json:"kind"`
	Queue       string          `json:"queue"`
	Args        json.RawMessage `json:"args"`
	Attempt     int             `json:"attempt"`
	MaxAttempts int             `json:"max_attempts"`
	LastError   string          `json:"last_error"`
	CreatedAt   time.Time       `json:"created_at"`
	FinalizedAt *time.Time      `json:"finalized_at"`
}

type discardedJobsJSON struct {
	Jobs []discardedJobJSON `json:"jobs"`
}

// GET /api/admin/jobs/discarded?limit=
// Jobs that failed every attempt, newest first.
func (app *application) discardedJobs(w http.ResponseWriter, r *http.Request) {
	var v validate.Validator
	limit := queryInt(&v, r.URL.Query(), "limit", defaultDiscardedLimit)
	v.Check(limit >= 1 && limit <= maxDiscardedLimit, "limit", "out_of_range")
	if err := v.Err(); err != nil {
		app.handleError(w, r, err)
		return
	}

	res, err := app.jobs.JobList(r.Context(), river.NewJobListParams().
		States(rivertype.JobStateDiscarded).
		OrderBy(river.JobListOrderByFinalizedAt, river.SortOrderDesc).
		First(limit))
	if err != nil {
		app.handleError(w, r, err)
		return
	}

	resp := discardedJobsJSON{Jobs: make([]discardedJobJSON, 0, len(res.Jobs))}
	for _, j := range res.Jobs {
		var lastErr string
		if len(j.Errors) > 0 {
			lastErr = j.Errors[len(j.Errors)-1].Error
		}
		resp.Jobs = append(resp.Jobs, discardedJobJSON{
			ID:          j.ID,
			Kind:        j.Kind,
			Queue:       j.Queue,
			Args:        j.EncodedArgs,
			Attempt:     j.Attempt,
			MaxAttempts: j.MaxAttempts,
			LastError:   lastErr,
			CreatedAt:   j.CreatedAt,
			FinalizedAt: j.FinalizedAt,
		})
	}
	app.writeJSON(w, r, http.StatusOK, resp)
}
