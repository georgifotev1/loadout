package main

import (
	"net/http"
	"net/url"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/gfotev/loadout/internal/catalog"
	"github.com/gfotev/loadout/internal/money"
	"github.com/gfotev/loadout/internal/platform/validate"
)

const defaultPageSize = 24

// priceJSON is how every price leaves the API: euro cents (VAT included) and the currency, so
// clients never guess. The web app formats it; it never does price math.
type priceJSON struct {
	AmountCents money.Cents `json:"amount_cents"`
	Currency    string      `json:"currency"`
}

func eur(c money.Cents) priceJSON {
	return priceJSON{AmountCents: c, Currency: money.Currency}
}

// The JSON types below are what shoppers see. They are separate from the catalog types on
// purpose: the dealer price, markup rule, hashes and Vali's source URL never leave the server.

type productSummaryJSON struct {
	Slug     string    `json:"slug"`
	Name     string    `json:"name"`
	Brand    string    `json:"brand"`
	Category string    `json:"category"`
	Image    string    `json:"image,omitempty"`
	Price    priceJSON `json:"price"`
	InStock  bool      `json:"in_stock"`
}

type productListJSON struct {
	Products []productSummaryJSON `json:"products"`
	Page     int                  `json:"page"`
	PageSize int                  `json:"page_size"`
	Total    int                  `json:"total"`
}

type productDetailJSON struct {
	Slug           string            `json:"slug"`
	Name           string            `json:"name"`
	Brand          string            `json:"brand"`
	Category       string            `json:"category"`
	MPN            string            `json:"mpn,omitempty"`
	EAN            string            `json:"ean,omitempty"`
	Images         []string          `json:"images"`
	RawSpecs       map[string]string `json:"raw_specs"` // TODO(phase-2): canonical specs next to these
	WarrantyMonths *int              `json:"warranty_months"`
	WarrantyText   string            `json:"warranty_text,omitempty"`
	Price          priceJSON         `json:"price"`
	InStock        bool              `json:"in_stock"`
}

// GET /api/products?category=&page=&page_size=
func (app *application) listProducts(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	var v validate.Validator
	page := queryInt(&v, q, "page", 1)
	pageSize := queryInt(&v, q, "page_size", defaultPageSize)
	if err := v.Err(); err != nil {
		app.handleError(w, r, err)
		return
	}

	res, err := app.catalog.List(r.Context(), catalog.ListFilter{
		Category: q.Get("category"),
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		app.handleError(w, r, err)
		return
	}

	resp := productListJSON{
		Products: make([]productSummaryJSON, 0, len(res.Products)),
		Page:     page,
		PageSize: pageSize,
		Total:    res.Total,
	}
	for _, p := range res.Products {
		resp.Products = append(resp.Products, productSummaryJSON{
			Slug:     p.Slug,
			Name:     p.Name,
			Brand:    p.Brand,
			Category: p.Category,
			Image:    p.Image,
			Price:    eur(p.RetailCents),
			InStock:  p.StockQty > 0,
		})
	}
	app.writeJSON(w, r, http.StatusOK, resp)
}

// GET /api/products/{slug}
func (app *application) showProduct(w http.ResponseWriter, r *http.Request) {
	p, err := app.catalog.Product(r.Context(), chi.URLParam(r, "slug"))
	if err != nil {
		app.handleError(w, r, err)
		return
	}
	app.writeJSON(w, r, http.StatusOK, productDetailJSON{
		Slug:           p.Slug,
		Name:           p.Name,
		Brand:          p.Brand,
		Category:       p.Category,
		MPN:            p.MPN,
		EAN:            p.EAN,
		Images:         p.Images,
		RawSpecs:       p.RawSpecs,
		WarrantyMonths: p.WarrantyMonths,
		WarrantyText:   p.WarrantyText,
		Price:          eur(p.RetailCents),
		InStock:        p.StockQty > 0,
	})
}

// queryInt reads an optional integer query parameter. A value that isn't an integer is recorded
// as an "invalid_integer" error on v; range checks are the service's job.
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
