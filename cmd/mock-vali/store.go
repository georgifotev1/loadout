package main

import (
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"slices"
	"sync"
	"time"

	"github.com/gfotev/loadout/internal/vali"
)

// product is one item in Vali's raw format. It's the same type the Loadout client decodes, so
// the mock and the client can't disagree about the wire format.
type product = vali.RawProduct

var errNotFound = errors.New("product not found")

// store is the in-memory catalog. Products are treated as immutable values: a mutation builds a
// new product with fresh maps and swaps it in, so a caller can encode what it got from list/get
// without holding the lock.
type store struct {
	src      fs.FS
	now      func() time.Time
	mu       sync.RWMutex
	bySKU    map[string]*product
	skus     []string        // sorted, so pages are stable
	delisted map[string]bool // hidden from list and get, to simulate a product leaving Vali's catalog
}

func newStore(src fs.FS, now func() time.Time) (*store, error) {
	s := &store{src: src, now: now}
	if err := s.reset(); err != nil {
		return nil, err
	}
	return s, nil
}

// reset reloads every fixture and drops all mutations.
func (s *store) reset() error {
	products, err := vali.LoadFixtures(s.src)
	if err != nil {
		return err
	}
	if len(products) == 0 {
		return errors.New("no fixtures found")
	}
	bySKU := make(map[string]*product, len(products))
	for i := range products {
		p := &products[i]
		if _, dup := bySKU[p.SKU]; dup {
			return fmt.Errorf("duplicate sku %q", p.SKU)
		}
		bySKU[p.SKU] = p
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.bySKU = bySKU
	s.skus = slices.Sorted(maps.Keys(bySKU))
	s.delisted = map[string]bool{}
	return nil
}

// list returns one page of the listed products changed at or after since (zero = all), plus the
// total that matched. Inclusive, so a client that passes its last seen updated_at gets that
// product again rather than missing a change made in the same second.
func (s *store) list(since time.Time, page, pageSize int) ([]*product, int) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var matched []*product
	for _, sku := range s.skus {
		p := s.bySKU[sku]
		if s.delisted[sku] || p.UpdatedAt.Before(since) {
			continue
		}
		matched = append(matched, p)
	}
	start := min((page-1)*pageSize, len(matched))
	end := min(start+pageSize, len(matched))
	return matched[start:end], len(matched)
}

func (s *store) get(sku string) (*product, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.bySKU[sku]
	if !ok || s.delisted[sku] {
		return nil, errNotFound
	}
	return p, nil
}

// mutation changes one product. Nil fields are left alone.
type mutation struct {
	SKU              string             `json:"sku"`
	DealerPriceCents *int64             `json:"dealer_price_cents"`
	RRPCents         *int64             `json:"rrp_cents"`
	Stock            map[string]int     `json:"stock"` // merged per warehouse
	Specs            map[string]*string `json:"specs"` // merged; null deletes the key
	Name             *string            `json:"name"`  // a content change, for raw_hash tests
	Delisted         *bool              `json:"delisted"`
}

// mutate applies m and bumps updated_at, the way a change on Vali's side would. A delisted
// product can still be mutated (that's how it comes back).
func (s *store) mutate(m mutation) (*product, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	old, ok := s.bySKU[m.SKU]
	if !ok {
		return nil, errNotFound
	}
	p := *old
	p.Specs = maps.Clone(old.Specs)
	p.Stock = maps.Clone(old.Stock)

	if m.DealerPriceCents != nil {
		p.DealerPriceCents = *m.DealerPriceCents
	}
	if m.RRPCents != nil {
		p.RRPCents = *m.RRPCents
	}
	maps.Copy(p.Stock, m.Stock)
	for k, v := range m.Specs {
		if v == nil {
			delete(p.Specs, k)
		} else {
			p.Specs[k] = *v
		}
	}
	if m.Name != nil {
		p.Name = *m.Name
	}
	if m.Delisted != nil {
		s.delisted[m.SKU] = *m.Delisted
	}
	p.UpdatedAt = s.now().UTC().Truncate(time.Second) // same precision as the fixtures

	s.bySKU[m.SKU] = &p
	return &p, nil
}
