package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/riverqueue/river/rivertype"

	"github.com/gfotev/loadout/internal/catalog"
	"github.com/gfotev/loadout/internal/configurator"
	"github.com/gfotev/loadout/internal/orders"
	"github.com/gfotev/loadout/internal/platform/jobs"
)

func newJobsApp(t *testing.T) *application {
	t.Helper()
	app := newCatalogApp(t)
	c, err := jobs.New(app.db, jobs.Config{Logger: app.logger})
	if err != nil {
		t.Fatal(err)
	}
	app.jobs = c
	app.orders = orders.NewService(app.db, app.catalog, configurator.NewService(app.db, app.catalog, app.specs), c)
	return app
}

func post(t *testing.T, app *application, target string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, target, nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	app.routes().ServeHTTP(rec, req)
	return rec
}

// discard fails job id by hand, the way River records a last failed attempt.
func discard(t *testing.T, app *application, id int64) {
	t.Helper()
	_, err := app.db.Exec(context.Background(), `
		UPDATE river.river_job
		SET state = 'discarded', finalized_at = now(), attempt = 1,
		    errors = ARRAY['{"at":"2026-10-05T00:00:00Z","attempt":1,"error":"fetch products: boom"}'::jsonb]
		WHERE id = $1`, id)
	if err != nil {
		t.Fatal(err)
	}
}

func TestJobsAdminAPI(t *testing.T) {
	app := newJobsApp(t)
	admin := adminCookie(t, app)
	var jobID int64

	t.Run("sync now queues one job", func(t *testing.T) {
		first := decode[SyncTriggered](t, post(t, app, "/api/admin/sync", admin), http.StatusAccepted)
		second := decode[SyncTriggered](t, post(t, app, "/api/admin/sync", admin), http.StatusAccepted)
		if first.AlreadyQueued || first.JobId == 0 {
			t.Errorf("first = %+v, want a new job", first)
		}
		if !second.AlreadyQueued || second.JobId != first.JobId {
			t.Errorf("second = %+v, want the same job %d", second, first.JobId)
		}
		jobID = first.JobId
	})

	t.Run("sync runs", func(t *testing.T) {
		body := decode[SyncRunList](t, get(t, app, "/api/admin/sync-runs", admin), http.StatusOK)
		if body.Total != 1 || len(body.Items) != 1 || !body.Queued {
			t.Fatalf("body = %+v, want the setup's run and a queued job", body)
		}
		run := body.Items[0]
		if run.Status != SyncStatusSucceeded || run.FinishedAt == nil || run.ErrorMessage != nil || run.Fetched == 0 {
			t.Errorf("run = %+v", run)
		}
	})

	t.Run("discarded jobs", func(t *testing.T) {
		discard(t, app, jobID)

		body := decode[DiscardedJobList](t, get(t, app, "/api/admin/jobs/discarded", admin), http.StatusOK)
		if body.Total != 1 || len(body.Items) != 1 {
			t.Fatalf("body = %+v, want 1 job", body)
		}
		j := body.Items[0]
		if j.Kind != "supplier_sync" || j.Queue != jobs.QueueSync || j.FinalizedAt == nil || j.Args == nil {
			t.Errorf("job = %+v", j)
		}
		if len(j.Errors) != 1 || j.Errors[0].Error != "fetch products: boom" || j.Errors[0].Attempt != 1 {
			t.Errorf("errors = %+v", j.Errors)
		}

		runs := decode[SyncRunList](t, get(t, app, "/api/admin/sync-runs", admin), http.StatusOK)
		if runs.Queued {
			t.Error("a discarded sync job still counts as queued")
		}
	})

	t.Run("overview", func(t *testing.T) {
		body := decode[AdminOverview](t, get(t, app, "/api/admin/overview", admin), http.StatusOK)
		if body.DiscardedJobs != 1 || body.Products.Active == 0 || body.LastSync == nil || body.LastSync.Status != SyncStatusSucceeded ||
			body.Orders != (AdminOrderCounts{}) {
			t.Errorf("overview = %+v", body)
		}
		// Only builder parts with all required specs leave the queue; the fixtures have none.
		if body.SpecsIncomplete == 0 {
			t.Error("specs_incomplete = 0, want the fixtures' builder parts")
		}
	})

	t.Run("retry", func(t *testing.T) {
		target := fmt.Sprintf("/api/admin/jobs/discarded/%d/retry", jobID)
		if rec := send(t, app, http.MethodPost, target, "", admin); rec.Code != http.StatusNoContent {
			t.Fatalf("status %d: %s", rec.Code, rec.Body)
		}
		j, err := app.jobs.JobGet(context.Background(), jobID)
		if err != nil {
			t.Fatal(err)
		}
		if j.State != rivertype.JobStateAvailable || j.MaxAttempts != 2 {
			t.Errorf("state %s, max attempts %d; want available with one more attempt", j.State, j.MaxAttempts)
		}
		// No longer discarded, so neither endpoint touches it.
		if rec := send(t, app, http.MethodPost, target, "", admin); rec.Code != http.StatusNotFound {
			t.Errorf("second retry: status %d, want 404", rec.Code)
		}
		if rec := send(t, app, http.MethodDelete, fmt.Sprintf("/api/admin/jobs/discarded/%d", jobID), "", admin); rec.Code != http.StatusNotFound {
			t.Errorf("delete a queued job: status %d, want 404", rec.Code)
		}
	})

	t.Run("delete", func(t *testing.T) {
		discard(t, app, jobID)
		target := fmt.Sprintf("/api/admin/jobs/discarded/%d", jobID)
		if rec := send(t, app, http.MethodDelete, target, "", admin); rec.Code != http.StatusNoContent {
			t.Fatalf("status %d: %s", rec.Code, rec.Body)
		}
		body := decode[DiscardedJobList](t, get(t, app, "/api/admin/jobs/discarded", admin), http.StatusOK)
		if body.Total != 0 || len(body.Items) != 0 {
			t.Errorf("body = %+v, want no jobs", body)
		}
		if rec := send(t, app, http.MethodDelete, target, "", admin); rec.Code != http.StatusNotFound {
			t.Errorf("second delete: status %d, want 404", rec.Code)
		}
	})

	t.Run("retry while an equal sync is queued is 409", func(t *testing.T) {
		// Two failed syncs: inserting the second is allowed once the first is discarded.
		var ids []int64
		for range 2 {
			res, err := app.jobs.Insert(context.Background(), catalog.SyncArgs{}, nil)
			if err != nil {
				t.Fatal(err)
			}
			discard(t, app, res.Job.ID)
			ids = append(ids, res.Job.ID)
		}
		retry := func(id int64) *httptest.ResponseRecorder {
			return send(t, app, http.MethodPost, fmt.Sprintf("/api/admin/jobs/discarded/%d/retry", id), "", admin)
		}
		if rec := retry(ids[0]); rec.Code != http.StatusNoContent {
			t.Fatalf("first retry: status %d: %s", rec.Code, rec.Body)
		}
		rec := retry(ids[1])
		if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), `"already_queued"`) {
			t.Errorf("second retry: status %d: %s, want 409 already_queued", rec.Code, rec.Body)
		}
		// It stays discarded, to retry once the first one is done.
		if j, _ := app.jobs.JobGet(context.Background(), ids[1]); j.State != rivertype.JobStateDiscarded {
			t.Errorf("second job state %s, want discarded", j.State)
		}
	})

	t.Run("bad input is 422", func(t *testing.T) {
		for _, target := range []string{
			"/api/admin/jobs/discarded?limit=0",
			"/api/admin/jobs/discarded?limit=1000",
			"/api/admin/jobs/discarded?limit=abc",
			"/api/admin/sync-runs?page=0",
			"/api/admin/sync-runs?page_size=1000",
		} {
			if rec := get(t, app, target, admin); rec.Code != http.StatusUnprocessableEntity {
				t.Errorf("%s: status %d, want 422", target, rec.Code)
			}
		}
		if rec := send(t, app, http.MethodPost, "/api/admin/jobs/discarded/abc/retry", "", admin); rec.Code != http.StatusUnprocessableEntity {
			t.Errorf("job id abc: status %d, want 422", rec.Code)
		}
	})

	t.Run("admins only", func(t *testing.T) {
		for _, target := range []string{"/api/admin/overview", "/api/admin/sync-runs", "/api/admin/jobs/discarded"} {
			if rec := get(t, app, target); rec.Code != http.StatusUnauthorized {
				t.Errorf("%s: status %d, want 401", target, rec.Code)
			}
		}
	})
}
