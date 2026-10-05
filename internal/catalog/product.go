// Package catalog owns products, prices, stock, markup rules and the Vali sync. Other modules
// reach this data only through the Service (added with the sync); the store is unexported.
package catalog

import (
	"errors"
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/gfotev/loadout/internal/money"
)

var (
	// ErrNotFound is returned when a product, price, rule or sync run doesn't exist (or isn't
	// visible to the caller).
	ErrNotFound = errors.New("catalog: not found")

	// ErrSyncRunNotRunning is returned when finishing a sync run that doesn't exist or has
	// already finished.
	ErrSyncRunNotRunning = errors.New("catalog: sync run not found or already finished")
)

// Product is a catalog product as we store it: Vali's content plus our own flags. Prices and
// stock live in their own tables because they change far more often than the content.
type Product struct {
	ID             uuid.UUID
	Provider       Provider
	ProviderSKU    string // unique only within Provider
	EAN            string
	MPN            string
	Brand          string
	Name           string
	Slug           string
	Category       string
	URL            string
	RawSpecs       map[string]string
	Images         []string
	WarrantyMonths *int // nil = lifetime or unknown
	WarrantyText   string
	RawHash        string
	CommercialHash string
	MissingRuns    int
	Active         bool
	Featured       bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// Categories are the canonical product categories. Suppliers' own category trees are mapped
// onto these by their adapters.
var Categories = []string{
	"cpu",
	"motherboard",
	"ram",
	"gpu",
	"psu",
	"case",
	"cooler",
	"storage",
	"peripheral.chair",
	"peripheral.headset",
	"peripheral.keyboard",
	"peripheral.monitor",
	"peripheral.mouse",
	"peripheral.mousepad",
}

// IsCategory reports whether c is a canonical category.
func IsCategory(c string) bool {
	return slices.Contains(Categories, c)
}

// Provider is the supplier a product comes from.
type Provider string

const ProviderVali Provider = "vali"

// Price is a product's current price. All amounts are euro cents and include VAT.
type Price struct {
	ProductID   uuid.UUID
	DealerCents money.Cents
	RRPCents    money.Cents
	RetailCents money.Cents
	VATbp       int    // 2000 = 20%
	RuleID      *int64 // the markup rule that produced RetailCents; nil if it was deleted
	ComputedAt  time.Time
}

// RuleScope says what a markup rule matches on. More specific scopes win.
type RuleScope string

const (
	ScopeDefault       RuleScope = "default"
	ScopeCategory      RuleScope = "category"
	ScopeBrandCategory RuleScope = "brand_category"
	ScopeSKU           RuleScope = "sku"
)

// Rounding is how a retail price is rounded: to a .99 or .90 ending, or not at all.
type Rounding string

const (
	RoundNone Rounding = "none"
	Round99   Rounding = "99"
	Round90   Rounding = "90"
)

// MarkupRule turns a dealer price into a retail price. Only the fields its Scope matches on are
// set; the database rejects any other combination.
type MarkupRule struct {
	ID             int64
	Scope          RuleScope
	Category       string // set for ScopeCategory and ScopeBrandCategory
	Brand          string // set for ScopeBrandCategory
	SKU            string // set for ScopeSKU
	MarkupBP       int    // 2500 = +25%
	MinMarginCents money.Cents
	Rounding       Rounding
	Priority       int
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// SyncStatus is the state of a sync run.
type SyncStatus string

const (
	SyncRunning   SyncStatus = "running"
	SyncSucceeded SyncStatus = "succeeded"
	SyncFailed    SyncStatus = "failed"
)

// SyncRun records one Vali sync.
type SyncRun struct {
	ID           int64
	StartedAt    time.Time
	FinishedAt   *time.Time // nil while running
	Fetched      int
	Changed      int
	Errors       int
	Status       SyncStatus
	ErrorMessage string
}

// ProductSummary is a product as the storefront lists it.
type ProductSummary struct {
	Slug        string
	Name        string
	Brand       string
	Category    string
	Image       string // the first image, "" if there is none
	RetailCents money.Cents
	StockQty    int // total over all warehouses
}

// ProductDetail is everything the product page shows. It has the retail price only: the dealer
// price and markup are for admins, so shopper-facing code can't leak them by accident.
type ProductDetail struct {
	Product
	RetailCents money.Cents
	StockQty    int // total over all warehouses
}

// ProductPage is one page of a product listing.
type ProductPage struct {
	Products []ProductSummary
	Total    int // matching products over all pages
}

// ListFilter is the storefront's product query.
type ListFilter struct {
	Category string // "" = all categories
	Page     int    // 1-based
	PageSize int
}

// MaxPageSize caps ListFilter.PageSize.
const MaxPageSize = 100
