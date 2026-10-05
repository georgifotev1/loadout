package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
	app.specs = specs.NewService(pool)
	app.catalog = catalog.NewService(pool, &supplier.Fake{Items: products}, app.specs, app.logger)
	if _, err := app.catalog.Sync(context.Background()); err != nil {
		t.Fatal(err)
	}
	return app
}

func get(t *testing.T, app *application, target string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	app.routes().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
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
		if p.Brand != "AMD" || p.Category != "cpu" || p.Attributes["Сокет"] != "AM4" || !p.InStock ||
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

// Every catalog category must be in the Category enum in api/openapi.yaml, or the web app's
// generated types won't know it.
func TestCategoryEnumCoversCatalog(t *testing.T) {
	for _, c := range catalog.Categories {
		if !Category(c).Valid() {
			t.Errorf("catalog category %q is missing from the Category enum in api/openapi.yaml", c)
		}
	}
}
