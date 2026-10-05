// Package vali is the adapter for Vali Computers, our supplier. It is the only package that
// knows Vali's data format: it reads Vali's products and turns them into supplier.Products.
//
// Vali's dealer API is unknown until the dealership is signed. Until then MockClient talks to
// cmd/mock-vali, which serves fixtures copied from vali.bg's public pages, and everything about
// the format is an ASSUMPTION. When the real API arrives, a real client replaces MockClient and
// toProduct changes; nothing outside this package should have to. Work that waits for the real
// API is marked TODO(vali-api).
package vali

import (
	"errors"
	"fmt"
	"time"

	"github.com/gfotev/loadout/internal/money"
	"github.com/gfotev/loadout/internal/supplier"
)

// Name is how Vali products are identified in the catalog.
const Name = "vali"

// RawProduct is one product in mock-vali's format (fixtures/vali/product.schema.json): what
// vali.bg's public pages show, plus invented dealer fields.
// ASSUMPTION: the real dealer API returns something like this; its real shape is unknown.
type RawProduct struct {
	SKU              string            `json:"sku"`
	ManufacturerCode string            `json:"manufacturer_code,omitempty"`
	EAN              string            `json:"ean,omitempty"`
	Brand            string            `json:"brand"`
	Name             string            `json:"name"`
	CategoryPath     []string          `json:"category_path"`
	URL              string            `json:"url"`
	Warranty         string            `json:"warranty,omitempty"`
	Images           []string          `json:"images"`
	Specs            map[string]string `json:"specs"`
	DealerPriceCents int64             `json:"dealer_price_cents"` // VAT included
	RRPCents         int64             `json:"rrp_cents"`          // VAT included
	Currency         string            `json:"currency"`
	Stock            map[string]int    `json:"stock"` // warehouse → quantity
	UpdatedAt        time.Time         `json:"updated_at"`
}

var (
	errUnknownCategory = errors.New("unknown category")
	errCurrency        = errors.New("not priced in EUR")
)

// toProduct maps a Vali product onto the catalog's supplier.Product. Only Vali-specific checks
// happen here; the catalog validates what applies to every supplier (a name, a price, ...).
func toProduct(raw RawProduct) (supplier.Product, error) {
	category, ok := categoryOf(raw.CategoryPath)
	if !ok {
		return supplier.Product{}, fmt.Errorf("%w %q", errUnknownCategory, raw.CategoryPath)
	}
	if raw.Currency != money.Currency {
		return supplier.Product{}, fmt.Errorf("%w: %q", errCurrency, raw.Currency)
	}
	// ASSUMPTION: we can order from any of Vali's warehouses, so the total is what's available.
	stock := 0
	for _, n := range raw.Stock {
		stock += n
	}
	return supplier.Product{
		SKU:        raw.SKU,
		Name:       raw.Name,
		Brand:      raw.Brand,
		Category:   category,
		CostCents:  money.Cents(raw.DealerPriceCents),
		StockQty:   stock,
		Images:     raw.Images,
		Attributes: raw.Specs,
		// TODO(vali-api): fill Specs (canonical spec values) from Vali's attributes once their real
		// format is known. A parser for the vali.bg page format is archived in plan/archive/.
	}, nil
}

// categories maps the second level of Vali's category tree to the catalog's canonical
// categories. The third level is just "<second level> <brand>", so it adds nothing.
// ASSUMPTION: these are the names vali.bg shows; the dealer API may use ids instead.
var categories = map[string]string{
	"Процесори":                       "cpu",
	"Дънни платки":                    "motherboard",
	"Памети":                          "ram",
	"Видео карти":                     "gpu",
	"Захранвания":                     "psu",
	"Кутии за компютри":               "case",
	"Охладители за процесори":         "cooler",
	"Solid State Drive (SSD) дискове": "storage",
	"Геймърски столове":               "peripheral.chair",
	"Геймърски слушалки":              "peripheral.headset",
	"Геймърски клавиатури":            "peripheral.keyboard",
	"Монитори и публични дисплеи":     "peripheral.monitor",
	"Геймърски мишки":                 "peripheral.mouse",
	"Геймърски падове":                "peripheral.mousepad",
}

// categoryOf returns the canonical category for a Vali category path. ok is false for categories
// we don't sell (yet); those products become item errors, so a new Vali category shows up in the
// sync run instead of being silently dropped.
func categoryOf(path []string) (category string, ok bool) {
	if len(path) < 2 {
		return "", false
	}
	category, ok = categories[path[1]]
	return category, ok
}
