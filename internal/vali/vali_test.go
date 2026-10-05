package vali

import (
	"errors"
	"io/fs"
	"path"
	"strings"
	"testing"

	"github.com/gfotev/loadout/fixtures"
	"github.com/gfotev/loadout/internal/catalog"
)

func TestCategoriesAreCanonical(t *testing.T) {
	for valiName, c := range categories {
		if !catalog.IsCategory(c) {
			t.Errorf("%q maps to %q, which isn't a catalog category", valiName, c)
		}
	}
}

// Every fixture maps, and lands in the category its directory is named after.
func TestFixtureProducts(t *testing.T) {
	files, err := fs.Glob(fixtures.Vali, FixtureGlob)
	if err != nil {
		t.Fatal(err)
	}
	dirs := make(map[string]string, len(files))
	for _, f := range files {
		dirs[strings.TrimSuffix(path.Base(f), ".json")] = path.Base(path.Dir(f))
	}

	products, err := FixtureProducts(fixtures.Vali)
	if err != nil {
		t.Fatal(err)
	}
	if len(products) != len(files) {
		t.Fatalf("%d products from %d files", len(products), len(files))
	}
	for _, p := range products {
		if p.Category != dirs[p.SKU] {
			t.Errorf("%s: category %q, fixture is in %q", p.SKU, p.Category, dirs[p.SKU])
		}
		if p.Name == "" || p.Brand == "" || p.CostCents <= 0 || len(p.Attributes) == 0 {
			t.Errorf("%s: incomplete %+v", p.SKU, p)
		}
	}
}

func TestToProduct(t *testing.T) {
	raw := RawProduct{
		SKU:              "AMD-AM4-R3-RYZ-4100-BOX",
		Brand:            "AMD",
		Name:             "Процесор AMD Ryzen 3 4100",
		CategoryPath:     []string{"Компютърни компоненти", "Процесори", "AMD"},
		Images:           []string{"https://www.vali.bg/a.jpg"},
		Specs:            map[string]string{"Сокет": "AM4"},
		DealerPriceCents: 5235,
		Currency:         "EUR",
		Stock:            map[string]int{"Sofia-Central": 16, "Plovdiv": 3},
	}
	p, err := toProduct(raw)
	if err != nil {
		t.Fatal(err)
	}
	if p.Category != "cpu" || p.CostCents != 5235 || p.StockQty != 19 || p.Attributes["Сокет"] != "AM4" || p.Specs != nil {
		t.Errorf("got %+v", p)
	}

	t.Run("unknown category", func(t *testing.T) {
		for _, cp := range [][]string{nil, {"Компютърни компоненти"}, {"Компютърни компоненти", "Твърди дискове (HDD)"}} {
			r := raw
			r.CategoryPath = cp
			if _, err := toProduct(r); !errors.Is(err, errUnknownCategory) {
				t.Errorf("%v: err = %v, want errUnknownCategory", cp, err)
			}
		}
	})

	t.Run("other currency", func(t *testing.T) {
		r := raw
		r.Currency = "BGN"
		if _, err := toProduct(r); !errors.Is(err, errCurrency) {
			t.Errorf("err = %v, want errCurrency", err)
		}
	})
}
