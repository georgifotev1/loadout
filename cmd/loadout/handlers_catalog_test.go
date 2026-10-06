package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/gfotev/loadout/fixtures"
	"github.com/gfotev/loadout/internal/catalog"
	"github.com/gfotev/loadout/internal/platform/db/dbtest"
	"github.com/gfotev/loadout/internal/specs"
	"github.com/gfotev/loadout/internal/supplier"
	"github.com/gfotev/loadout/internal/vali"
)

// newCatalogApp returns an app over a fresh database holding every fixture product.
func newCatalogApp(t *testing.T) *application {
	t.Helper()
	pool := dbtest.New(t)
	products, err := vali.FixtureProducts(fixtures.Vali)
	if err != nil {
		t.Fatal(err)
	}
	app := newTestApp()
	app.db = pool
	app.auth = newAuthService(t, pool)
	app.specs = specs.NewService(pool)
	app.catalog = catalog.NewService(pool, &supplier.Fake{Provider: "vali", Items: products}, app.specs, app.logger)
	if _, err := app.catalog.Sync(context.Background()); err != nil {
		t.Fatal(err)
	}
	return app
}

func get(t *testing.T, app *application, target string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	app.routes().ServeHTTP(rec, req)
	return rec
}

func TestProductsAPI(t *testing.T) {
	app := newCatalogApp(t)

	t.Run("list by category", func(t *testing.T) {
		rec := get(t, app, "/api/products?category=cpu&page_size=5")
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d: %s", rec.Code, rec.Body)
		}
		var body ProductList
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if body.Total != 7 || len(body.Products) != 5 || body.Page != 1 || body.PageSize != 5 {
			t.Fatalf("got total=%d len=%d page=%d size=%d", body.Total, len(body.Products), body.Page, body.PageSize)
		}
		for _, p := range body.Products {
			if p.Category != "cpu" || p.Price.Currency != "EUR" || p.Price.AmountCents <= 0 || p.Slug == "" {
				t.Errorf("product %+v", p)
			}
		}

		rec = get(t, app, "/api/products?category=cpu&page_size=5&page=2")
		_ = json.Unmarshal(rec.Body.Bytes(), &body)
		if len(body.Products) != 2 {
			t.Errorf("page 2 has %d products, want 2", len(body.Products))
		}
	})

	t.Run("list everything by default", func(t *testing.T) {
		var body ProductList
		_ = json.Unmarshal(get(t, app, "/api/products").Body.Bytes(), &body)
		if body.Total != 67 || len(body.Products) != defaultPageSize {
			t.Errorf("total=%d len=%d", body.Total, len(body.Products))
		}
	})

	t.Run("bad query parameters are 422", func(t *testing.T) {
		for _, target := range []string{
			"/api/products?category=spaceship",
			"/api/products?page=0",
			"/api/products?page=abc",
			"/api/products?page_size=101",
		} {
			if rec := get(t, app, target); rec.Code != http.StatusUnprocessableEntity {
				t.Errorf("%s: status %d, want 422", target, rec.Code)
			}
		}
	})

	t.Run("product page", func(t *testing.T) {
		rec := get(t, app, "/api/products/amd-am4-r3-ryz-4100-box")
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d: %s", rec.Code, rec.Body)
		}
		var p ProductDetail
		if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
			t.Fatal(err)
		}
		// Seeded default rule: 52.35 € + 15% = 60.20 €, rounded up to 60.90 €.
		if p.Sku != "AMD-AM4-R3-RYZ-4100-BOX" || p.Brand != "AMD" || p.Category != "cpu" || p.Attributes["Сокет"] != "AM4" || !p.InStock ||
			p.Price != (Price{AmountCents: 6090, Currency: PriceCurrencyEUR}) || p.Specs == nil {
			t.Errorf("got %+v", p)
		}
	})

	t.Run("dealer data never leaves the server", func(t *testing.T) {
		for _, target := range []string{"/api/products", "/api/products/amd-am4-r3-ryz-4100-box"} {
			body := get(t, app, target).Body.String()
			for _, leak := range []string{"dealer", "5235", "rule", "hash", "vali.bg/procesor"} {
				if strings.Contains(body, leak) {
					t.Errorf("%s: response contains %q", target, leak)
				}
			}
		}
	})

	t.Run("unknown product is a JSON 404", func(t *testing.T) {
		rec := get(t, app, "/api/products/nope")
		if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), `"not_found"`) {
			t.Errorf("status %d body %s", rec.Code, rec.Body)
		}
	})
}

func TestSearchAPI(t *testing.T) {
	app := newCatalogApp(t)

	t.Run("filters and facets", func(t *testing.T) {
		rec := get(t, app, "/api/search?q=core&category=cpu&brand=AMD&brand=INTEL&sort=price_asc&page_size=2")
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d: %s", rec.Code, rec.Body)
		}
		var body SearchResult
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if body.Total == 0 || len(body.Products) > 2 || body.PageSize != 2 {
			t.Fatalf("total=%d len=%d size=%d", body.Total, len(body.Products), body.PageSize)
		}
		if len(body.Facets.Brands) == 0 || len(body.Facets.Categories) == 0 || len(body.Facets.Prices) == 0 {
			t.Errorf("facets %+v", body.Facets)
		}
	})

	t.Run("no matches still has facet arrays", func(t *testing.T) {
		rec := get(t, app, "/api/search?q=spaceship")
		if !strings.Contains(rec.Body.String(), `"facets":{"brands":[],"categories":[],"prices":[],"specs":[]}`) {
			t.Errorf("body %s", rec.Body)
		}
	})

	t.Run("bad query parameters are 422", func(t *testing.T) {
		for _, target := range []string{
			"/api/search?category=spaceship",
			"/api/search?sort=random",
			"/api/search?price_min=-1",
			"/api/search?price_min=5000&price_max=100",
			"/api/search?price_max=abc",
			"/api/search?q=" + strings.Repeat("a", 201),
			"/api/search?category=gpu&spec=vram_gb",
			"/api/search?category=gpu&spec=cores:8",
			"/api/search?spec=vram_gb:8",
		} {
			if rec := get(t, app, target); rec.Code != http.StatusUnprocessableEntity {
				t.Errorf("%s: status %d, want 422", target, rec.Code)
			}
		}
	})

	t.Run("dealer data never leaves the server", func(t *testing.T) {
		body := get(t, app, "/api/search?category=cpu").Body.String()
		for _, leak := range []string{"dealer", "5235", "rule", "hash"} {
			if strings.Contains(body, leak) {
				t.Errorf("response contains %q", leak)
			}
		}
	})
}

func TestSearchSpecsAPI(t *testing.T) {
	app := newBuilderApp(t)

	rec := get(t, app, "/api/search?category=gpu&spec=vram_gb:12&spec=vram_gb:16")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	var body SearchResult
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Total != 4 {
		t.Errorf("total=%d, want the 12 and 16 GB cards", body.Total)
	}
	for _, p := range body.Products {
		if vram := p.Specs["vram_gb"]; vram != 12.0 && vram != 16.0 {
			t.Errorf("%s: vram_gb %v", p.Slug, vram)
		}
	}
	want := []SpecFacet{
		{Field: "chipset", Values: []ValueCount{
			{Value: "Arc B580", Count: 1},
			{Value: "GeForce RTX 5070", Count: 1},
			{Value: "Radeon RX 9060 XT", Count: 1},
			{Value: "Radeon RX 9070 GRE", Count: 1},
		}},
		// Ignores its own filter: the 8 GB cards are still counted.
		{Field: "vram_gb", Values: []ValueCount{{Value: "8", Count: 2}, {Value: "12", Count: 3}, {Value: "16", Count: 1}}},
	}
	if !reflect.DeepEqual(body.Facets.Specs, want) {
		t.Errorf("spec facets\n got %+v\nwant %+v", body.Facets.Specs, want)
	}

	// Peripherals have no specs: the key is left out.
	if rec := get(t, app, "/api/search?category=peripheral.mouse"); strings.Contains(rec.Body.String(), `"specs":{`) {
		t.Errorf("a peripheral has specs: %s", rec.Body)
	}
}

// Every catalog category must be in the Category enum in api/openapi.yaml, or the web app's
// generated types won't know it.
func TestCategoryEnumCoversCatalog(t *testing.T) {
	for _, c := range catalog.Categories {
		if !Category(c).Valid() {
			t.Errorf("catalog category %q is missing from the Category enum in api/openapi.yaml", c)
		}
	}
}
