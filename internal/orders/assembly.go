package orders

import (
	"context"
	"slices"

	"github.com/gfotev/loadout/internal/configurator"
)

// Builds checks that an order's PC parts make a complete, compatible build:
// *configurator.Service. Declared here, by the consumer.
type Builds interface {
	Validate(ctx context.Context, sel configurator.Selection) (configurator.Validation, error)
}

// buildPart is a cart or order line as assembly sees it.
type buildPart struct {
	Category string
	Slug     string
	Quantity int
}

// inBuild reports whether a product of this category goes into an assembled PC. Peripherals
// don't: they ship as they are.
func inBuild(category string) bool {
	return slices.Contains(configurator.Slots, configurator.Slot(category))
}

// selection turns the PC parts among lines into the build they make. It reports false when they
// can't be one PC: no PC parts, or two of a part a PC has one of (two CPUs, two cases), or more
// RAM kits or drives than a build takes.
func selection(lines []buildPart) (configurator.Selection, bool) {
	var sel configurator.Selection
	one := func(slot *string, l buildPart) bool {
		if *slot != "" || l.Quantity != 1 {
			return false
		}
		*slot = l.Slug
		return true
	}
	parts := 0
	for _, l := range lines {
		if !inBuild(l.Category) {
			continue
		}
		parts++
		ok := true
		switch configurator.Slot(l.Category) {
		case configurator.SlotCPU:
			ok = one(&sel.CPU, l)
		case configurator.SlotMotherboard:
			ok = one(&sel.Motherboard, l)
		case configurator.SlotGPU:
			ok = one(&sel.GPU, l)
		case configurator.SlotPSU:
			ok = one(&sel.PSU, l)
		case configurator.SlotCase:
			ok = one(&sel.Case, l)
		case configurator.SlotCooler:
			ok = one(&sel.Cooler, l)
		case configurator.SlotRAM:
			for range l.Quantity {
				sel.RAM = append(sel.RAM, l.Slug)
			}
		case configurator.SlotStorage:
			for range l.Quantity {
				sel.Storage = append(sel.Storage, l.Slug)
			}
		}
		if !ok {
			return configurator.Selection{}, false
		}
	}
	ok := parts > 0 && len(sel.RAM) <= configurator.MaxRAMKits && len(sel.Storage) <= configurator.MaxDrives
	return sel, ok
}

// assemblable reports whether lines hold a PC that can be assembled: a complete build, every
// part for sale, with no compatibility errors (configurator.Assembly). Warnings don't stand in
// the way.
func (s *Service) assemblable(ctx context.Context, lines []buildPart) (bool, error) {
	sel, ok := selection(lines)
	if !ok {
		return false, nil
	}
	v, err := s.builds.Validate(ctx, sel)
	if err != nil {
		return false, err
	}
	return v.Assembly.Offered, nil
}

// slotRank orders a PC's parts the way the builder lists them; other products come last.
func slotRank(category string) int {
	if i := slices.Index(configurator.Slots, configurator.Slot(category)); i >= 0 {
		return i
	}
	return len(configurator.Slots)
}

// cartParts are a cart's lines for sale and in stock, as assembly sees them.
func cartParts(c Cart) []buildPart {
	parts := make([]buildPart, 0, len(c.Lines))
	for _, l := range c.Lines {
		if l.Product != nil && l.Product.StockQty > 0 {
			parts = append(parts, buildPart{Category: l.Product.Category, Slug: l.Slug, Quantity: l.Quantity})
		}
	}
	return parts
}
