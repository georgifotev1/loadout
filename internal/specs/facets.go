package specs

import (
	"cmp"
	"context"
	"maps"
	"slices"
	"strconv"

	"github.com/google/uuid"
)

// facetFields are the spec fields a category's listing can be filtered by, in the order the
// listing shows them: the ones shoppers pick parts by (socket, memory type, VRAM, form factor).
// Booleans aren't facets.
var facetFields = map[string][]string{
	"cpu":         {"socket"},
	"motherboard": {"socket", "chipset", "form_factor", "memory_type"},
	"ram":         {"type", "capacity_gb_total", "modules", "speed_mhz"},
	"gpu":         {"chipset", "vram_gb"},
	"cooler":      {"type", "sockets", "radiator_mm"},
	"storage":     {"interface", "capacity_gb", "form_factor"},
	"psu":         {"wattage_w", "form_factor"},
	"case":        {"form_factors_supported", "psu_form_factor"},
}

// FacetFields returns the spec fields a category's listing can be filtered by. A category
// without canonical specs has none.
func FacetFields(category string) []string {
	return slices.Clone(facetFields[category])
}

// Filter selects products by spec values: field → the values wanted, as Facet spells them. A
// product matches when, for every field, it has one of the wanted values.
type Filter map[string][]string

// Facet counts products per value of one spec field.
type Facet struct {
	Field  string
	Values []FacetValue // smallest number first for integer fields, else alphabetical
}

// FacetValue is how many products have one value of a field.
type FacetValue struct {
	Value string // the canonical value as text: "AM5", "16", "nvme"
	Count int
}

// FacetIndex holds the facet values of every product in one category, so a listing can be
// filtered and counted without a query per facet.
type FacetIndex struct {
	category string
	values   map[uuid.UUID]map[string][]string // product → field → values as text
}

// FacetIndex loads the facet values of a category's products. A product counts with the
// values it has, complete or not: a board with a known socket can be found by its socket.
func (s *Service) FacetIndex(ctx context.Context, category string) (*FacetIndex, error) {
	if len(facetFields[category]) == 0 {
		return newFacetIndex(category, nil), nil
	}
	specs, err := s.store.categorySpecs(ctx, s.pool, category)
	if err != nil {
		return nil, err
	}
	return newFacetIndex(category, specs), nil
}

func newFacetIndex(category string, specs map[uuid.UUID]map[string]any) *FacetIndex {
	ix := &FacetIndex{category: category, values: make(map[uuid.UUID]map[string][]string, len(specs))}
	for id, s := range specs {
		vals := map[string][]string{}
		for _, field := range facetFields[category] {
			if v := facetText(s[field]); len(v) > 0 {
				vals[field] = v
			}
		}
		ix.values[id] = vals
	}
	return ix
}

// facetText turns a spec value into the facet values it counts under: one for a string or a
// number, one per entry for a list.
func facetText(v any) []string {
	switch v := v.(type) {
	case string:
		return []string{v}
	case float64: // from JSON
		return []string{strconv.FormatInt(int64(v), 10)}
	case int:
		return []string{strconv.Itoa(v)}
	case []any, []string:
		return slices.Compact(slices.Sorted(slices.Values(stringList(v))))
	}
	return nil
}

// Match returns the products that match every field of f, in no particular order.
func (ix *FacetIndex) Match(f Filter) []uuid.UUID {
	ids := []uuid.UUID{}
	for id, vals := range ix.values {
		if matches(vals, f, "") {
			ids = append(ids, id)
		}
	}
	return ids
}

// Facets counts, among the given products, how many have each value of the category's facet
// fields. Like the catalog's other facets, each one ignores its own field of f, so after picking
// AM5 the socket facet still counts AM4. Products not in the index are skipped; fields with no
// values left out.
func (ix *FacetIndex) Facets(among []uuid.UUID, f Filter) []Facet {
	facets := []Facet{}
	for _, field := range facetFields[ix.category] {
		counts := map[string]int{}
		for _, id := range among {
			vals, ok := ix.values[id]
			if !ok || !matches(vals, f, field) {
				continue
			}
			for _, v := range vals[field] {
				counts[v]++
			}
		}
		if len(counts) == 0 {
			continue
		}
		values := make([]FacetValue, 0, len(counts))
		for _, v := range slices.Collect(maps.Keys(counts)) {
			values = append(values, FacetValue{Value: v, Count: counts[v]})
		}
		integer := schemas[ix.category].kinds[field] == KindInteger
		slices.SortFunc(values, func(a, b FacetValue) int {
			if integer {
				x, _ := strconv.Atoi(a.Value)
				y, _ := strconv.Atoi(b.Value)
				return cmp.Compare(x, y)
			}
			return cmp.Compare(a.Value, b.Value)
		})
		facets = append(facets, Facet{Field: field, Values: values})
	}
	return facets
}

// matches reports whether a product's values match every field of f except skip.
func matches(vals map[string][]string, f Filter, skip string) bool {
	for field, want := range f {
		if field == skip || len(want) == 0 {
			continue
		}
		if !slices.ContainsFunc(vals[field], func(v string) bool { return slices.Contains(want, v) }) {
			return false
		}
	}
	return true
}
