package vali

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gfotev/loadout/internal/supplier"
)

// pagedServer serves n products in pages of pageSize, like mock-vali. fail decides, per request
// number (1-based), whether to answer with that status instead.
func pagedServer(t *testing.T, n int, fail func(req int) int) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var reqs atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req := int(reqs.Add(1))
		if r.URL.Path != "/v1/products" {
			http.NotFound(w, r)
			return
		}
		if fail != nil {
			if status := fail(req); status != 0 {
				w.WriteHeader(status)
				return
			}
		}
		pg, _ := strconv.Atoi(r.URL.Query().Get("page"))
		size, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
		var resp page
		resp.Products = []RawProduct{}
		for i := (pg - 1) * size; i < min(pg*size, n); i++ {
			resp.Products = append(resp.Products, RawProduct{
				SKU:          fmt.Sprintf("SKU-%03d", i),
				CategoryPath: []string{"Компютърни компоненти", "Процесори"},
				Currency:     "EUR",
			})
		}
		if pg*size < n {
			next := pg + 1
			resp.NextPage = &next
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	t.Cleanup(srv.Close)
	return srv, &reqs
}

func newTestClient(url string) *MockClient {
	c := NewMockClient(url, nil)
	c.backoff = time.Millisecond
	return c
}

func collect(t *testing.T, c *MockClient) ([]supplier.Product, error) {
	t.Helper()
	var got []supplier.Product
	for p, err := range c.Products(context.Background()) {
		if err != nil {
			return got, err
		}
		got = append(got, p)
	}
	return got, nil
}

func TestProductsPaginates(t *testing.T) {
	srv, reqs := pagedServer(t, 250, nil) // 3 pages of 100
	got, err := collect(t, newTestClient(srv.URL))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 250 || got[0].SKU != "SKU-000" || got[249].SKU != "SKU-249" {
		t.Fatalf("got %d products (%v … %v)", len(got), got[0].SKU, got[len(got)-1].SKU)
	}
	if reqs.Load() != 3 {
		t.Errorf("%d requests, want 3", reqs.Load())
	}
}

func TestProductsSkipsUnmappableItems(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"products": [
			{"sku": "A", "category_path": ["x", "Процесори"], "currency": "EUR"},
			{"sku": "B", "category_path": ["x", "Принтери"], "currency": "EUR"},
			{"sku": "C", "category_path": ["x", "Памети"], "currency": "EUR"}
		], "next_page": null}`))
	}))
	defer srv.Close()

	var skus []string
	var itemErrs []*supplier.ItemError
	for p, err := range newTestClient(srv.URL).Products(context.Background()) {
		var ie *supplier.ItemError
		switch {
		case errors.As(err, &ie):
			itemErrs = append(itemErrs, ie)
		case err != nil:
			t.Fatal(err)
		default:
			skus = append(skus, p.SKU)
		}
	}
	if !slices.Equal(skus, []string{"A", "C"}) || len(itemErrs) != 1 || itemErrs[0].SKU != "B" {
		t.Errorf("products %v, item errors %v; want A and C, and an error for B", skus, itemErrs)
	}
}

func TestProductsRetriesServerErrors(t *testing.T) {
	// Requests 1 and 2 fail (page 1 succeeds on the 3rd attempt), then page 2 fails once.
	srv, reqs := pagedServer(t, 150, func(req int) int {
		if req <= 2 || req == 4 {
			return http.StatusServiceUnavailable
		}
		return 0
	})
	got, err := collect(t, newTestClient(srv.URL))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 150 || reqs.Load() != 5 {
		t.Errorf("got %d products in %d requests, want 150 in 5", len(got), reqs.Load())
	}
}

func TestProductsGivesUp(t *testing.T) {
	t.Run("after the last attempt", func(t *testing.T) {
		srv, reqs := pagedServer(t, 10, func(int) int { return http.StatusBadGateway })
		_, err := collect(t, newTestClient(srv.URL))
		if err == nil || reqs.Load() != attempts {
			t.Fatalf("err = %v after %d requests, want an error after %d", err, reqs.Load(), attempts)
		}
	})
	t.Run("at once on a client error", func(t *testing.T) {
		srv, reqs := pagedServer(t, 10, func(int) int { return http.StatusBadRequest })
		_, err := collect(t, newTestClient(srv.URL))
		if err == nil || reqs.Load() != 1 {
			t.Fatalf("err = %v after %d requests, want an error after 1", err, reqs.Load())
		}
	})
	t.Run("products before the error are still yielded", func(t *testing.T) {
		srv, _ := pagedServer(t, 150, func(req int) int {
			if req > 1 {
				return http.StatusBadRequest
			}
			return 0
		})
		got, err := collect(t, newTestClient(srv.URL))
		if err == nil || len(got) != 100 {
			t.Fatalf("got %d products, err = %v; want 100 then an error", len(got), err)
		}
	})
}

func TestProductsStopsWhenTheConsumerDoes(t *testing.T) {
	srv, reqs := pagedServer(t, 250, nil)
	for p, err := range newTestClient(srv.URL).Products(context.Background()) {
		if err != nil {
			t.Fatal(err)
		}
		if p.SKU == "SKU-005" {
			break
		}
	}
	if reqs.Load() != 1 {
		t.Errorf("%d requests, want 1: the next page must not be fetched after break", reqs.Load())
	}
}

func TestProductsCancelledDuringBackoff(t *testing.T) {
	srv, _ := pagedServer(t, 10, func(int) int { return http.StatusServiceUnavailable })
	c := NewMockClient(srv.URL, nil)
	c.backoff = time.Hour

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	start := time.Now()
	var gotErr error
	for _, err := range c.Products(ctx) {
		gotErr = err
	}
	if !errors.Is(gotErr, context.DeadlineExceeded) {
		t.Errorf("err = %v, want DeadlineExceeded", gotErr)
	}
	if time.Since(start) > 5*time.Second {
		t.Error("waited out the backoff instead of returning on cancel")
	}
}
