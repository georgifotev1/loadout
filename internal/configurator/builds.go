package configurator

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/gfotev/loadout/internal/money"
	"github.com/gfotev/loadout/internal/platform/validate"
)

// ErrNotFound is returned for a saved build that doesn't exist.
var ErrNotFound = errors.New("configurator: not found")

// SavedPart is one part of a saved build, with its price when the build was saved.
type SavedPart struct {
	Slot        Slot        `json:"slot"`
	ProductID   uuid.UUID   `json:"product_id"`
	Slug        string      `json:"slug"` // its slug then, for a part that's no longer for sale
	RetailCents money.Cents `json:"retail_cents"`
}

// SavedBuild is a build as it was saved.
type SavedBuild struct {
	Slug      string
	Parts     []SavedPart // in slot order; a part bought twice is listed twice
	CreatedAt time.Time
}

// PriceChange is a part whose price changed since the build was saved.
type PriceChange struct {
	Slot         Slot
	Slug         string
	SavedCents   money.Cents
	CurrentCents money.Cents
}

// Saved is a saved build checked again at today's parts and prices.
type Saved struct {
	SavedBuild
	Validation
	SavedTotalCents money.Cents
	PriceChanges    []PriceChange // one per product, in slot order
}

// SaveBuild stores a build at its current prices under a new short slug. A build with
// compatibility errors can be saved (to share it and ask for advice); checkout checks it again.
// An empty build, or one with a part that isn't for sale, is a validation error.
func (s *Service) SaveBuild(ctx context.Context, sel Selection) (Saved, error) {
	v, err := s.Validate(ctx, sel)
	if err != nil {
		return Saved{}, err
	}
	var verr validate.Validator
	verr.Check(len(v.Parts) > 0 || len(v.Unavailable) > 0, "build", "empty")
	for _, u := range v.Unavailable {
		verr.Add(string(u.Slot), "unavailable")
	}
	if err := verr.Err(); err != nil {
		return Saved{}, err
	}

	parts := make([]SavedPart, 0, len(v.Parts))
	for _, p := range v.Parts {
		parts = append(parts, SavedPart{
			Slot:        p.Slot,
			ProductID:   p.Part.ID,
			Slug:        p.Part.Slug,
			RetailCents: p.Part.RetailCents,
		})
	}
	// 40 random bits make a collision rare; retrying covers the rest.
	for range 3 {
		b, ok, err := s.store.insertBuild(ctx, s.pool, newSlug(), parts)
		if err != nil {
			return Saved{}, err
		}
		if ok {
			return Saved{SavedBuild: b, Validation: v, SavedTotalCents: v.TotalCents, PriceChanges: []PriceChange{}}, nil
		}
	}
	return Saved{}, errors.New("configurator: no free build slug after 3 tries")
}

// Build returns a saved build, checked again at today's parts and prices: parts no longer for
// sale are Unavailable, and parts whose price changed are in PriceChanges.
func (s *Service) Build(ctx context.Context, slug string) (Saved, error) {
	b, err := s.store.build(ctx, s.pool, slug)
	if err != nil {
		return Saved{}, err
	}

	// Look the parts up by ID, so a product whose slug changed is still found.
	ids := make([]uuid.UUID, 0, len(b.Parts))
	for _, p := range b.Parts {
		ids = append(ids, p.ProductID)
	}
	current, err := s.catalog.PartsByIDs(ctx, ids)
	if err != nil {
		return Saved{}, err
	}
	slugs := make(map[uuid.UUID]string, len(current))
	for _, p := range current {
		slugs[p.ID] = p.Slug
	}
	var sel Selection
	for _, p := range b.Parts {
		partSlug, ok := slugs[p.ProductID]
		if !ok {
			partSlug = p.Slug // not for sale: Validate reports it as unavailable
		}
		sel.set(p.Slot, partSlug)
	}
	v, err := s.Validate(ctx, sel)
	if err != nil {
		return Saved{}, fmt.Errorf("configurator: check saved build %q: %w", slug, err)
	}

	out := Saved{SavedBuild: b, Validation: v, PriceChanges: priceChanges(b.Parts, v.Parts)}
	for _, p := range b.Parts {
		out.SavedTotalCents += p.RetailCents
	}
	return out, nil
}

// priceChanges compares saved prices with the current ones. A part that's no longer for sale
// isn't a price change; Validate reports it as unavailable.
func priceChanges(saved []SavedPart, current []Picked) []PriceChange {
	now := make(map[uuid.UUID]Picked, len(current))
	for _, p := range current {
		now[p.Part.ID] = p
	}
	changes := []PriceChange{}
	seen := map[uuid.UUID]bool{}
	for _, sp := range saved {
		p, ok := now[sp.ProductID]
		if !ok || seen[sp.ProductID] || p.Part.RetailCents == sp.RetailCents {
			continue
		}
		seen[sp.ProductID] = true
		changes = append(changes, PriceChange{
			Slot:         sp.Slot,
			Slug:         p.Part.Slug,
			SavedCents:   sp.RetailCents,
			CurrentCents: p.Part.RetailCents,
		})
	}
	return changes
}

// set puts slug into slot, next to the others for RAM and storage.
func (sel *Selection) set(slot Slot, slug string) {
	switch slot {
	case SlotCPU:
		sel.CPU = slug
	case SlotMotherboard:
		sel.Motherboard = slug
	case SlotRAM:
		sel.RAM = append(sel.RAM, slug)
	case SlotGPU:
		sel.GPU = slug
	case SlotStorage:
		sel.Storage = append(sel.Storage, slug)
	case SlotPSU:
		sel.PSU = slug
	case SlotCase:
		sel.Case = slug
	case SlotCooler:
		sel.Cooler = slug
	}
}

// slugAlphabet is lowercase letters and digits without 0, 1, l and o, which are easy to misread
// when a link is typed. 32 symbols, so one random byte maps onto it without bias.
const slugAlphabet = "23456789abcdefghijkmnpqrstuvwxyz"

// newSlug returns 8 random symbols: 40 bits, short enough to share, too many to guess.
func newSlug() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b) // never fails (crypto/rand panics instead)
	for i := range b {
		b[i] = slugAlphabet[b[i]%32]
	}
	return string(b)
}
