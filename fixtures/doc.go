// Package fixtures holds the hand-approved product data that mock-vali serves.
//
// Layout: vali/<category>/<sku>.json, one product per file, in the raw format described by
// vali/product.schema.json. <category> is the canonical catalog category (cpu, motherboard,
// ram, gpu, psu, case, cooler, storage, peripheral.*).
//
// The approved source pages are listed in vali/sources.txt. To add a product, add its vali.bg
// URL there and run `go run ./tools/vali-fixture`. Public fields are copied as-is (no
// normalization, that is Phase 2's job); dealer price, stock and updated_at are invented and
// the `_mock` field says so. Hand edits are fine for test scenarios, but note them in `_mock`.
//
// specs.json holds admin spec values for the builder parts, keyed by SKU, so the PC builder has
// data before admins can enter it (`loadout specs seed`).
package fixtures

import "embed"

// Vali holds the product files as vali/<category>/<sku>.json, for mock-vali to serve.
//
//go:embed vali/*/*.json
var Vali embed.FS

// Specs is specs.json: canonical spec values for the builder parts in Vali.
//
//go:embed specs.json
var Specs []byte
