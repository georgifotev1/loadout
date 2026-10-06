// Package orders owns carts and, later in Phase 7, orders and their state machine. It reads
// products and prices through the catalog's service, never its tables.
package orders

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gfotev/loadout/internal/catalog"
)

// ErrNotFound is returned for a cart line that doesn't exist.
var ErrNotFound = errors.New("orders: not found")

// Catalog is what orders needs from the catalog: the products for sale and their prices.
// Declared here, by the consumer, like configurator.Catalog.
type Catalog interface {
	PartsByIDs(ctx context.Context, ids []uuid.UUID) ([]catalog.Part, error)
	PartsBySlugs(ctx context.Context, slugs []string) ([]catalog.Part, error)
}

// Service is the orders module's public API.
type Service struct {
	pool    *pgxpool.Pool
	store   store
	catalog Catalog
}

func NewService(pool *pgxpool.Pool, c Catalog) *Service {
	return &Service{pool: pool, catalog: c}
}
