package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"
)

const (
	defaultPageSize = 20
	maxPageSize     = 100
	maxLatency      = 30 * time.Second
)

type server struct {
	store  *store
	logger *slog.Logger
	// Defaults for the test knobs, used when a request doesn't set ?latency_ms= / ?fail_rate=.
	latency  time.Duration
	failRate float64
	// rand decides injected failures; tests replace it to make them deterministic.
	rand func() float64
}

func (s *server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	v1 := http.NewServeMux()
	v1.HandleFunc("GET /v1/products", s.listProducts)
	v1.HandleFunc("GET /v1/products/{sku}", s.getProduct)
	mux.Handle("/v1/", s.chaos(v1))

	// Test controls. Not part of Vali's API, so the chaos knobs don't apply.
	mux.HandleFunc("POST /admin/mutate", s.mutate)
	mux.HandleFunc("POST /admin/reset", s.reset)

	return s.logRequest(mux)
}

// productPage is one page of GET /v1/products.
// ASSUMPTION: Vali paginates with ?page= and tells the client whether there is more. Their real
// mechanism (page numbers, cursors, a total) is unknown.
type productPage struct {
	Products []*product `json:"products"`
	Page     int        `json:"page"`
	PageSize int        `json:"page_size"`
	Total    int        `json:"total"`
	NextPage *int       `json:"next_page"` // null on the last page
}

// GET /v1/products?since=<RFC 3339>&page=<1..>&page_size=<1..100>
func (s *server) listProducts(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	var since time.Time
	if v := q.Get("since"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_since", "since must be RFC 3339, e.g. 2026-09-30T08:00:00Z")
			return
		}
		since = t
	}
	page, ok := intParam(w, q.Get("page"), "page", 1, 1, 1<<20)
	if !ok {
		return
	}
	pageSize, ok := intParam(w, q.Get("page_size"), "page_size", defaultPageSize, 1, maxPageSize)
	if !ok {
		return
	}

	products, total := s.store.list(since, page, pageSize)
	resp := productPage{Products: products, Page: page, PageSize: pageSize, Total: total}
	if resp.Products == nil {
		resp.Products = []*product{}
	}
	if page*pageSize < total {
		next := page + 1
		resp.NextPage = &next
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *server) getProduct(w http.ResponseWriter, r *http.Request) {
	p, err := s.store.get(r.PathValue("sku"))
	if errors.Is(err, errNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "no product with that sku")
		return
	}
	writeJSON(w, http.StatusOK, p)
}

// POST /admin/mutate {"sku": "...", "rrp_cents": 6990, "stock": {"Plovdiv": 0}, ...}
// Responds with the product after the change.
func (s *server) mutate(w http.ResponseWriter, r *http.Request) {
	var m mutation
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&m); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}
	if msg := validateMutation(m); msg != "" {
		writeError(w, http.StatusUnprocessableEntity, "validation_failed", msg)
		return
	}
	p, err := s.store.mutate(m)
	if errors.Is(err, errNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "no product with that sku")
		return
	}
	s.logger.Info("product mutated", "sku", m.SKU)
	writeJSON(w, http.StatusOK, p)
}

func validateMutation(m mutation) string {
	switch {
	case m.SKU == "":
		return "sku is required"
	case m.DealerPriceCents == nil && m.RRPCents == nil && m.Stock == nil && m.Specs == nil &&
		m.Name == nil && m.Delisted == nil:
		return "nothing to change"
	case m.DealerPriceCents != nil && *m.DealerPriceCents <= 0:
		return "dealer_price_cents must be positive"
	case m.RRPCents != nil && *m.RRPCents <= 0:
		return "rrp_cents must be positive"
	case m.Name != nil && *m.Name == "":
		return "name must not be empty"
	}
	for wh, qty := range m.Stock {
		if wh == "" || qty < 0 {
			return "stock needs warehouse names and quantities >= 0"
		}
	}
	return ""
}

func (s *server) reset(w http.ResponseWriter, r *http.Request) {
	if err := s.store.reset(); err != nil {
		s.logger.Error("reset", "err", err)
		writeError(w, http.StatusInternalServerError, "internal", "reset failed")
		return
	}
	s.logger.Info("fixtures reset")
	w.WriteHeader(http.StatusNoContent)
}

// chaos applies the test knobs: wait latency_ms, then fail with 503 with probability fail_rate.
// Query parameters override the defaults from the command-line flags.
func (s *server) chaos(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		latency := s.latency
		if v := q.Get("latency_ms"); v != "" {
			ms, ok := intParam(w, v, "latency_ms", 0, 0, int(maxLatency/time.Millisecond))
			if !ok {
				return
			}
			latency = time.Duration(ms) * time.Millisecond
		}
		failRate := s.failRate
		if v := q.Get("fail_rate"); v != "" {
			f, err := strconv.ParseFloat(v, 64)
			if err != nil || f < 0 || f > 1 {
				writeError(w, http.StatusBadRequest, "invalid_fail_rate", "fail_rate must be between 0 and 1")
				return
			}
			failRate = f
		}

		if latency > 0 {
			t := time.NewTimer(latency)
			defer t.Stop()
			select {
			case <-r.Context().Done():
				return // the client gave up, which is usually what a latency test wants to see
			case <-t.C:
			}
		}
		if failRate > 0 && s.rand() < failRate {
			writeError(w, http.StatusServiceUnavailable, "unavailable", "injected failure (fail_rate)")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *server) logRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		s.logger.LogAttrs(r.Context(), slog.LevelInfo, "http request",
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.String("query", r.URL.RawQuery),
			slog.Int("status", sw.status),
			slog.Duration("duration", time.Since(start)),
		)
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

// intParam parses an optional integer query parameter in [lo, hi]. On failure it writes a 400
// and returns false.
func intParam(w http.ResponseWriter, v, name string, def, lo, hi int) (int, bool) {
	if v == "" {
		return def, true
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < lo || n > hi {
		writeError(w, http.StatusBadRequest, "invalid_"+name, fmt.Sprintf("%s must be an integer in [%d, %d]", name, lo, hi))
		return 0, false
	}
	return n, true
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v) // nothing useful to do if the client went away
}

// writeError uses the same {"error": {"code", "message"}} shape as the Loadout API, which keeps
// test output familiar. ASSUMPTION: Vali's real error format is unknown.
func writeError(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": msg}})
}
