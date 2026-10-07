// Package orders owns carts, orders and the orders' state machine. It reads products and prices
// through the catalog's service, never its tables.
package orders

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/gfotev/loadout/internal/catalog"
)

// ErrNotFound is returned for a cart line or an order that doesn't exist.
var ErrNotFound = errors.New("orders: not found")

// Catalog is what orders needs from the catalog: the products for sale and their prices.
// Declared here, by the consumer, like configurator.Catalog.
type Catalog interface {
	PartsByIDs(ctx context.Context, ids []uuid.UUID) ([]catalog.Part, error)
	PartsBySlugs(ctx context.Context, slugs []string) ([]catalog.Part, error)
	SaleParts(ctx context.Context, ids []uuid.UUID) ([]catalog.SalePart, error)
}

// Inserter enqueues background jobs in the caller's transaction: *jobs.Client.
type Inserter interface {
	InsertTx(ctx context.Context, tx pgx.Tx, args river.JobArgs, opts *river.InsertOpts) (*rivertype.JobInsertResult, error)
}

// Service is the orders module's public API.
type Service struct {
	pool    *pgxpool.Pool
	store   store
	catalog Catalog
	jobs    Inserter
	now     func() time.Time
}

// NewService returns the orders service. jobs enqueues the orders' steps (StepArgs).
func NewService(pool *pgxpool.Pool, c Catalog, jobs Inserter) *Service {
	return &Service{pool: pool, catalog: c, jobs: jobs, now: time.Now}
}
