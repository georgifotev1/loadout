package catalog

import "github.com/gfotev/loadout/internal/money"

// Sort orders search results.
type Sort string

const (
	SortRelevance Sort = "relevance" // best text match first; by name without a text query
	SortPriceAsc  Sort = "price_asc"
	SortPriceDesc Sort = "price_desc"
)

// SearchQuery is a storefront search: optional text plus optional filters. Without text it's a
// faceted listing (the category page).
//
// TODO(phase-6): filters and facets on canonical spec fields (socket, memory type, form factor)
// through the specs service, once admins can fill spec values. No supplier fills them yet.
type SearchQuery struct {
	Text     string      // what the shopper typed; "" = no text filter
	Category string      // "" = all categories
	Brands   []string    // any of these; empty = all brands
	MinCents money.Cents // inclusive
	MaxCents money.Cents // exclusive, so it pairs with PriceBucket.MaxCents; 0 = no limit
	Sort     Sort        // "" = SortRelevance
	Page     int         // 1-based
	PageSize int
}

// Limits on SearchQuery, so one request can't build a huge query.
const (
	MaxSearchTextLen = 200 // runes
	MaxSearchBrands  = 20
)

// SearchResult is one page of matches and the facets over all of them.
type SearchResult struct {
	Products []ProductSummary
	Total    int // matching products over all pages
	Facets   Facets
}

// Facets counts matches per filter value. Each facet ignores its own filter, so after picking a
// brand the shopper still sees the other brands they could add, with their counts.
type Facets struct {
	Categories []FacetCount // most matches first
	Brands     []FacetCount // most matches first
	Prices     []PriceBucket
}

// FacetCount is how many matches have one filter value.
type FacetCount struct {
	Value string
	Count int
}

// PriceBucket is how many matches cost at least MinCents and less than MaxCents.
type PriceBucket struct {
	MinCents money.Cents
	MaxCents money.Cents // 0 for the top bucket, which has no upper limit
	Count    int
}

// priceEdges split prices into the price facet's buckets: under 50 €, 50–100 €, ...,
// 2000 € and up.
var priceEdges = []money.Cents{5000, 10000, 20000, 50000, 100000, 200000}

// priceBucket returns bucket i as Postgres' width_bucket numbers them: 0 is below the first edge,
// len(priceEdges) is the last edge and up.
func priceBucket(i, count int) PriceBucket {
	b := PriceBucket{Count: count}
	if i > 0 {
		b.MinCents = priceEdges[i-1]
	}
	if i < len(priceEdges) {
		b.MaxCents = priceEdges[i]
	}
	return b
}
