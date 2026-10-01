package main

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gfotev/loadout/fixtures"
)

const testSKU = "AMD-AM4-R3-RYZ-4100-BOX"

// fixtureTime is the updated_at of every fixture; mutations happen at mutateTime.
var (
	fixtureTime = time.Date(2026, 9, 30, 8, 0, 0, 0, time.UTC)
	mutateTime  = time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
)

func newTestServer(t *testing.T) (*server, *httptest.Server) {
	t.Helper()
	st, err := newStore(fixtures.Vali, func() time.Time { return mutateTime })
	if err != nil {
		t.Fatal(err)
	}
	s := &server{
		store:  st,
		logger: slog.New(slog.DiscardHandler),
		rand:   func() float64 { return 0.5 },
	}
	ts := httptest.NewServer(s.routes())
	t.Cleanup(ts.Close)
	return s, ts
}

func do(t *testing.T, method, url, body string) (int, []byte) {
	t.Helper()
	req, err := http.NewRequest(method, url, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode, b
}

func getPage(t *testing.T, url string) productPage {
	t.Helper()
	status, b := do(t, http.MethodGet, url, "")
	if status != http.StatusOK {
		t.Fatalf("GET %s = %d %s", url, status, b)
	}
	var p productPage
	if err := json.Unmarshal(b, &p); err != nil {
		t.Fatal(err)
	}
	return p
}

func mutate(t *testing.T, ts *httptest.Server, body string) {
	t.Helper()
	if status, b := do(t, http.MethodPost, ts.URL+"/admin/mutate", body); status != http.StatusOK {
		t.Fatalf("mutate %s = %d %s", body, status, b)
	}
}

func TestListWalksEveryProductOnce(t *testing.T) {
	_, ts := newTestServer(t)
	files, err := fixturesCount()
	if err != nil {
		t.Fatal(err)
	}

	seen := map[string]bool{}
	var last string
	url := ts.URL + "/v1/products?page_size=7"
	for page := 1; ; page++ {
		p := getPage(t, url+"&page="+strconv.Itoa(page))
		if p.Total != files {
			t.Fatalf("total = %d, want %d", p.Total, files)
		}
		for _, prod := range p.Products {
			if seen[prod.SKU] {
				t.Errorf("sku %s returned twice", prod.SKU)
			}
			if prod.SKU <= last {
				t.Errorf("sku %s after %s, want sorted", prod.SKU, last)
			}
			seen[prod.SKU], last = true, prod.SKU
		}
		if p.NextPage == nil {
			break
		}
		if *p.NextPage != page+1 {
			t.Fatalf("next_page = %d on page %d", *p.NextPage, page)
		}
	}
	if len(seen) != files {
		t.Errorf("walked %d products, want %d", len(seen), files)
	}
}

func TestListSince(t *testing.T) {
	_, ts := newTestServer(t)

	if p := getPage(t, ts.URL+"/v1/products?since="+fixtureTime.Format(time.RFC3339)); p.Total == 0 {
		t.Error("since = fixture time should include the fixtures (inclusive)")
	}
	after := fixtureTime.Add(time.Second).Format(time.RFC3339)
	if p := getPage(t, ts.URL+"/v1/products?since="+after); p.Total != 0 {
		t.Fatalf("since after fixtures: total = %d, want 0", p.Total)
	}

	mutate(t, ts, `{"sku":"`+testSKU+`","rrp_cents":5999}`)
	p := getPage(t, ts.URL+"/v1/products?since="+after)
	if p.Total != 1 || p.Products[0].SKU != testSKU {
		t.Fatalf("after one mutation: %+v", p)
	}
	if got := p.Products[0]; got.RRPCents != 5999 || !got.UpdatedAt.Equal(mutateTime) {
		t.Errorf("mutated product: rrp %d, updated_at %s", got.RRPCents, got.UpdatedAt)
	}
}

func TestGetProduct(t *testing.T) {
	_, ts := newTestServer(t)

	status, b := do(t, http.MethodGet, ts.URL+"/v1/products/"+testSKU, "")
	if status != http.StatusOK {
		t.Fatalf("status = %d", status)
	}
	var p product
	if err := json.Unmarshal(b, &p); err != nil {
		t.Fatal(err)
	}
	if p.SKU != testSKU || p.Specs["Сокет"] != "AM4" || p.Currency != "EUR" {
		t.Errorf("got %+v", p)
	}
	if strings.Contains(string(b), "_mock") {
		t.Error("the fixture-only _mock note leaked into the API")
	}

	if status, _ := do(t, http.MethodGet, ts.URL+"/v1/products/NOPE", ""); status != http.StatusNotFound {
		t.Errorf("unknown sku: status = %d, want 404", status)
	}
}

func TestMutateSpecsAndStock(t *testing.T) {
	_, ts := newTestServer(t)
	mutate(t, ts, `{"sku":"`+testSKU+`","stock":{"Plovdiv":0},"specs":{"Тегло":null,"Нов ключ":"x"}}`)

	_, b := do(t, http.MethodGet, ts.URL+"/v1/products/"+testSKU, "")
	var p product
	if err := json.Unmarshal(b, &p); err != nil {
		t.Fatal(err)
	}
	if p.Stock["Plovdiv"] != 0 || p.Stock["Sofia-Central"] == 0 {
		t.Errorf("stock = %v, want Plovdiv set to 0 and Sofia-Central kept", p.Stock)
	}
	if _, ok := p.Specs["Тегло"]; ok || p.Specs["Нов ключ"] != "x" {
		t.Errorf("specs = %v, want Тегло deleted and Нов ключ added", p.Specs)
	}
}

func TestMutateRejectsBadInput(t *testing.T) {
	_, ts := newTestServer(t)
	tests := []struct {
		body string
		want int
	}{
		{`{"sku":"NOPE","rrp_cents":1}`, http.StatusNotFound},
		{`{"sku":"` + testSKU + `"}`, http.StatusUnprocessableEntity},
		{`{"rrp_cents":1}`, http.StatusUnprocessableEntity},
		{`{"sku":"` + testSKU + `","rrp_cents":0}`, http.StatusUnprocessableEntity},
		{`{"sku":"` + testSKU + `","stock":{"Plovdiv":-1}}`, http.StatusUnprocessableEntity},
		{`{"sku":"` + testSKU + `","price":1}`, http.StatusBadRequest},
		{`not json`, http.StatusBadRequest},
	}
	for _, tt := range tests {
		if status, b := do(t, http.MethodPost, ts.URL+"/admin/mutate", tt.body); status != tt.want {
			t.Errorf("%s: status = %d, want %d (%s)", tt.body, status, tt.want, b)
		}
	}
}

func TestDelistAndReset(t *testing.T) {
	_, ts := newTestServer(t)
	before := getPage(t, ts.URL+"/v1/products").Total

	mutate(t, ts, `{"sku":"`+testSKU+`","delisted":true}`)
	if got := getPage(t, ts.URL+"/v1/products").Total; got != before-1 {
		t.Errorf("after delist: total = %d, want %d", got, before-1)
	}
	if status, _ := do(t, http.MethodGet, ts.URL+"/v1/products/"+testSKU, ""); status != http.StatusNotFound {
		t.Errorf("delisted get: status = %d, want 404", status)
	}

	if status, _ := do(t, http.MethodPost, ts.URL+"/admin/reset", ""); status != http.StatusNoContent {
		t.Fatalf("reset: status = %d", status)
	}
	if got := getPage(t, ts.URL+"/v1/products").Total; got != before {
		t.Errorf("after reset: total = %d, want %d", got, before)
	}
	after := fixtureTime.Add(time.Second).Format(time.RFC3339)
	if got := getPage(t, ts.URL+"/v1/products?since="+after).Total; got != 0 {
		t.Errorf("after reset: %d changed products, want 0", got)
	}
}

func TestChaosKnobs(t *testing.T) {
	s, ts := newTestServer(t) // rand always returns 0.5

	tests := []struct {
		query string
		want  int
	}{
		{"?fail_rate=1", http.StatusServiceUnavailable},
		{"?fail_rate=0.6", http.StatusServiceUnavailable},
		{"?fail_rate=0.4", http.StatusOK},
		{"?fail_rate=2", http.StatusBadRequest},
		{"?latency_ms=-1", http.StatusBadRequest},
		{"?page=0", http.StatusBadRequest},
		{"?page_size=101", http.StatusBadRequest},
		{"?since=yesterday", http.StatusBadRequest},
	}
	for _, tt := range tests {
		if status, b := do(t, http.MethodGet, ts.URL+"/v1/products"+tt.query, ""); status != tt.want {
			t.Errorf("%s: status = %d, want %d (%s)", tt.query, status, tt.want, b)
		}
	}

	start := time.Now()
	getPage(t, ts.URL+"/v1/products?latency_ms=50")
	if d := time.Since(start); d < 50*time.Millisecond {
		t.Errorf("latency_ms=50 took %s", d)
	}

	// The -fail-rate flag applies to /v1 but not to the admin endpoints.
	s.failRate = 1
	if status, _ := do(t, http.MethodGet, ts.URL+"/v1/products/"+testSKU, ""); status != http.StatusServiceUnavailable {
		t.Errorf("default fail rate: status = %d, want 503", status)
	}
	mutate(t, ts, `{"sku":"`+testSKU+`","rrp_cents":1234}`)
}

func fixturesCount() (int, error) {
	st, err := newStore(fixtures.Vali, time.Now)
	if err != nil {
		return 0, err
	}
	return len(st.skus), nil
}

// Run with -race: encoding responses while mutations swap products in must not race.
func TestConcurrentReadsAndMutations(t *testing.T) {
	_, ts := newTestServer(t)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := range 50 {
			// Not mutate(): t.Fatal must not be called from a goroutine other than the test's.
			body := `{"sku":"` + testSKU + `","stock":{"Plovdiv":` + strconv.Itoa(i) + `},"specs":{"k":"v"}}`
			resp, err := http.Post(ts.URL+"/admin/mutate", "application/json", strings.NewReader(body))
			if err != nil {
				t.Errorf("mutate: %v", err)
				return
			}
			resp.Body.Close()
		}
	}()
	for range 50 {
		getPage(t, ts.URL+"/v1/products?page_size=100")
	}
	<-done
}
