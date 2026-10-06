// Package specs keeps the canonical, typed specs of builder parts (socket, TDP, lengths, ...) and
// decides whether a product's specs are complete enough for the PC builder.
//
// Values come from two sources: the supplier's adapter, which reads what it can from its own
// data (supplier.Product.Specs), and an admin, who fills gaps and corrects mistakes. An admin
// value always wins. Specs never reads a supplier's raw format: that's the adapter's job, so the
// supplier's real API changes nothing here.
package specs

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	// ErrNotFound is returned when a product has no specs.
	ErrNotFound = errors.New("specs: not found")
	// ErrInvalidValue is returned when a supplier's value doesn't fit its canonical field: an
	// unknown field name or the wrong type. That's a bug in the adapter.
	ErrInvalidValue = errors.New("specs: invalid value")
)

// Input is one product's spec values from its supplier.
type Input struct {
	ProductID uuid.UUID
	Category  string         // canonical catalog category
	Name      string         // shown in the review queue
	Values    map[string]any // canonical field → value; may be empty
}

// Status says whether a product's specs are good enough for the builder.
type Status string

const (
	StatusIncomplete Status = "incomplete" // a required field has no value
	StatusApproved   Status = "approved"   // every required field has a value
)

// Source is where a field value came from.
type Source string

const (
	SourceSupplier Source = "supplier"
	SourceAdmin    Source = "admin"
)

// FieldValue is one value for a canonical field from one source.
type FieldValue struct {
	Field  string
	Source Source
	Value  any // as JSON: a number, bool, string or list of strings
}

// ProductSpecs is a product's effective specs.
type ProductSpecs struct {
	ProductID       uuid.UUID
	Category        string
	ProductName     string
	Specs           map[string]any // field → effective value
	Missing         []string       // required fields without a value
	Status          Status
	BuilderEligible bool
	UpdatedAt       time.Time
}

// Field is one canonical field of a product with its values. A nil value means none. Value is
// the effective one: the admin's, else the supplier's, else derived from other fields.
type Field struct {
	Name     string
	Kind     Kind
	Required bool // the builder needs it, given the product's other values
	Supplier any
	Admin    any
	Value    any
}

// ProductFields is a product's specs with all of its category's fields, for the admin's editor.
type ProductFields struct {
	ProductSpecs
	Fields []Field
}

// ReviewFilter selects review queue items.
type ReviewFilter struct {
	Category string // "" = all categories
	Page     int    // 1-based
	PageSize int
}

// ReviewPage is one page of the review queue.
type ReviewPage struct {
	Items []ProductSpecs
	Total int
}

// MaxPageSize caps ReviewFilter.PageSize.
const MaxPageSize = 100
