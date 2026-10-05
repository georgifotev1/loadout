package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"slices"

	"github.com/gfotev/loadout/internal/catalog"
	"github.com/gfotev/loadout/internal/specs"
)

// specSeed is the format of fixtures/specs.json.
type specSeed struct {
	Provider string                    `json:"provider"`
	Products map[string]map[string]any `json:"products"` // SKU → canonical field → value
}

// seedSpecs stores spec values from a seed file as admin values. It's safe to repeat: unchanged
// values aren't rewritten. SKUs that aren't in the catalog are listed and skipped.
func seedSpecs(ctx context.Context, data []byte, cat *catalog.Service, sp *specs.Service, stdout io.Writer) error {
	var seed specSeed
	if err := json.Unmarshal(data, &seed); err != nil {
		return fmt.Errorf("specs seed: %w", err)
	}
	skus := slices.Sorted(maps.Keys(seed.Products))
	ids, err := cat.ProductIDsBySKU(ctx, seed.Provider, skus)
	if err != nil {
		return err
	}
	if len(ids) == 0 {
		return errors.New("specs seed: none of the products are in the catalog: run `loadout sync` first")
	}

	var approved, incomplete int
	for _, sku := range skus {
		id, ok := ids[sku]
		if !ok {
			fmt.Fprintf(stdout, "skipped %s: not in the catalog\n", sku)
			continue
		}
		status, err := sp.SetFields(ctx, id, seed.Products[sku])
		if err != nil {
			return fmt.Errorf("specs seed: %s: %w", sku, err)
		}
		if status == specs.StatusApproved {
			approved++
		} else {
			incomplete++
			fmt.Fprintf(stdout, "%s is still incomplete\n", sku)
		}
	}
	fmt.Fprintf(stdout, "seeded %d products: %d approved, %d incomplete\n", approved+incomplete, approved, incomplete)
	return nil
}
