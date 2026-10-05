package vali

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"iter"
	"time"
)

// FakeClient is an in-memory Client for tests: it yields Products (ignoring since), then Err if
// it's set.
type FakeClient struct {
	Products []RawProduct
	Err      error
}

func (f *FakeClient) ListProducts(ctx context.Context, since time.Time) iter.Seq2[RawProduct, error] {
	return func(yield func(RawProduct, error) bool) {
		for _, p := range f.Products {
			if err := ctx.Err(); err != nil {
				yield(RawProduct{}, err)
				return
			}
			if !yield(p, nil) {
				return
			}
		}
		if f.Err != nil {
			yield(RawProduct{}, f.Err)
		}
	}
}

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
