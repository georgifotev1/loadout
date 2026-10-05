package specs

import (
	"context"
	"fmt"
	"slices"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gfotev/loadout/internal/platform/db"
	"github.com/gfotev/loadout/internal/platform/validate"
)

// Service is the specs module's public API.
// TODO(phase-6): ClearField for admins, used by the review queue UI with SetFields.
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
	return s.recompute(ctx, tx, in.ProductID, in.Category)
}

// SetFields stores admin values for some of a product's fields and recomputes its effective
// specs. Admin values win over the supplier's and survive syncs; fields not in values keep
// theirs. A product without specs (a peripheral, or not synced yet) is ErrNotFound, and a
// value that doesn't fit its field is ErrInvalidValue.
func (s *Service) SetFields(ctx context.Context, productID uuid.UUID, values map[string]any) (Status, error) {
	var status Status
	err := db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		category, err := s.store.lockProduct(ctx, tx, productID)
		if err != nil {
			return err
		}
		if err := schemas[category].validate(values); err != nil {
			return fmt.Errorf("%w: product %s: %v", ErrInvalidValue, productID, err)
		}
		if err := s.store.upsertFields(ctx, tx, productID, SourceAdmin, values); err != nil {
			return err
		}
		status, err = s.recompute(ctx, tx, productID, category)
		return err
	})
	return status, err
}

// recompute resolves a product's field values into its effective specs and stores them.
func (s *Service) recompute(ctx context.Context, tx pgx.Tx, productID uuid.UUID, category string) (Status, error) {
	rows, err := s.store.fields(ctx, tx, productID)
	if err != nil {
		return "", err
	}
	specs, missing, status := resolve(category, rows)
	if err := s.store.saveResolved(ctx, tx, productID, specs, missing, status); err != nil {
		return "", err
	}
	return status, nil
}

// Specs returns a product's effective specs, or ErrNotFound if it has none (a peripheral, or a
// product that hasn't been synced yet).
func (s *Service) Specs(ctx context.Context, productID uuid.UUID) (ProductSpecs, error) {
	return s.store.productSpecs(ctx, s.pool, productID)
}

// SpecsByIDs returns the effective specs of the given products, by product ID. Products without
// specs are left out.
func (s *Service) SpecsByIDs(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]ProductSpecs, error) {
	items, err := s.store.productSpecsByIDs(ctx, s.pool, ids)
	if err != nil {
		return nil, err
	}
	byID := make(map[uuid.UUID]ProductSpecs, len(items))
	for _, it := range items {
		byID[it.ProductID] = it
	}
	return byID, nil
}

// EligibleParts returns the products of a builder category whose specs are complete enough for
// the builder. A category without canonical specs has none.
func (s *Service) EligibleParts(ctx context.Context, category string) ([]ProductSpecs, error) {
	if _, ok := schemas[category]; !ok {
		return nil, nil
	}
	return s.store.eligible(ctx, s.pool, category)
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
