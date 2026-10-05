package catalog

import (
	"context"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gfotev/loadout/internal/platform/validate"
	"github.com/gfotev/loadout/internal/vali"
)

// Service is the catalog's public API. Handlers and other modules call it; nothing outside the
// package touches the catalog schema.
type Service struct {
	pool     *pgxpool.Pool
	store    store
	supplier vali.Client
	logger   *slog.Logger
}

// NewService wires the catalog. supplier is where Sync fetches products from.
func NewService(pool *pgxpool.Pool, supplier vali.Client, logger *slog.Logger) *Service {
	return &Service{pool: pool, supplier: supplier, logger: logger}
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

// Product returns the product page data for slug. Inactive or unpriced products are
// ErrNotFound, the same as a slug that doesn't exist.
func (s *Service) Product(ctx context.Context, slug string) (ProductDetail, error) {
	return s.store.visibleProductBySlug(ctx, s.pool, slug)
}
