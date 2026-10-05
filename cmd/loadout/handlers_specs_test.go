package main

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestSpecsAdminAPI(t *testing.T) {
	app := newCatalogApp(t)

	t.Run("review queue", func(t *testing.T) {
		rec := get(t, app, "/api/admin/specs/review-queue?category=cpu")
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d: %s", rec.Code, rec.Body)
		}
		var body reviewQueueJSON
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		// The vali adapter extracts no specs yet, so every fixture CPU is incomplete.
		if body.Total != 7 || len(body.Items) != 7 || body.Page != 1 {
			t.Fatalf("total=%d len=%d page=%d", body.Total, len(body.Items), body.Page)
		}
		for _, it := range body.Items {
			if it.Category != "cpu" || it.ProductID == "" || len(it.Missing) == 0 {
				t.Errorf("item %+v", it)
			}
		}
	})

	t.Run("bad query parameters are 422", func(t *testing.T) {
		for _, target := range []string{
			"/api/admin/specs/review-queue?category=peripheral.mouse",
			"/api/admin/specs/review-queue?page=abc",
			"/api/admin/specs/review-queue?page_size=1000",
		} {
			if rec := get(t, app, target); rec.Code != http.StatusUnprocessableEntity {
				t.Errorf("%s: status %d, want 422", target, rec.Code)
			}
		}
	})
}
