package vali

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"iter"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gfotev/loadout/internal/supplier"
)

const (
	pageSize     = 100      // mock-vali's maximum
	maxBodyBytes = 32 << 20 // a page of 100 products is ~200 KB; this is just a backstop
	attempts     = 3        // per page
	firstBackoff = 250 * time.Millisecond
)

// MockClient is the supplier.Source for Vali until the real dealer API is known: it talks to
// cmd/mock-vali over HTTP. Everything it assumes about the wire format is marked ASSUMPTION in
// mock-vali.
type MockClient struct {
	baseURL string
	http    *http.Client
	backoff time.Duration // before the second attempt; doubles after that
}

// NewMockClient returns a client for the mock-vali at baseURL (e.g. "http://localhost:8081").
// A nil hc gets a client with a 30s timeout.
func NewMockClient(baseURL string, hc *http.Client) *MockClient {
	if hc == nil {
		hc = &http.Client{Timeout: 30 * time.Second}
	}
	return &MockClient{baseURL: strings.TrimRight(baseURL, "/"), http: hc, backoff: firstBackoff}
}

// page is one page of GET /v1/products.
type page struct {
	Products []RawProduct `json:"products"`
	NextPage *int         `json:"next_page"` // null on the last page
}

var _ supplier.Source = (*MockClient)(nil)

func (c *MockClient) Name() string { return Name }

// Products pages through the whole catalog. A product that can't be mapped is yielded as a
// *supplier.ItemError and the listing carries on.
func (c *MockClient) Products(ctx context.Context) iter.Seq2[supplier.Product, error] {
	return func(yield func(supplier.Product, error) bool) {
		next := 1
		for {
			pg, err := c.fetchPage(ctx, next)
			if err != nil {
				yield(supplier.Product{}, err)
				return
			}
			for _, raw := range pg.Products {
				p, err := toProduct(raw)
				if err != nil {
					err = &supplier.ItemError{SKU: raw.SKU, Err: err}
				}
				if !yield(p, err) {
					return
				}
			}
			if pg.NextPage == nil {
				return
			}
			next = *pg.NextPage
		}
	}
}

// fetchPage gets one page, retrying network errors and 5xx/429 responses with backoff.
func (c *MockClient) fetchPage(ctx context.Context, n int) (page, error) {
	q := url.Values{}
	q.Set("page", strconv.Itoa(n))
	q.Set("page_size", strconv.Itoa(pageSize))
	u := c.baseURL + "/v1/products?" + q.Encode()

	backoff := c.backoff
	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		if attempt > 1 {
			t := time.NewTimer(backoff)
			select {
			case <-ctx.Done():
				t.Stop()
				return page{}, fmt.Errorf("vali: list products page %d: %w", n, ctx.Err())
			case <-t.C:
			}
			backoff *= 2
		}

		pg, retry, err := c.get(ctx, u)
		if err == nil {
			return pg, nil
		}
		lastErr = fmt.Errorf("vali: list products page %d: %w", n, err)
		if !retry || ctx.Err() != nil {
			return page{}, lastErr
		}
	}
	return page{}, fmt.Errorf("%w (after %d attempts)", lastErr, attempts)
}

// get does one request. retry says whether trying again could help.
func (c *MockClient) get(ctx context.Context, u string) (pg page, retry bool, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return page{}, false, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return page{}, true, err
	}
	defer resp.Body.Close()

	body := io.LimitReader(resp.Body, maxBodyBytes)
	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, body) // let the connection be reused
		retry := resp.StatusCode >= 500 || resp.StatusCode == http.StatusTooManyRequests
		return page{}, retry, fmt.Errorf("unexpected status %s", resp.Status)
	}
	if err := json.NewDecoder(body).Decode(&pg); err != nil {
		var netErr interface{ Timeout() bool }
		// A body cut off mid-way is worth retrying; malformed JSON is not.
		retry := errors.Is(err, io.ErrUnexpectedEOF) || (errors.As(err, &netErr) && netErr.Timeout())
		return page{}, retry, fmt.Errorf("decode response: %w", err)
	}
	return pg, false, nil
}
