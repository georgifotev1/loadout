// Package migrations embeds the goose SQL migrations so the binary carries its own schema.
//
// One sequence for the whole app. Each file touches one module's schema (AGENT_PLAN.md §1).
// Create a new one with: make migration name=create_catalog_products
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
