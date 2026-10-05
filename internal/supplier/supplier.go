// Package supplier is the port between the catalog and the companies we buy from. The catalog
// only ever sees a Product; each supplier's adapter (internal/vali) turns its own format into
// one. When a supplier's real API arrives, only its adapter changes.
//
// Product holds only what any supplier must give us to sell something. Add a field when a real
// supplier provides it and a feature needs it, not before.
package supplier

import (
	"context"
	"fmt"
	"iter"

	"github.com/gfotev/loadout/internal/money"
)

// Source is one supplier's product feed.
type Source interface {
	// Name identifies the supplier ("vali"). It's stored with every product, so it must not
	// change.
	Name() string
	// Products yields the supplier's whole current catalog. An *ItemError is about one product
	// and the sequence carries on after it; any other error ends the sequence. A sequence that
	// ends without an error is the complete list, which the catalog relies on to notice products
	// that are gone.
	Products(ctx context.Context) iter.Seq2[Product, error]
}

// Product is one product as the catalog needs it, whatever the supplier's format.
type Product struct {
	SKU       string // the supplier's id for it; unique per supplier
	Name      string
	Brand     string
	Category  string      // canonical catalog category; the adapter maps the supplier's own
	CostCents money.Cents // what we pay, VAT included
	StockQty  int         // available to order from the supplier
	Images    []string    // URLs

	// Attributes are the product's spec rows as the supplier shows them, for display.
	Attributes map[string]string
	// Specs are canonical spec values (field name → value, see internal/specs) the adapter could
	// read from the supplier's data. Whatever it can't read stays out; specs reports it missing.
	Specs map[string]any
}

// ItemError is a supplier product the adapter couldn't turn into a Product (an unknown
// category, a price in another currency, ...).
type ItemError struct {
	SKU string
	Err error
}

func (e *ItemError) Error() string { return fmt.Sprintf("supplier product %s: %v", e.SKU, e.Err) }
func (e *ItemError) Unwrap() error { return e.Err }
