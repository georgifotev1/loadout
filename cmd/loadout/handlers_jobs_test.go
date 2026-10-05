package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

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
	return app
}

func post(t *testing.T, app *application, target string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	app.routes().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, target, nil))
	return rec
}

func TestJobsAdminAPI(t *testing.T) {
	app := newJobsApp(t)

	t.Run("sync now queues one job", func(t *testing.T) {
		var first, second syncTriggeredJSON
		for _, out := range []*syncTriggeredJSON{&first, &second} {
			rec := post(t, app, "/api/admin/sync")
			if rec.Code != http.StatusAccepted {
				t.Fatalf("status %d: %s", rec.Code, rec.Body)
			}
			if err := json.Unmarshal(rec.Body.Bytes(), out); err != nil {
				t.Fatal(err)
			}
		}
		if first.AlreadyQueued || first.JobID == 0 {
			t.Errorf("first = %+v, want a new job", first)
		}
		if !second.AlreadyQueued || second.JobID != first.JobID {
			t.Errorf("second = %+v, want the same job %d", second, first.JobID)
		}
	})

	t.Run("discarded jobs", func(t *testing.T) {
		// Fail the queued sync job by hand, the way River records a last failed attempt.
		_, err := app.db.Exec(context.Background(), `
			UPDATE river.river_job
			SET state = 'discarded', finalized_at = now(), attempt = 1,
			    errors = ARRAY['{"at":"2026-10-05T00:00:00Z","attempt":1,"error":"fetch products: boom"}'::jsonb]
			WHERE kind = 'supplier_sync'`)
		if err != nil {
			t.Fatal(err)
		}

		rec := get(t, app, "/api/admin/jobs/discarded")
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d: %s", rec.Code, rec.Body)
		}
		var body discardedJobsJSON
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if len(body.Jobs) != 1 {
			t.Fatalf("%d jobs, want 1", len(body.Jobs))
		}
		j := body.Jobs[0]
		if j.Kind != "supplier_sync" || j.Queue != jobs.QueueSync || j.LastError != "fetch products: boom" || j.FinalizedAt == nil {
			t.Errorf("job = %+v", j)
		}
	})

	t.Run("bad limit is 422", func(t *testing.T) {
		for _, target := range []string{
			"/api/admin/jobs/discarded?limit=0",
			"/api/admin/jobs/discarded?limit=1000",
			"/api/admin/jobs/discarded?limit=abc",
		} {
			if rec := get(t, app, target); rec.Code != http.StatusUnprocessableEntity {
				t.Errorf("%s: status %d, want 422", target, rec.Code)
			}
		}
	})
}
