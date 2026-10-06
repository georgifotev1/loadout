package specs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/gfotev/loadout/internal/platform/db"
)

type store struct{}

// upsertProduct creates or refreshes a product's product_specs row. On an existing row the
// UPDATE takes its row lock, so two saves of one product run one after the other.
func (store) upsertProduct(ctx context.Context, q db.DBTX, in Input) error {
	query := `
		INSERT INTO specs.product_specs (product_id, category, product_name, spec_status)
		VALUES ($1, $2, $3, 'incomplete')
		ON CONFLICT (product_id) DO UPDATE SET
			category     = EXCLUDED.category,
			product_name = EXCLUDED.product_name`

	args := []any{
		in.ProductID,
		in.Category,
		in.Name,
	}
	if _, err := q.Exec(ctx, query, args...); err != nil {
		return fmt.Errorf("specs: upsert product %s: %w", in.ProductID, err)
	}
	return nil
}

// deleteProduct removes a product's specs and field values.
func (store) deleteProduct(ctx context.Context, q db.DBTX, productID uuid.UUID) error {
	if _, err := q.Exec(ctx, `DELETE FROM specs.product_specs WHERE product_id = $1`, productID); err != nil {
		return fmt.Errorf("specs: delete product %s: %w", productID, err)
	}
	return nil
}

// replaceSupplierFields makes the product's supplier rows exactly values. Admin rows are never
// touched. Unchanged values aren't rewritten.
func (st store) replaceSupplierFields(ctx context.Context, q db.DBTX, productID uuid.UUID, values map[string]any) error {
	names := append([]string{}, slices.Collect(maps.Keys(values))...) // never nil: NULL would delete nothing
	deleteQuery := `
		DELETE FROM specs.spec_fields
		WHERE product_id = $1 AND source = 'supplier' AND NOT (field = ANY($2::text[]))`
	if _, err := q.Exec(ctx, deleteQuery, productID, names); err != nil {
		return fmt.Errorf("specs: delete stale fields of %s: %w", productID, err)
	}
	return st.upsertFields(ctx, q, productID, SourceSupplier, values)
}

// upsertFields stores one source's values for the given fields, leaving its other fields
// alone. Unchanged values aren't rewritten.
func (store) upsertFields(ctx context.Context, q db.DBTX, productID uuid.UUID, src Source, values map[string]any) error {
	names := make([]string, 0, len(values))
	encoded := make([]string, 0, len(values))
	for field, v := range values {
		b, err := json.Marshal(v)
		if err != nil {
			return fmt.Errorf("specs: encode %s: %w", field, err)
		}
		names = append(names, field)
		encoded = append(encoded, string(b))
	}

	// unnest turns the parallel arrays into rows: one statement for all fields.
	query := `
		INSERT INTO specs.spec_fields AS f (product_id, field, source, value)
		SELECT $1, u.field, $4, u.value
		FROM unnest($2::text[], $3::text[]::jsonb[]) AS u(field, value)
		ON CONFLICT (product_id, field, source) DO UPDATE SET
			value      = EXCLUDED.value,
			updated_at = now()
		WHERE f.value IS DISTINCT FROM EXCLUDED.value`

	args := []any{
		productID,
		names,
		encoded,
		src,
	}
	if _, err := q.Exec(ctx, query, args...); err != nil {
		return fmt.Errorf("specs: upsert %s fields of %s: %w", src, productID, err)
	}
	return nil
}

// deleteFields removes one source's values for the given fields.
func (store) deleteFields(ctx context.Context, q db.DBTX, productID uuid.UUID, src Source, fields []string) error {
	if len(fields) == 0 {
		return nil
	}
	query := `DELETE FROM specs.spec_fields WHERE product_id = $1 AND source = $2 AND field = ANY($3::text[])`

	args := []any{
		productID,
		src,
		fields,
	}
	if _, err := q.Exec(ctx, query, args...); err != nil {
		return fmt.Errorf("specs: delete %s fields of %s: %w", src, productID, err)
	}
	return nil
}

// lockProduct returns a product's category and locks its product_specs row until the
// transaction ends, so a sync can't recompute the specs halfway through an admin change.
func (store) lockProduct(ctx context.Context, q db.DBTX, productID uuid.UUID) (string, error) {
	var category string
	err := q.QueryRow(ctx, `SELECT category FROM specs.product_specs WHERE product_id = $1 FOR UPDATE`, productID).Scan(&category)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return "", ErrNotFound
	case err != nil:
		return "", fmt.Errorf("specs: lock product %s: %w", productID, err)
	}
	return category, nil
}

// fields returns every source's values for a product.
func (store) fields(ctx context.Context, q db.DBTX, productID uuid.UUID) ([]FieldValue, error) {
	query := `
		SELECT field, source, value
		FROM specs.spec_fields
		WHERE product_id = $1
		ORDER BY field, source`

	rows, err := q.Query(ctx, query, productID)
	if err != nil {
		return nil, fmt.Errorf("specs: fields of %s: %w", productID, err)
	}
	fields, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (FieldValue, error) {
		var f FieldValue
		err := row.Scan(
			&f.Field,
			&f.Source,
			&f.Value,
		)
		return f, err
	})
	if err != nil {
		return nil, fmt.Errorf("specs: fields of %s: %w", productID, err)
	}
	return fields, nil
}

// saveResolved stores a product's effective specs and status. updated_at only moves when
// something changed, so the review queue's "oldest first" means "waiting longest".
func (store) saveResolved(ctx context.Context, q db.DBTX, productID uuid.UUID, specs map[string]any, missing []string, status Status) error {
	query := `
		UPDATE specs.product_specs SET
			specs       = $2,
			missing     = $3,
			spec_status = $4,
			updated_at  = now()
		WHERE product_id = $1
		  AND (specs, missing, spec_status) IS DISTINCT FROM ($2::jsonb, $3::text[], $4::text)`

	if missing == nil {
		missing = []string{} // the column is NOT NULL
	}
	args := []any{
		productID,
		specs,
		missing,
		status,
	}
	if _, err := q.Exec(ctx, query, args...); err != nil {
		return fmt.Errorf("specs: save specs of %s: %w", productID, err)
	}
	return nil
}

// productSpecsColumns and scanProductSpecs go together, like the catalog's productColumns.
const productSpecsColumns = `product_id, category, product_name, specs, missing, spec_status, builder_eligible, updated_at`

func scanProductSpecs(row pgx.Row, extra ...any) (ProductSpecs, error) {
	var p ProductSpecs
	dest := []any{
		&p.ProductID,
		&p.Category,
		&p.ProductName,
		&p.Specs,
		&p.Missing,
		&p.Status,
		&p.BuilderEligible,
		&p.UpdatedAt,
	}
	err := row.Scan(append(dest, extra...)...)
	return p, err
}

func (store) productSpecs(ctx context.Context, q db.DBTX, productID uuid.UUID) (ProductSpecs, error) {
	query := `SELECT ` + productSpecsColumns + ` FROM specs.product_specs WHERE product_id = $1`

	p, err := scanProductSpecs(q.QueryRow(ctx, query, productID))
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return ProductSpecs{}, ErrNotFound
	case err != nil:
		return ProductSpecs{}, fmt.Errorf("specs: product %s: %w", productID, err)
	}
	return p, nil
}

// reviewQueue returns one page of incomplete products, oldest first, and the number of matching
// products over all pages.
func (store) reviewQueue(ctx context.Context, q db.DBTX, f ReviewFilter) ([]ProductSpecs, int, error) {
	query := `
		SELECT ` + productSpecsColumns + `, count(*) OVER ()
		FROM specs.product_specs
		WHERE spec_status = 'incomplete'
		  AND ($1 = '' OR category = $1)
		ORDER BY updated_at, product_id
		LIMIT $2 OFFSET $3`

	args := []any{
		f.Category,
		f.PageSize,
		(f.Page - 1) * f.PageSize,
	}
	rows, err := q.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("specs: review queue: %w", err)
	}
	var total int
	items, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (ProductSpecs, error) {
		return scanProductSpecs(row, &total)
	})
	if err != nil {
		return nil, 0, fmt.Errorf("specs: review queue: %w", err)
	}
	return items, total, nil
}

func collectProductSpecs(rows pgx.Rows) ([]ProductSpecs, error) {
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (ProductSpecs, error) {
		return scanProductSpecs(row)
	})
}

// productSpecsByIDs returns the specs of the given products; products without specs are left out.
func (store) productSpecsByIDs(ctx context.Context, q db.DBTX, ids []uuid.UUID) ([]ProductSpecs, error) {
	query := `SELECT ` + productSpecsColumns + ` FROM specs.product_specs WHERE product_id = ANY($1)`

	rows, err := q.Query(ctx, query, ids)
	if err != nil {
		return nil, fmt.Errorf("specs: products by ids: %w", err)
	}
	items, err := collectProductSpecs(rows)
	if err != nil {
		return nil, fmt.Errorf("specs: products by ids: %w", err)
	}
	return items, nil
}

// eligible returns a category's builder-eligible products.
func (store) eligible(ctx context.Context, q db.DBTX, category string) ([]ProductSpecs, error) {
	query := `
		SELECT ` + productSpecsColumns + `
		FROM specs.product_specs
		WHERE category = $1 AND builder_eligible
		ORDER BY product_id`

	rows, err := q.Query(ctx, query, category)
	if err != nil {
		return nil, fmt.Errorf("specs: eligible %s: %w", category, err)
	}
	items, err := collectProductSpecs(rows)
	if err != nil {
		return nil, fmt.Errorf("specs: eligible %s: %w", category, err)
	}
	return items, nil
}
