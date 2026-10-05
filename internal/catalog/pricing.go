package catalog

import (
	"strings"

	"github.com/gfotev/loadout/internal/money"
)

// specificity ranks rule scopes: a more specific matching rule beats a less specific one.
var specificity = map[RuleScope]int{
	ScopeDefault:       0,
	ScopeCategory:      1,
	ScopeBrandCategory: 2,
	ScopeSKU:           3,
}

// ResolveRule picks the rule that prices p: the most specific matching scope wins
// (sku > brand_category > category > default), then the higher Priority, then the lower ID so
// the result never depends on the order of rules. ok is false if no rule matches.
//
// Brands match case-insensitively because suppliers aren't consistent ("GIGABYTE", "Gigabyte").
// SKUs and categories match exactly.
func ResolveRule(rules []MarkupRule, p Product) (best MarkupRule, ok bool) {
	for _, r := range rules {
		if !matches(r, p) {
			continue
		}
		if !ok || better(r, best) {
			best, ok = r, true
		}
	}
	return best, ok
}

func matches(r MarkupRule, p Product) bool {
	switch r.Scope {
	case ScopeDefault:
		return true
	case ScopeCategory:
		return r.Category == p.Category
	case ScopeBrandCategory:
		return r.Category == p.Category && strings.EqualFold(r.Brand, p.Brand)
	case ScopeSKU:
		return r.SKU == p.ProviderSKU
	}
	return false
}

func better(a, b MarkupRule) bool {
	if sa, sb := specificity[a.Scope], specificity[b.Scope]; sa != sb {
		return sa > sb
	}
	if a.Priority != b.Priority {
		return a.Priority > b.Priority
	}
	return a.ID < b.ID
}

// RetailPrice turns a dealer price (VAT included) into the customer price (VAT included):
//
//  1. markup: dealer × (1 + markup_bp/10000), rounded half up to the cent
//  2. floor:  at least dealer + min_margin
//  3. ending: rounded UP to the next .99 or .90 (a price already on the ending stays)
//
// The floor comes before the ending and the ending only ever rounds up, so the margin can't drop
// below min_margin. VAT needs no separate step: both prices include it, so the markup applies
// to the gross amount and the VAT share stays the same.
func RetailPrice(dealer money.Cents, r MarkupRule) money.Cents {
	// Integer half-up rounding: add half the divisor before dividing. dealer is never
	// negative (a CHECK on catalog.prices and the sync both reject it).
	price := (dealer*money.Cents(10000+r.MarkupBP) + 5000) / 10000
	price = max(price, dealer+r.MinMarginCents)
	return roundUpToEnding(price, r.Rounding)
}

func roundUpToEnding(c money.Cents, rounding Rounding) money.Cents {
	var ending money.Cents
	switch rounding {
	case Round99:
		ending = 99
	case Round90:
		ending = 90
	default:
		return c
	}
	rounded := c/100*100 + ending
	if rounded < c {
		rounded += 100 // e.g. 12.95 with a .90 ending → 13.90
	}
	return rounded
}
