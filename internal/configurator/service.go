package configurator

import (
	"cmp"
	"context"
	"fmt"
	"slices"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gfotev/loadout/internal/catalog"
	"github.com/gfotev/loadout/internal/money"
	"github.com/gfotev/loadout/internal/platform/validate"
	"github.com/gfotev/loadout/internal/specs"
)

// Catalog is what the configurator needs from the catalog: visible parts and their prices.
// Declared here, by the consumer, like catalog.Specs.
type Catalog interface {
	PartsByIDs(ctx context.Context, ids []uuid.UUID) ([]catalog.Part, error)
	PartsBySlugs(ctx context.Context, slugs []string) ([]catalog.Part, error)
}

// Specs is what the configurator needs from the specs module.
type Specs interface {
	SpecsByIDs(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]specs.ProductSpecs, error)
	EligibleParts(ctx context.Context, category string) ([]specs.ProductSpecs, error)
}

// Service checks builds, finds the parts that fit them and saves builds to share. It reads
// parts through the catalog and specs services; its own tables hold only saved builds.
type Service struct {
	pool    *pgxpool.Pool
	store   store
	catalog Catalog
	specs   Specs
}

func NewService(pool *pgxpool.Pool, c Catalog, s Specs) *Service {
	return &Service{pool: pool, catalog: c, specs: s}
}

// Selection is a build as the shopper picked it: a product slug per slot, "" for none. RAM and
// Storage can repeat a slug to buy that part more than once.
type Selection struct {
	CPU         string
	Motherboard string
	RAM         []string
	GPU         string
	Storage     []string
	PSU         string
	Case        string
	Cooler      string
}

// Limits on a Selection's lists: more than any real build has.
const (
	MaxRAMKits = 4
	MaxDrives  = 8
)

// Picked is one part in a build.
type Picked struct {
	Slot  Slot
	Part  catalog.Part
	Specs map[string]any // its effective specs; empty if it has none
}

// Unavailable is a picked slug that isn't for sale (any more): unknown, hidden or unpriced.
type Unavailable struct {
	Slot Slot
	Slug string
}

// Validation is a checked build.
type Validation struct {
	Result
	Parts       []Picked // in slot order; a part bought twice is listed twice
	Unavailable []Unavailable
	TotalCents  money.Cents
}

// Option is a part that fits the build in the slot asked for.
type Option struct {
	Part     catalog.Part
	Specs    map[string]any
	Findings []Finding // warnings and notes about this slot if the part is picked
}

// completionRules report a part the build still needs, not a conflict with what's picked: a
// part isn't left out of the options because the build isn't finished yet.
var completionRules = []string{RulePartRequired, RuleGPURequired, RuleCoolerRequired}

// Validate checks a build and totals its price. Unavailable parts are reported and left out of
// the check; a part in the wrong slot (a GPU as the CPU) is a validation error.
func (s *Service) Validate(ctx context.Context, sel Selection) (Validation, error) {
	picked, unavailable, err := s.load(ctx, sel)
	if err != nil {
		return Validation{}, err
	}
	b, err := buildOf(picked)
	if err != nil {
		return Validation{}, err
	}
	v := Validation{
		Result:      Check(b),
		Parts:       picked,
		Unavailable: unavailable,
	}
	for _, p := range picked {
		v.TotalCents += p.Part.RetailCents
	}
	return v, nil
}

// CompatibleOptions returns the builder-eligible parts for slot that fit the rest of the build,
// cheapest first. A part fits if picking it in place of the slot's current parts adds no error
// involving the slot. Warnings don't rule a part out; they come with it.
func (s *Service) CompatibleOptions(ctx context.Context, sel Selection, slot Slot) ([]Option, error) {
	if !slices.Contains(Slots, slot) {
		var v validate.Validator
		v.Add("slot", "unknown")
		return nil, v.Err()
	}
	picked, _, err := s.load(ctx, sel)
	if err != nil {
		return nil, err
	}
	rest, err := buildOf(slices.DeleteFunc(picked, func(p Picked) bool { return p.Slot == slot }))
	if err != nil {
		return nil, err
	}

	eligible, err := s.specs.EligibleParts(ctx, string(slot))
	if err != nil {
		return nil, err
	}
	ids := make([]uuid.UUID, 0, len(eligible))
	for _, ps := range eligible {
		ids = append(ids, ps.ProductID)
	}
	parts, err := s.catalog.PartsByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	visible := make(map[uuid.UUID]catalog.Part, len(parts))
	for _, p := range parts {
		visible[p.ID] = p
	}

	options := []Option{}
	for _, ps := range eligible {
		part, ok := visible[ps.ProductID]
		if !ok {
			continue
		}
		b := rest
		if err := add(&b, slot, ps.Specs); err != nil {
			return nil, fmt.Errorf("configurator: product %s: %w", ps.ProductID, err)
		}
		res := Check(b)
		if slices.ContainsFunc(res.Errors, func(f Finding) bool {
			return slices.Contains(f.Slots, slot) && !slices.Contains(completionRules, f.Rule)
		}) {
			continue
		}
		findings := []Finding{}
		for _, f := range slices.Concat(res.Warnings, res.Info) {
			if slices.Contains(f.Slots, slot) {
				findings = append(findings, f)
			}
		}
		options = append(options, Option{Part: part, Specs: ps.Specs, Findings: findings})
	}
	slices.SortFunc(options, func(a, b Option) int {
		return cmp.Or(cmp.Compare(a.Part.RetailCents, b.Part.RetailCents), cmp.Compare(a.Part.Name, b.Part.Name))
	})
	return options, nil
}

// load looks up the selection's parts and their specs, in slot order.
func (s *Service) load(ctx context.Context, sel Selection) ([]Picked, []Unavailable, error) {
	type pick struct {
		slot Slot
		slug string
	}
	var picks []pick
	var v validate.Validator
	v.Check(len(sel.RAM) <= MaxRAMKits, string(SlotRAM), "too_many")
	v.Check(len(sel.Storage) <= MaxDrives, string(SlotStorage), "too_many")
	one := func(slot Slot, slug string) {
		if slug != "" {
			picks = append(picks, pick{slot, slug})
		}
	}
	many := func(slot Slot, slugs []string) {
		for _, slug := range slugs {
			v.Check(slug != "", string(slot), "required")
			one(slot, slug)
		}
	}
	one(SlotCPU, sel.CPU)
	one(SlotMotherboard, sel.Motherboard)
	many(SlotRAM, sel.RAM)
	one(SlotGPU, sel.GPU)
	many(SlotStorage, sel.Storage)
	one(SlotPSU, sel.PSU)
	one(SlotCase, sel.Case)
	one(SlotCooler, sel.Cooler)
	if err := v.Err(); err != nil {
		return nil, nil, err
	}
	if len(picks) == 0 {
		return []Picked{}, []Unavailable{}, nil
	}

	slugs := make([]string, 0, len(picks))
	for _, p := range picks {
		if !slices.Contains(slugs, p.slug) {
			slugs = append(slugs, p.slug)
		}
	}
	parts, err := s.catalog.PartsBySlugs(ctx, slugs)
	if err != nil {
		return nil, nil, err
	}
	bySlug := make(map[string]catalog.Part, len(parts))
	ids := make([]uuid.UUID, 0, len(parts))
	for _, p := range parts {
		bySlug[p.Slug] = p
		ids = append(ids, p.ID)
	}
	productSpecs, err := s.specs.SpecsByIDs(ctx, ids)
	if err != nil {
		return nil, nil, err
	}

	picked := []Picked{}
	unavailable := []Unavailable{}
	for _, p := range picks {
		part, ok := bySlug[p.slug]
		switch {
		case !ok:
			unavailable = append(unavailable, Unavailable{Slot: p.slot, Slug: p.slug})
		case part.Category != string(p.slot):
			v.Add(string(p.slot), "wrong_category")
		default:
			ps := productSpecs[part.ID].Specs
			if ps == nil {
				ps = map[string]any{} // not synced into specs yet: every check reports MISSING_SPEC
			}
			picked = append(picked, Picked{Slot: p.slot, Part: part, Specs: ps})
		}
	}
	if err := v.Err(); err != nil {
		return nil, nil, err
	}
	return picked, unavailable, nil
}

// buildOf decodes picked parts' specs into a Build.
func buildOf(picked []Picked) (Build, error) {
	var b Build
	for _, p := range picked {
		if err := add(&b, p.Slot, p.Specs); err != nil {
			return Build{}, fmt.Errorf("configurator: %s %s: %w", p.Slot, p.Part.Slug, err)
		}
	}
	return b, nil
}

// add puts a part with the given specs into slot: in place of the current one, or for RAM and
// storage, next to the others. It works on a copy of b's lists, so a Build can be reused as the
// base for several candidates.
func add(b *Build, slot Slot, m map[string]any) error {
	var err error
	switch slot {
	case SlotCPU:
		b.CPU, err = decode[specs.CPU](m)
	case SlotMotherboard:
		b.Motherboard, err = decode[specs.Motherboard](m)
	case SlotRAM:
		var r *specs.RAM
		if r, err = decode[specs.RAM](m); err == nil {
			b.RAM = append(slices.Clip(b.RAM), *r)
		}
	case SlotGPU:
		b.GPU, err = decode[specs.GPU](m)
	case SlotStorage:
		var s *specs.Storage
		if s, err = decode[specs.Storage](m); err == nil {
			b.Storage = append(slices.Clip(b.Storage), *s)
		}
	case SlotPSU:
		b.PSU, err = decode[specs.PSU](m)
	case SlotCase:
		b.Case, err = decode[specs.Case](m)
	case SlotCooler:
		b.Cooler, err = decode[specs.Cooler](m)
	}
	return err
}

func decode[T any](m map[string]any) (*T, error) {
	v, err := specs.Decode[T](m)
	return &v, err
}
