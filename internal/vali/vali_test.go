package vali

import (
	"testing"

	"github.com/gfotev/loadout/fixtures"
)

func TestLoadFixtures(t *testing.T) {
	products, err := LoadFixtures(fixtures.Vali)
	if err != nil {
		t.Fatal(err)
	}
	if len(products) == 0 {
		t.Fatal("no fixtures")
	}
	for _, p := range products {
		if p.SKU == "" || p.Name == "" || len(p.CategoryPath) == 0 {
			t.Errorf("incomplete fixture %+v", p)
		}
		if p.Currency != "EUR" {
			t.Errorf("%s: currency %q, want EUR", p.SKU, p.Currency)
		}
	}
}
