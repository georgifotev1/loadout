package specs

import (
	"context"
	"fmt"
	"slices"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gfotev/loadout/internal/platform/validate"
)

// Service is the specs module's public API.
// TODO(phase-6): SetField / ClearField for admins, used by the review queue UI.
type Service struct {
	pool  *pgxpool.Pool
	store store
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool}
}

// SaveTx stores a product's supplier spec values inside the caller's transaction (the catalog's
// per-product sync transaction) and recomputes its effective specs. Admin values are kept. It's
// safe to repeat.
//
// For a category without canonical specs (peripherals) it removes any specs the product had and
// returns "". A value that doesn't fit its field is ErrInvalidValue and nothing is stored.
func (s *Service) SaveTx(ctx context.Context, tx pgx.Tx, in Input) (Status, error) {
	sc, ok := schemas[in.Category]
	if !ok {
		return "", s.store.deleteProduct(ctx, tx, in.ProductID)
	}
	if err := sc.validate(in.Values); err != nil {
		return "", fmt.Errorf("%w: product %s: %v", ErrInvalidValue, in.ProductID, err)
	}
	// First, so the row lock serializes concurrent saves of this product.
	if err := s.store.upsertProduct(ctx, tx, in); err != nil {
		return "", err
	}
	if err := s.store.replaceSupplierFields(ctx, tx, in.ProductID, in.Values); err != nil {
		return "", err
	}
	rows, err := s.store.fields(ctx, tx, in.ProductID)
	if err != nil {
		return "", err
	}
	specs, missing, status := resolve(in.Category, rows)
	if err := s.store.saveResolved(ctx, tx, in.ProductID, specs, missing, status); err != nil {
		return "", err
	}
	return status, nil
}

// Specs returns a product's effective specs, or ErrNotFound if it has none (a peripheral, or a
// product that hasn't been synced yet).
func (s *Service) Specs(ctx context.Context, productID uuid.UUID) (ProductSpecs, error) {
	return s.store.productSpecs(ctx, s.pool, productID)
}

// ReviewQueue returns one page of products whose specs are incomplete, oldest first.
func (s *Service) ReviewQueue(ctx context.Context, f ReviewFilter) (ReviewPage, error) {
	var v validate.Validator
	v.Check(f.Category == "" || slices.Contains(Categories(), f.Category), "category", "unknown")
	v.Check(f.Page >= 1, "page", "out_of_range")
	v.Check(f.PageSize >= 1 && f.PageSize <= MaxPageSize, "page_size", "out_of_range")
	if err := v.Err(); err != nil {
		return ReviewPage{}, err
	}

	items, total, err := s.store.reviewQueue(ctx, s.pool, f)
	if err != nil {
		return ReviewPage{}, err
	}
	return ReviewPage{Items: items, Total: total}, nil
}
