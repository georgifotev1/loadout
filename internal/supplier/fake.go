package supplier

import (
	"context"
	"iter"
)

// Fake is an in-memory Source for tests: it yields Items, then ItemErrors, then Err if it's set.
type Fake struct {
	Provider   string // its Name; "fake" if empty
	Items      []Product
	ItemErrors []*ItemError
	Err        error
}

func (f *Fake) Name() string {
	if f.Provider == "" {
		return "fake"
	}
	return f.Provider
}

func (f *Fake) Products(ctx context.Context) iter.Seq2[Product, error] {
	return func(yield func(Product, error) bool) {
		for _, p := range f.Items {
			if err := ctx.Err(); err != nil {
				yield(Product{}, err)
				return
			}
			if !yield(p, nil) {
				return
			}
		}
		for _, e := range f.ItemErrors {
			if !yield(Product{}, e) {
				return
			}
		}
		if f.Err != nil {
			yield(Product{}, f.Err)
		}
	}
}
