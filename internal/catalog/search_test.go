package catalog

import (
	"context"
	"errors"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/gfotev/loadout/internal/money"
	"github.com/gfotev/loadout/internal/platform/validate"
	"github.com/gfotev/loadout/internal/specs"
)

func TestSearch(t *testing.T) {
	env := newSyncEnv(t)
	env.mustSync(t)
	ctx := context.Background()

	search := func(t *testing.T, q SearchQuery) SearchResult {
		t.Helper()
		if q.Page == 0 {
			q.Page, q.PageSize = 1, MaxPageSize
		}
		res, err := env.svc.Search(ctx, q)
		if err != nil {
			t.Fatalf("search %+v: %v", q, err)
		}
		return res
	}
	count := func(facet []FacetCount, value string) int {
		for _, f := range facet {
			if f.Value == value {
				return f.Count
			}
		}
		return 0
	}

	t.Run("category without text is a faceted listing", func(t *testing.T) {
		res := search(t, SearchQuery{Category: "cpu"})
		if res.Total != 7 || len(res.Products) != 7 {
			t.Fatalf("total=%d len=%d, want 7", res.Total, len(res.Products))
		}
		if count(res.Facets.Brands, "AMD") != 4 || count(res.Facets.Brands, "INTEL") != 3 {
			t.Errorf("brands %+v", res.Facets.Brands)
		}
		// The category facet ignores the category filter: it counts every category.
		if count(res.Facets.Categories, "cpu") != 7 || len(res.Facets.Categories) != len(Categories) {
			t.Errorf("categories %+v", res.Facets.Categories)
		}
		sum := 0
		for _, b := range res.Facets.Prices {
			sum += b.Count
		}
		if sum != 7 {
			t.Errorf("price buckets %+v add up to %d, want 7", res.Facets.Prices, sum)
		}
		// Relevance without text is by name.
		for i := 1; i < len(res.Products); i++ {
			if res.Products[i-1].Name > res.Products[i].Name {
				t.Errorf("not by name: %q before %q", res.Products[i-1].Name, res.Products[i].Name)
			}
		}
	})

	t.Run("a facet ignores its own filter", func(t *testing.T) {
		res := search(t, SearchQuery{Category: "cpu", Brands: []string{"AMD"}})
		if res.Total != 4 {
			t.Fatalf("total=%d, want 4", res.Total)
		}
		for _, p := range res.Products {
			if p.Brand != "AMD" {
				t.Errorf("got %s product %s", p.Brand, p.Slug)
			}
		}
		if count(res.Facets.Brands, "INTEL") != 3 {
			t.Errorf("brands %+v, want INTEL still counted", res.Facets.Brands)
		}
		if count(res.Facets.Categories, "cpu") != 4 {
			t.Errorf("categories %+v, want the brand filter applied", res.Facets.Categories)
		}
	})

	t.Run("text matches words and typos", func(t *testing.T) {
		for _, text := range []string{"ryzen", "RYZEN", "ryzn", "amd ryzen"} {
			res := search(t, SearchQuery{Text: text})
			if res.Total != 4 {
				t.Errorf("%q: total=%d, want the 4 Ryzens", text, res.Total)
			}
			for _, p := range res.Products {
				if !strings.Contains(strings.ToLower(p.Name), "ryzen") {
					t.Errorf("%q matched %q", text, p.Name)
				}
			}
		}
		// Cyrillic, with a typo.
		if res := search(t, SearchQuery{Text: "процесар", Category: "cpu"}); res.Total != 7 {
			t.Errorf("процесар: total=%d, want 7", res.Total)
		}
		if res := search(t, SearchQuery{Text: "spaceship"}); res.Total != 0 || len(res.Facets.Brands) != 0 {
			t.Errorf("spaceship: %+v", res)
		}
	})

	t.Run("the exact model ranks first", func(t *testing.T) {
		res := search(t, SearchQuery{Text: "7800X3D"})
		if len(res.Products) == 0 || !strings.Contains(res.Products[0].Name, "7800X3D") {
			t.Errorf("got %+v", res.Products)
		}
	})

	t.Run("price filter and sort", func(t *testing.T) {
		all := search(t, SearchQuery{Category: "cpu", Sort: SortPriceAsc})
		cheapest, dearest := all.Products[0].RetailCents, all.Products[len(all.Products)-1].RetailCents
		for i := 1; i < len(all.Products); i++ {
			if all.Products[i-1].RetailCents > all.Products[i].RetailCents {
				t.Fatalf("not by price: %+v", all.Products)
			}
		}

		desc := search(t, SearchQuery{Category: "cpu", Sort: SortPriceDesc})
		if desc.Products[0].RetailCents != dearest {
			t.Errorf("price_desc starts at %d, want %d", desc.Products[0].RetailCents, dearest)
		}

		// Min inclusive, max exclusive: [cheapest, dearest) drops only the dearest.
		res := search(t, SearchQuery{Category: "cpu", MinCents: cheapest, MaxCents: dearest})
		if res.Total != 6 {
			t.Errorf("total=%d, want 6", res.Total)
		}
		// The price facet ignores the price filter.
		sum := 0
		for _, b := range res.Facets.Prices {
			sum += b.Count
		}
		if sum != 7 {
			t.Errorf("price buckets add up to %d, want 7", sum)
		}
	})

	t.Run("pages", func(t *testing.T) {
		first := search(t, SearchQuery{Category: "cpu", Page: 1, PageSize: 5})
		second := search(t, SearchQuery{Category: "cpu", Page: 2, PageSize: 5})
		if len(first.Products) != 5 || len(second.Products) != 2 || second.Total != 7 {
			t.Errorf("page sizes %d and %d, total %d", len(first.Products), len(second.Products), second.Total)
		}
	})

	t.Run("invalid queries", func(t *testing.T) {
		for name, q := range map[string]SearchQuery{
			"q":         {Text: strings.Repeat("я", MaxSearchTextLen+1)},
			"category":  {Category: "spaceship"},
			"brand":     {Brands: make([]string, MaxSearchBrands+1)},
			"price_min": {MinCents: -1},
			"price_max": {MinCents: 5000, MaxCents: 5000},
			"sort":      {Sort: "random"},
			"spec":      {Category: "cpu", Specs: specs.Filter{"cores": {"8"}}},
		} {
			q.Page, q.PageSize = 1, 10
			_, err := env.svc.Search(ctx, q)
			var verr *validate.Error
			if !errors.As(err, &verr) || verr.Fields[0].Field != name {
				t.Errorf("%s: err = %v", name, err)
			}
		}
	})
}

func TestSearchSpecs(t *testing.T) {
	env := newSyncEnv(t)
	env.mustSync(t)
	ctx := context.Background()

	sockets := map[string]string{
		"AMD-AM4-R3-RYZ-4100-BOX": "AM4",
		"AMD-AM4-R5-RYZEN-5600G":  "AM4",
		"AMD-AM5-R5-7600-BOX":     "AM5",
		"AMD-AM5-R7-7800X3D-BOX":  "AM5",
		"INTEL-I5-12400F-BOX":     "LGA1700",
		"INTEL-I5-14600K-TRAY":    "LGA1700",
		"INTEL-ULTRA-5-245K-BOX":  "LGA1851",
	}
	ids, err := env.svc.ProductIDsBySKU(ctx, env.src.Name(), slices.Collect(maps.Keys(sockets)))
	if err != nil || len(ids) != len(sockets) {
		t.Fatalf("ids %v, err %v", ids, err)
	}
	sp := specs.NewService(env.pool)
	for sku, socket := range sockets {
		if _, err := sp.SetFields(ctx, ids[sku], map[string]any{"socket": socket}); err != nil {
			t.Fatal(err)
		}
	}

	search := func(t *testing.T, q SearchQuery) SearchResult {
		t.Helper()
		q.Page, q.PageSize = 1, MaxPageSize
		res, err := env.svc.Search(ctx, q)
		if err != nil {
			t.Fatalf("search %+v: %v", q, err)
		}
		return res
	}
	socketFacet := func(t *testing.T, res SearchResult) []specs.FacetValue {
		t.Helper()
		if len(res.Facets.Specs) != 1 || res.Facets.Specs[0].Field != "socket" {
			t.Fatalf("spec facets %+v, want socket only", res.Facets.Specs)
		}
		return res.Facets.Specs[0].Values
	}

	t.Run("a category counts its spec values", func(t *testing.T) {
		res := search(t, SearchQuery{Category: "cpu"})
		want := []specs.FacetValue{{Value: "AM4", Count: 2}, {Value: "AM5", Count: 2}, {Value: "LGA1700", Count: 2}, {Value: "LGA1851", Count: 1}}
		if got := socketFacet(t, res); !slices.Equal(got, want) {
			t.Errorf("sockets %+v", got)
		}
		for _, p := range res.Products {
			if p.Specs["socket"] == nil {
				t.Errorf("%s has no socket in its specs: %v", p.Slug, p.Specs)
			}
		}
	})

	t.Run("filtering on a spec value", func(t *testing.T) {
		res := search(t, SearchQuery{Category: "cpu", Specs: specs.Filter{"socket": {"AM5", "LGA1851"}}})
		if res.Total != 3 {
			t.Fatalf("total=%d, want 3", res.Total)
		}
		// The socket facet ignores the socket filter; the others apply it.
		if got := socketFacet(t, res); len(got) != 4 {
			t.Errorf("sockets %+v, want all four", got)
		}
		if b := res.Facets.Brands; len(b) != 2 || b[0] != (FacetCount{Value: "AMD", Count: 2}) || b[1] != (FacetCount{Value: "INTEL", Count: 1}) {
			t.Errorf("brands %+v", b)
		}
	})

	t.Run("other filters narrow the spec counts", func(t *testing.T) {
		res := search(t, SearchQuery{Category: "cpu", Brands: []string{"INTEL"}})
		want := []specs.FacetValue{{Value: "LGA1700", Count: 2}, {Value: "LGA1851", Count: 1}}
		if got := socketFacet(t, res); !slices.Equal(got, want) {
			t.Errorf("sockets %+v", got)
		}
	})

	t.Run("a value nothing has", func(t *testing.T) {
		res := search(t, SearchQuery{Category: "cpu", Specs: specs.Filter{"socket": {"AM3"}}})
		if res.Total != 0 || len(res.Facets.Brands) != 0 {
			t.Errorf("total=%d brands=%+v, want nothing", res.Total, res.Facets.Brands)
		}
	})

	t.Run("no spec facets without a category", func(t *testing.T) {
		res := search(t, SearchQuery{Text: "ryzen"})
		if len(res.Facets.Specs) != 0 {
			t.Errorf("spec facets %+v", res.Facets.Specs)
		}
		if _, err := env.svc.Search(ctx, SearchQuery{Specs: specs.Filter{"socket": {"AM5"}}, Page: 1, PageSize: 10}); err == nil {
			t.Error("a spec filter without a category was accepted")
		}
	})
}

func TestPriceBucket(t *testing.T) {
	last := len(priceEdges)
	for _, tc := range []struct {
		i        int
		min, max money.Cents
	}{
		{0, 0, priceEdges[0]},
		{1, priceEdges[0], priceEdges[1]},
		{last, priceEdges[last-1], 0},
	} {
		if b := priceBucket(tc.i, 3); b != (PriceBucket{MinCents: tc.min, MaxCents: tc.max, Count: 3}) {
			t.Errorf("priceBucket(%d) = %+v", tc.i, b)
		}
	}
}
