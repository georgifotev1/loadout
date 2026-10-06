package main

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/gfotev/loadout/internal/catalog"
	"github.com/gfotev/loadout/internal/platform/validate"
)

const (
	defaultDiscardedLimit = 50
	maxDiscardedLimit     = 100
)

// TriggerSync handles POST /api/admin/sync.
func (app *application) TriggerSync(w http.ResponseWriter, r *http.Request) {
	res, err := app.jobs.Insert(r.Context(), catalog.SyncArgs{}, nil)
	if err != nil {
		app.handleError(w, r, err)
		return
	}
	app.writeJSON(w, r, http.StatusAccepted, SyncTriggered{
		JobId:         res.Job.ID,
		AlreadyQueued: res.UniqueSkippedAsDuplicate,
	})
}

// ListSyncRuns handles GET /api/admin/sync-runs.
func (app *application) ListSyncRuns(w http.ResponseWriter, r *http.Request, params ListSyncRunsParams) {
	page := valueOr(params.Page, 1)
	pageSize := valueOr(params.PageSize, defaultPageSize)

	res, err := app.catalog.SyncRuns(r.Context(), page, pageSize)
	if err != nil {
		app.handleError(w, r, err)
		return
	}
	// The states in which SyncArgs counts as a duplicate are exactly "waiting or running".
	queued, err := app.jobs.JobList(r.Context(), river.NewJobListParams().
		Kinds(catalog.SyncArgs{}.Kind()).
		States(catalog.SyncArgs{}.InsertOpts().UniqueOpts.ByState...).
		First(1))
	if err != nil {
		app.handleError(w, r, err)
		return
	}

	resp := SyncRunList{
		Items:    make([]SyncRun, 0, len(res.Runs)),
		Page:     page,
		PageSize: pageSize,
		Total:    res.Total,
		Queued:   len(queued.Jobs) > 0,
	}
	for _, run := range res.Runs {
		resp.Items = append(resp.Items, syncRunJSON(run))
	}
	app.writeJSON(w, r, http.StatusOK, resp)
}

func syncRunJSON(run catalog.SyncRun) SyncRun {
	out := SyncRun{
		Id:         run.ID,
		StartedAt:  run.StartedAt,
		FinishedAt: run.FinishedAt,
		Status:     SyncStatus(run.Status),
		Fetched:    run.Fetched,
		Changed:    run.Changed,
		Errors:     run.Errors,
	}
	if run.ErrorMessage != "" {
		out.ErrorMessage = &run.ErrorMessage
	}
	return out
}

// ListDiscardedJobs handles GET /api/admin/jobs/discarded.
func (app *application) ListDiscardedJobs(w http.ResponseWriter, r *http.Request, params ListDiscardedJobsParams) {
	limit := valueOr(params.Limit, defaultDiscardedLimit)
	var v validate.Validator
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
	total, err := app.jobs.DiscardedCount(r.Context())
	if err != nil {
		app.handleError(w, r, err)
		return
	}

	resp := DiscardedJobList{Items: make([]DiscardedJob, 0, len(res.Jobs)), Total: total}
	for _, j := range res.Jobs {
		var args map[string]any
		if err := json.Unmarshal(j.EncodedArgs, &args); err != nil {
			app.handleError(w, r, fmt.Errorf("job %d args: %w", j.ID, err))
			return
		}
		if args == nil {
			args = map[string]any{}
		}
		errs := make([]JobAttemptError, 0, len(j.Errors))
		for _, e := range j.Errors {
			errs = append(errs, JobAttemptError{Attempt: e.Attempt, At: e.At, Error: e.Error})
		}
		resp.Items = append(resp.Items, DiscardedJob{
			Id:          j.ID,
			Kind:        j.Kind,
			Queue:       j.Queue,
			Args:        args,
			Attempt:     j.Attempt,
			MaxAttempts: j.MaxAttempts,
			Errors:      errs,
			CreatedAt:   j.CreatedAt,
			FinalizedAt: j.FinalizedAt,
		})
	}
	app.writeJSON(w, r, http.StatusOK, resp)
}

// RetryDiscardedJob handles POST /api/admin/jobs/discarded/{job_id}/retry.
func (app *application) RetryDiscardedJob(w http.ResponseWriter, r *http.Request, jobID JobID) {
	if err := app.jobs.RetryDiscarded(r.Context(), jobID); err != nil {
		app.handleError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// DeleteDiscardedJob handles DELETE /api/admin/jobs/discarded/{job_id}.
func (app *application) DeleteDiscardedJob(w http.ResponseWriter, r *http.Request, jobID JobID) {
	if err := app.jobs.DeleteDiscarded(r.Context(), jobID); err != nil {
		app.handleError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
