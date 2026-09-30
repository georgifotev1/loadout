package main

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newTestApp() *application {
	return &application{logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
}

func TestLivez(t *testing.T) {
	rec := httptest.NewRecorder()
	newTestApp().routes().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/livez", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if rec.Header().Get(requestIDHeader) == "" {
		t.Error("missing request id header")
	}
}

func TestReadyzFailsWhileShuttingDown(t *testing.T) {
	app := newTestApp()
	app.shuttingDown.Store(true)

	rec := httptest.NewRecorder()
	app.routes().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
}

func TestUnknownAPIRouteIsJSON404(t *testing.T) {
	rec := httptest.NewRecorder()
	newTestApp().routes().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/nope", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
}
