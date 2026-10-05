package configurator

import (
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/gfotev/loadout/internal/catalog"
	"github.com/gfotev/loadout/internal/money"
)

func TestNewSlug(t *testing.T) {
	seen := map[string]bool{}
	for range 1000 {
		s := newSlug()
		if len(s) != 8 || strings.Trim(s, slugAlphabet) != "" {
			t.Fatalf("slug %q", s)
		}
		if seen[s] {
			t.Fatalf("slug %q repeated", s)
		}
		seen[s] = true
	}
}

func TestPriceChanges(t *testing.T) {
	cpu, ram, gpu := uuid.New(), uuid.New(), uuid.New()
	picked := func(slot Slot, id uuid.UUID, slug string, cents money.Cents) Picked {
		return Picked{Slot: slot, Part: catalog.Part{ID: id, ProductSummary: catalog.ProductSummary{Slug: slug, RetailCents: cents}}}
	}
	saved := []SavedPart{
		{Slot: SlotCPU, ProductID: cpu, Slug: "cpu", RetailCents: 200_00},
		{Slot: SlotRAM, ProductID: ram, Slug: "ram", RetailCents: 80_00},
		{Slot: SlotRAM, ProductID: ram, Slug: "ram", RetailCents: 80_00},
		{Slot: SlotGPU, ProductID: gpu, Slug: "gpu", RetailCents: 500_00},
	}
	current := []Picked{
		picked(SlotCPU, cpu, "cpu", 200_00),
		picked(SlotRAM, ram, "ram-renamed", 75_00),
		picked(SlotRAM, ram, "ram-renamed", 75_00),
		// the GPU is no longer for sale
	}

	got := priceChanges(saved, current)
	want := []PriceChange{{Slot: SlotRAM, Slug: "ram-renamed", SavedCents: 80_00, CurrentCents: 75_00}}
	if !slices.Equal(got, want) {
		t.Errorf("changes %+v, want %+v", got, want)
	}
}
