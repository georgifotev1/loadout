package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func postJSON(t *testing.T, app *application, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, target, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	app.routes().ServeHTTP(rec, req)
	return rec
}

func TestSavedBuildsAPI(t *testing.T) {
	app := newBuilderApp(t)
	slug := slugs(t, app)
	ctx := context.Background()

	sel := BuildSelection{
		Cpu:         slug["AMD-AM5-R5-7600-BOX"],
		Motherboard: slug["GA-MB-B650-EAGLE-AX"],
		Ram:         []string{slug["KIN-RAM-KF552C36BBE-16"], slug["KIN-RAM-KF552C36BBE-16"]},
		Gpu:         slug["GA-VC-N5070WF3-12GD"],
		Storage:     []string{slug["KIN-SSD-SNV3S-1000G"]},
		Psu:         slug["CRS-PSU-9020296-EU"],
		Case:        slug["LL-CASE-GELI-216X"],
		Cooler:      slug["ARCTIC-FAN-ACFRE00123A"],
	}
	body, err := json.Marshal(sel)
	if err != nil {
		t.Fatal(err)
	}

	rec := postJSON(t, app, "/api/builds", string(body))
	if rec.Code != http.StatusCreated {
		t.Fatalf("save: status %d: %s", rec.Code, rec.Body)
	}
	var saved SavedBuild
	if err := json.Unmarshal(rec.Body.Bytes(), &saved); err != nil {
		t.Fatal(err)
	}
	if len(saved.Slug) != 8 || rec.Header().Get("Location") != "/api/builds/"+saved.Slug {
		t.Errorf("slug %q, location %q", saved.Slug, rec.Header().Get("Location"))
	}
	if len(saved.Check.Parts) != 9 || len(saved.Check.Errors) != 0 || len(saved.PriceChanges) != 0 ||
		saved.SavedTotal != saved.Check.Total || saved.Check.Total.AmountCents == 0 {
		t.Errorf("saved %+v", saved)
	}

	t.Run("read back unchanged", func(t *testing.T) {
		got := decodeBody[SavedBuild](t, "/api/builds/"+saved.Slug, app)
		if got.Slug != saved.Slug || !got.CreatedAt.Equal(saved.CreatedAt) || len(got.Check.Parts) != 9 ||
			got.SavedTotal != saved.SavedTotal || len(got.PriceChanges) != 0 {
			t.Errorf("build %+v", got)
		}
	})

	t.Run("prices changed and a part gone since saved", func(t *testing.T) {
		// The RAM kit (bought twice) costs 5 € more, and the GPU is no longer sold.
		if _, err := app.db.Exec(ctx, `
			UPDATE catalog.prices SET retail_cents = retail_cents + 500
			WHERE product_id = (SELECT id FROM catalog.products WHERE slug = $1)`, sel.Ram[0]); err != nil {
			t.Fatal(err)
		}
		if _, err := app.db.Exec(ctx, `UPDATE catalog.products SET active = false WHERE slug = $1`, sel.Gpu); err != nil {
			t.Fatal(err)
		}

		got := decodeBody[SavedBuild](t, "/api/builds/"+saved.Slug, app)
		if len(got.PriceChanges) != 1 {
			t.Fatalf("price changes %+v", got.PriceChanges)
		}
		c := got.PriceChanges[0]
		if c.Slot != SlotRam || c.Slug != sel.Ram[0] || c.Current.AmountCents != c.Saved.AmountCents+500 {
			t.Errorf("change %+v", c)
		}
		if len(got.Check.Unavailable) != 1 || got.Check.Unavailable[0].Slug != sel.Gpu || len(got.Check.Parts) != 8 {
			t.Errorf("unavailable %+v, %d parts", got.Check.Unavailable, len(got.Check.Parts))
		}
		if got.SavedTotal != saved.SavedTotal {
			t.Errorf("saved total %+v, want %+v", got.SavedTotal, saved.SavedTotal)
		}
	})

	t.Run("bad saves", func(t *testing.T) {
		tests := []struct {
			name, body string
			status     int
			field      string
		}{
			{"empty build", `{}`, http.StatusUnprocessableEntity, "build"},
			{"part not for sale", `{"cpu": "no-such-cpu"}`, http.StatusUnprocessableEntity, "cpu"},
			{"part in the wrong slot", `{"cpu": "` + sel.Psu + `"}`, http.StatusUnprocessableEntity, "cpu"},
			{"unknown field", `{"toaster": "x"}`, http.StatusBadRequest, ""},
			{"malformed", `{`, http.StatusBadRequest, ""},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				rec := postJSON(t, app, "/api/builds", tt.body)
				if rec.Code != tt.status {
					t.Fatalf("status %d, want %d: %s", rec.Code, tt.status, rec.Body)
				}
				var e Error
				if err := json.Unmarshal(rec.Body.Bytes(), &e); err != nil {
					t.Fatal(err)
				}
				if tt.field != "" && (len(e.Error.Fields) != 1 || e.Error.Fields[0].Field != tt.field) {
					t.Errorf("fields %+v, want %s", e.Error.Fields, tt.field)
				}
			})
		}
	})

	t.Run("unknown build is 404", func(t *testing.T) {
		if rec := get(t, app, "/api/builds/zzzzzzzz"); rec.Code != http.StatusNotFound {
			t.Errorf("status %d, want 404", rec.Code)
		}
	})
}
