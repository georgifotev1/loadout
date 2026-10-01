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
package fixtures
