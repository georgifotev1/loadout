package main

import (
	"net/http"
	"net/url"
	"strconv"

	"github.com/gfotev/loadout/internal/catalog"
	"github.com/gfotev/loadout/internal/money"
	"github.com/gfotev/loadout/internal/platform/validate"
)

const defaultPageSize = 24

// eur is how every price leaves the API: euro cents (VAT included) and the currency, so clients
// never guess. The web app formats it; it never does price math.
func eur(c money.Cents) Price {
	return Price{AmountCents: int64(c), Currency: PriceCurrencyEUR}
}

// The response types (ProductSummary, ProductDetail, ...) come from api/openapi.yaml and are
// separate from the catalog types on purpose: the dealer price, markup rule and hashes never
// leave the server.

// ListProducts handles GET /api/products.
func (app *application) ListProducts(w http.ResponseWriter, r *http.Request, params ListProductsParams) {
	page := valueOr(params.Page, 1)
	pageSize := valueOr(params.PageSize, defaultPageSize)

	res, err := app.catalog.List(r.Context(), catalog.ListFilter{
		Category: string(params.Category),
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		app.handleError(w, r, err)
		return
	}

	resp := ProductList{
		Products: make([]ProductSummary, 0, len(res.Products)),
		Page:     page,
		PageSize: pageSize,
		Total:    res.Total,
	}
	for _, p := range res.Products {
		resp.Products = append(resp.Products, summary(p))
	}
	app.writeJSON(w, r, http.StatusOK, resp)
}

// SearchProducts handles GET /api/search.
func (app *application) SearchProducts(w http.ResponseWriter, r *http.Request, params SearchProductsParams) {
	page := valueOr(params.Page, 1)
	pageSize := valueOr(params.PageSize, defaultPageSize)

	res, err := app.catalog.Search(r.Context(), catalog.SearchQuery{
		Text:     params.Q,
		Category: string(params.Category),
		Brands:   params.Brand,
		MinCents: money.Cents(params.PriceMin),
		MaxCents: money.Cents(params.PriceMax),
		Sort:     catalog.Sort(params.Sort),
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		app.handleError(w, r, err)
		return
	}

	resp := SearchResult{
		Products: make([]ProductSummary, 0, len(res.Products)),
		Page:     page,
		PageSize: pageSize,
		Total:    res.Total,
		Facets: Facets{
			Categories: make([]CategoryCount, 0, len(res.Facets.Categories)),
			Brands:     make([]ValueCount, 0, len(res.Facets.Brands)),
			Prices:     make([]PriceBucket, 0, len(res.Facets.Prices)),
		},
	}
	for _, p := range res.Products {
		resp.Products = append(resp.Products, summary(p))
	}
	for _, c := range res.Facets.Categories {
		resp.Facets.Categories = append(resp.Facets.Categories, CategoryCount{Value: Category(c.Value), Count: c.Count})
	}
	for _, b := range res.Facets.Brands {
		resp.Facets.Brands = append(resp.Facets.Brands, ValueCount{Value: b.Value, Count: b.Count})
	}
	for _, b := range res.Facets.Prices {
		resp.Facets.Prices = append(resp.Facets.Prices, PriceBucket{
			MinCents: int64(b.MinCents),
			MaxCents: int64(b.MaxCents),
			Count:    b.Count,
		})
	}
	app.writeJSON(w, r, http.StatusOK, resp)
}

func summary(p catalog.ProductSummary) ProductSummary {
	return ProductSummary{
		Slug:     p.Slug,
		Name:     p.Name,
		Brand:    p.Brand,
		Category: Category(p.Category),
		Image:    p.Image,
		Price:    eur(p.RetailCents),
		InStock:  p.StockQty > 0,
	}
}

// GetProduct handles GET /api/products/{slug}.
func (app *application) GetProduct(w http.ResponseWriter, r *http.Request, slug string) {
	p, err := app.catalog.Product(r.Context(), slug)
	if err != nil {
		app.handleError(w, r, err)
		return
	}
	app.writeJSON(w, r, http.StatusOK, ProductDetail{
		Slug:       p.Slug,
		Sku:        p.ProviderSKU,
		Name:       p.Name,
		Brand:      p.Brand,
		Category:   Category(p.Category),
		Images:     p.Images,
		Attributes: p.Attributes,
		Specs:      p.Specs,
		Price:      eur(p.RetailCents),
		InStock:    p.StockQty > 0,
	})
}

// valueOr returns *p, or def when the optional parameter p is absent.
func valueOr[T any](p *T, def T) T {
	if p == nil {
		return def
	}
	return *p
}

// queryInt reads an optional integer query parameter. A value that isn't an integer is recorded
// as an "invalid_integer" error on v; range checks are the service's job.
// TODO(phase-6): drop once the admin endpoints move onto api/openapi.yaml.
func queryInt(v *validate.Validator, q url.Values, key string, def int) int {
	s := q.Get(key)
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		v.Add(key, "invalid_integer")
		return def
	}
	return n
}
