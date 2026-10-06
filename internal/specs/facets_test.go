package specs

import (
	"reflect"
	"slices"
	"testing"

	"github.com/google/uuid"
)

func TestFacetFieldsAreFields(t *testing.T) {
	for category, fields := range facetFields {
		sc, ok := schemas[category]
		if !ok {
			t.Errorf("%s: not a category with specs", category)
			continue
		}
		for _, f := range fields {
			if kind, ok := sc.kinds[f]; !ok || kind == KindBoolean {
				t.Errorf("%s: facet %q isn't a non-boolean field", category, f)
			}
		}
	}
}

func TestFacetIndex(t *testing.T) {
	am4DDR4, am5DDR5, am5DDR5b, unknown := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	ix := newFacetIndex("motherboard", map[uuid.UUID]map[string]any{
		am4DDR4:  {"socket": "AM4", "memory_type": "DDR4", "form_factor": "ATX"},
		am5DDR5:  {"socket": "AM5", "memory_type": "DDR5", "form_factor": "mATX"},
		am5DDR5b: {"socket": "AM5", "memory_type": "DDR5", "form_factor": "ATX"},
		unknown:  {},
	})
	all := []uuid.UUID{am4DDR4, am5DDR5, am5DDR5b, unknown}

	match := func(f Filter) []uuid.UUID {
		got := ix.Match(f)
		slices.SortFunc(got, func(a, b uuid.UUID) int { return slices.Index(all, a) - slices.Index(all, b) })
		return got
	}
	if got := match(nil); !slices.Equal(got, all) {
		t.Errorf("no filter matched %v, want all", got)
	}
	if got := match(Filter{"socket": {"AM5"}}); !slices.Equal(got, []uuid.UUID{am5DDR5, am5DDR5b}) {
		t.Errorf("socket AM5 matched %v", got)
	}
	// Values of one field widen; separate fields narrow.
	if got := match(Filter{"socket": {"AM4", "AM5"}, "form_factor": {"ATX"}}); !slices.Equal(got, []uuid.UUID{am4DDR4, am5DDR5b}) {
		t.Errorf("AM4|AM5 + ATX matched %v", got)
	}

	facets := ix.Facets(all, Filter{"socket": {"AM5"}})
	want := []Facet{
		// The socket facet ignores the socket filter.
		{Field: "socket", Values: []FacetValue{{"AM4", 1}, {"AM5", 2}}},
		{Field: "form_factor", Values: []FacetValue{{"ATX", 1}, {"mATX", 1}}},
		{Field: "memory_type", Values: []FacetValue{{"DDR5", 2}}},
	}
	if !reflect.DeepEqual(facets, want) {
		t.Errorf("facets\n got %+v\nwant %+v", facets, want)
	}

	// Only the given products count.
	if got := ix.Facets([]uuid.UUID{am4DDR4, uuid.New()}, nil)[0]; !reflect.DeepEqual(got.Values, []FacetValue{{"AM4", 1}}) {
		t.Errorf("among one board: %+v", got)
	}
}

func TestFacetValues(t *testing.T) {
	a, b := uuid.New(), uuid.New()
	// Numbers from JSON are float64; lists count once per entry.
	ix := newFacetIndex("case", map[uuid.UUID]map[string]any{
		a: {"form_factors_supported": []any{"ATX", "mATX", "ITX"}},
		b: {"form_factors_supported": []any{"ITX"}},
	})
	got := ix.Facets([]uuid.UUID{a, b}, nil)
	want := []Facet{{Field: "form_factors_supported", Values: []FacetValue{{"ATX", 1}, {"ITX", 2}, {"mATX", 1}}}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("case facets %+v", got)
	}

	ix = newFacetIndex("ram", map[uuid.UUID]map[string]any{
		a: {"capacity_gb_total": float64(32)},
		b: {"capacity_gb_total": float64(8)},
	})
	got = ix.Facets([]uuid.UUID{a, b}, Filter{"capacity_gb_total": {"32"}})
	// Integers sort as numbers: 8 before 32.
	if !reflect.DeepEqual(got[0].Values, []FacetValue{{"8", 1}, {"32", 1}}) {
		t.Errorf("ram facets %+v", got)
	}
	if ids := ix.Match(Filter{"capacity_gb_total": {"32"}}); !slices.Equal(ids, []uuid.UUID{a}) {
		t.Errorf("32 GB matched %v", ids)
	}
}
