// Package catalog owns products, prices, stock, markup rules and the Vali sync. Other modules
// reach this data only through the Service (added with the sync); the store is unexported.
package catalog

import (
	"errors"
	"time"

	"github.com/google/uuid"
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

// Provider is the supplier a product comes from.
type Provider string

const ProviderVali Provider = "vali"

// Price is a product's current price. All amounts are integer cents and include VAT.
// TODO(phase-1): switch the amounts to money.Cents once internal/money exists (pricing task).
type Price struct {
	ProductID   uuid.UUID
	DealerCents int64
	RRPCents    int64
	RetailCents int64
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
	MinMarginCents int64
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
