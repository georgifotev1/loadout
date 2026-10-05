package vali

import (
	"encoding/json"
	"fmt"
	"io/fs"

	"github.com/gfotev/loadout/internal/supplier"
)

// FixtureGlob matches the product files in the fixtures tree: vali/<category>/<sku>.json.
const FixtureGlob = "vali/*/*.json"

// LoadFixtures decodes every FixtureGlob file in fsys (normally fixtures.Vali), in a stable
// order. The fixtures' `_mock` note isn't part of RawProduct and is dropped.
func LoadFixtures(fsys fs.FS) ([]RawProduct, error) {
	files, err := fs.Glob(fsys, FixtureGlob)
	if err != nil {
		return nil, err
	}
	products := make([]RawProduct, 0, len(files))
	for _, f := range files {
		b, err := fs.ReadFile(fsys, f)
		if err != nil {
			return nil, err
		}
		var p RawProduct
		if err := json.Unmarshal(b, &p); err != nil {
			return nil, fmt.Errorf("%s: %w", f, err)
		}
		products = append(products, p)
	}
	return products, nil
}

// FixtureProducts loads the fixtures as the catalog sees them, for tests that need realistic
// supplier data without a running mock-vali.
func FixtureProducts(fsys fs.FS) ([]supplier.Product, error) {
	raws, err := LoadFixtures(fsys)
	if err != nil {
		return nil, err
	}
	products := make([]supplier.Product, 0, len(raws))
	for _, raw := range raws {
		p, err := toProduct(raw)
		if err != nil {
			return nil, &supplier.ItemError{SKU: raw.SKU, Err: err}
		}
		products = append(products, p)
	}
	return products, nil
}
