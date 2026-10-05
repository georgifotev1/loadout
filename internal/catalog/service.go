package catalog

import (
	"context"
	"errors"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gfotev/loadout/internal/platform/validate"
	"github.com/gfotev/loadout/internal/specs"
	"github.com/gfotev/loadout/internal/supplier"
)

// Specs is what the catalog needs from the specs module. It's declared here, by the consumer,
// so the catalog depends on these two methods rather than on everything *specs.Service does.
type Specs interface {
	// SaveTx stores a product's supplier spec values inside the catalog's sync transaction.
	SaveTx(ctx context.Context, tx pgx.Tx, in specs.Input) (specs.Status, error)
	// Specs returns a product's canonical specs, or specs.ErrNotFound.
	Specs(ctx context.Context, productID uuid.UUID) (specs.ProductSpecs, error)
}

// Service is the catalog's public API. Handlers and other modules call it; nothing outside the
// package touches the catalog schema.
type Service struct {
	pool     *pgxpool.Pool
	store    store
	supplier supplier.Source
	specs    Specs
	logger   *slog.Logger
}

// NewService wires the catalog. src is the supplier Sync fetches products from; specs keeps
// their canonical specs.
func NewService(pool *pgxpool.Pool, src supplier.Source, specs Specs, logger *slog.Logger) *Service {
	return &Service{pool: pool, supplier: src, specs: specs, logger: logger}
}

// List returns one page of the products shoppers can see: active and priced.
func (s *Service) List(ctx context.Context, f ListFilter) (ProductPage, error) {
	var v validate.Validator
	v.Check(f.Category == "" || IsCategory(f.Category), "category", "unknown")
	v.Check(f.Page >= 1, "page", "out_of_range")
	v.Check(f.PageSize >= 1 && f.PageSize <= MaxPageSize, "page_size", "out_of_range")
	if err := v.Err(); err != nil {
		return ProductPage{}, err
	}

	products, total, err := s.store.listProducts(ctx, s.pool, f.Category, f.PageSize, (f.Page-1)*f.PageSize)
	if err != nil {
		return ProductPage{}, err
	}
	return ProductPage{Products: products, Total: total}, nil
}

// Product returns the product page data for slug, with its canonical specs. Inactive or
// unpriced products are ErrNotFound, the same as a slug that doesn't exist.
func (s *Service) Product(ctx context.Context, slug string) (ProductDetail, error) {
	p, err := s.store.visibleProductBySlug(ctx, s.pool, slug)
	if err != nil {
		return ProductDetail{}, err
	}
	ps, err := s.specs.Specs(ctx, p.ID)
	switch {
	case errors.Is(err, specs.ErrNotFound):
		p.Specs = map[string]any{} // a peripheral
	case err != nil:
		return ProductDetail{}, err
	default:
		p.Specs = ps.Specs
	}
	return p, nil
}
