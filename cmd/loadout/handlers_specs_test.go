package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
)

func send(t *testing.T, app *application, method, target, body string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	for _, c := range cookies {
		req.AddCookie(c)
	}
	app.routes().ServeHTTP(rec, req)
	return rec
}

func decode[T any](t *testing.T, rec *httptest.ResponseRecorder, status int) T {
	t.Helper()
	if rec.Code != status {
		t.Fatalf("status %d, want %d: %s", rec.Code, status, rec.Body)
	}
	var v T
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatal(err)
	}
	return v
}

func field(ps ProductSpecs, name string) SpecField {
	for _, f := range ps.Fields {
		if f.Name == name {
			return f
		}
	}
	return SpecField{}
}

func TestSpecsAdminAPI(t *testing.T) {
	app := newCatalogApp(t)
	admin := adminCookie(t, app)

	queue := decode[ReviewQueue](t, get(t, app, "/api/admin/specs/review-queue?category=cpu", admin), http.StatusOK)
	t.Run("review queue", func(t *testing.T) {
		// The vali adapter extracts no specs yet, so every fixture CPU is incomplete.
		if queue.Total != 7 || len(queue.Items) != 7 || queue.Page != 1 {
			t.Fatalf("total=%d len=%d page=%d", queue.Total, len(queue.Items), queue.Page)
		}
		for _, it := range queue.Items {
			if it.Category != CategoryCpu || it.ProductName == "" || len(it.Missing) == 0 {
				t.Errorf("item %+v", it)
			}
		}
	})

	cpu := queue.Items[0].ProductId
	target := "/api/admin/specs/" + cpu.String()

	t.Run("product specs", func(t *testing.T) {
		ps := decode[ProductSpecs](t, get(t, app, target, admin), http.StatusOK)
		if ps.ProductId != cpu || ps.Sku == "" || ps.Slug == "" || !ps.Active || len(ps.Attributes) == 0 ||
			ps.Status != SpecStatusIncomplete || len(ps.Fields) != 6 {
			t.Fatalf("specs %+v", ps)
		}
		if f := field(ps, "tdp_w"); f.Kind != SpecKindInteger || !f.Required || f.Value != nil {
			t.Errorf("tdp_w %+v", f)
		}
	})

	t.Run("set values until complete, then clear one", func(t *testing.T) {
		body := `{"values": {"socket": "AM5", "tdp_w": 65, "igpu": true, "cooler_included": false}}`
		ps := decode[ProductSpecs](t, send(t, app, http.MethodPatch, target, body, admin), http.StatusOK)
		// memory_types is derived from the socket.
		if ps.Status != SpecStatusApproved || len(ps.Missing) != 0 || field(ps, "tdp_w").Admin != 65.0 ||
			!slices.Equal(anyStrings(field(ps, "memory_types").Value), []string{"DDR5"}) {
			t.Fatalf("after set %+v", ps)
		}
		q := decode[ReviewQueue](t, get(t, app, "/api/admin/specs/review-queue?category=cpu", admin), http.StatusOK)
		if q.Total != 6 {
			t.Errorf("queue total %d, want 6", q.Total)
		}

		ps = decode[ProductSpecs](t, send(t, app, http.MethodPatch, target, `{"values": {"tdp_w": null}}`, admin), http.StatusOK)
		if f := field(ps, "tdp_w"); ps.Status != SpecStatusIncomplete || f.Admin != nil || f.Value != nil ||
			!slices.Equal(ps.Missing, []string{"tdp_w"}) {
			t.Errorf("after clear %+v", ps)
		}
	})

	t.Run("a wrong value is 422 naming the field, and nothing is stored", func(t *testing.T) {
		rec := send(t, app, http.MethodPatch, target, `{"values": {"socket": "AM4", "tdp_w": "65 W", "cores": 6}}`, admin)
		e := decode[Error](t, rec, http.StatusUnprocessableEntity)
		want := []FieldError{{Field: "cores", Code: "unknown"}, {Field: "tdp_w", Code: "invalid_type"}}
		if !slices.Equal(e.Error.Fields, want) {
			t.Errorf("fields %+v, want %+v", e.Error.Fields, want)
		}
		ps := decode[ProductSpecs](t, get(t, app, target, admin), http.StatusOK)
		if field(ps, "socket").Value != "AM5" {
			t.Errorf("socket %v, want the earlier AM5", field(ps, "socket").Value)
		}
	})

	t.Run("not found", func(t *testing.T) {
		var mouse string
		err := app.db.QueryRow(context.Background(),
			`SELECT id::text FROM catalog.products WHERE category LIKE 'peripheral.%' LIMIT 1`).Scan(&mouse)
		if err != nil {
			t.Fatal(err)
		}
		for _, id := range []string{mouse, "6f1c8e1e-4a3b-4f0e-9a51-2a8d3c1b7e90"} {
			if rec := get(t, app, "/api/admin/specs/"+id, admin); rec.Code != http.StatusNotFound {
				t.Errorf("%s: status %d, want 404", id, rec.Code)
			}
			if rec := send(t, app, http.MethodPatch, "/api/admin/specs/"+id, `{"values": {}}`, admin); rec.Code != http.StatusNotFound {
				t.Errorf("%s: patch status %d, want 404", id, rec.Code)
			}
		}
	})

	t.Run("bad input", func(t *testing.T) {
		for _, target := range []string{
			"/api/admin/specs/review-queue?category=peripheral.mouse",
			"/api/admin/specs/review-queue?page=abc",
			"/api/admin/specs/review-queue?page_size=1000",
			"/api/admin/specs/not-a-uuid",
		} {
			if rec := get(t, app, target, admin); rec.Code != http.StatusUnprocessableEntity {
				t.Errorf("%s: status %d, want 422", target, rec.Code)
			}
		}
		if rec := send(t, app, http.MethodPatch, target, `{"socket": "AM5"}`, admin); rec.Code != http.StatusBadRequest {
			t.Errorf("body without values: status %d, want 400", rec.Code)
		}
	})
}

func anyStrings(v any) []string {
	l, _ := v.([]any)
	out := make([]string, 0, len(l))
	for _, e := range l {
		s, _ := e.(string)
		out = append(out, s)
	}
	return out
}
