// Package vali is the port to Vali Computers, our supplier. Catalog sync talks to a Client and
// never to Vali's API directly, so the real client can replace MockClient (HTTP to mock-vali)
// once the dealership is signed, without touching the sync.
package vali

import (
	"context"
	"iter"
	"time"
)

// Client lists Vali's products.
type Client interface {
	// ListProducts yields every product changed at or after since (zero = all of them). Paging
	// happens under the hood. After an error is yielded the sequence ends; a product sequence
	// that ends without an error is the complete list.
	ListProducts(ctx context.Context, since time.Time) iter.Seq2[RawProduct, error]
}

// RawProduct is one product in Vali's raw format (fixtures/vali/product.schema.json): Bulgarian
// spec keys, values as shown on vali.bg, nothing normalized. Mapping it onto our catalog is the
// catalog's job; this package only moves it over the wire.
// ASSUMPTION: the real dealer API returns roughly these fields; its real shape is unknown.
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
	Currency         string            `json:"currency"`           // always "EUR"; anything else is rejected by the sync
	Stock            map[string]int    `json:"stock"`              // warehouse → quantity
	UpdatedAt        time.Time         `json:"updated_at"`
}
