package catalog

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/gfotev/loadout/internal/platform/db"
)

type store struct{}

func (store) upsertProduct(ctx context.Context, q db.DBTX, p Product) (uuid.UUID, bool, error) {
	query := `
		INSERT INTO catalog.products AS p (
			provider, provider_sku, ean, mpn, brand, name, slug, category, url,
			raw_specs, images, warranty_months, warranty_text, raw_hash, commercial_hash
		) VALUES (
			$1, $2, NULLIF($3, ''), NULLIF($4, ''), $5, $6, $7, $8, NULLIF($9, ''),
			$10, $11, $12, NULLIF($13, ''), $14, $15
		)
		ON CONFLICT ON CONSTRAINT products_provider_sku_key DO UPDATE SET
			ean             = EXCLUDED.ean,
			mpn             = EXCLUDED.mpn,
			brand           = EXCLUDED.brand,
			name            = EXCLUDED.name,
			category        = EXCLUDED.category,
			url             = EXCLUDED.url,
			raw_specs       = EXCLUDED.raw_specs,
			images          = EXCLUDED.images,
			warranty_months = EXCLUDED.warranty_months,
			warranty_text   = EXCLUDED.warranty_text,
			raw_hash        = EXCLUDED.raw_hash,
			commercial_hash = EXCLUDED.commercial_hash,
			updated_at      = now()
		WHERE p.raw_hash        IS DISTINCT FROM EXCLUDED.raw_hash
		   OR p.commercial_hash IS DISTINCT FROM EXCLUDED.commercial_hash
		RETURNING id`

	// pgx encodes maps and slices as JSON for jsonb parameters. Nil would become JSON null,
	// which the NOT NULL columns reject, so send empty values instead.
	specs, images := p.RawSpecs, p.Images
	if specs == nil {
		specs = map[string]string{}
	}
	if images == nil {
		images = []string{}
	}

	args := []any{
		p.Provider,
		p.ProviderSKU,
		p.EAN,
		p.MPN,
		p.Brand,
		p.Name,
		p.Slug,
		p.Category,
		p.URL,
		specs,
		images,
		p.WarrantyMonths,
		p.WarrantyText,
		p.RawHash,
		p.CommercialHash,
	}

	var id uuid.UUID
	err := q.QueryRow(ctx, query, args...).Scan(&id)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return uuid.Nil, false, nil // row exists and nothing changed
	case err != nil:
		return uuid.Nil, false, fmt.Errorf("catalog: upsert product %s/%s: %w", p.Provider, p.ProviderSKU, err)
	}
	return id, true, nil
}

// productColumns and scanProduct go together: every query that returns a full Product selects
// productColumns and scans the row with scanProduct, so the two can't drift apart.
const productColumns = `
	id, provider, provider_sku, coalesce(ean, ''), coalesce(mpn, ''), brand, name, slug, category,
	coalesce(url, ''), raw_specs, images, warranty_months, coalesce(warranty_text, ''),
	raw_hash, commercial_hash, missing_runs, active, featured, created_at, updated_at`

func scanProduct(row pgx.Row) (Product, error) {
	var p Product
	err := row.Scan(
		&p.ID,
		&p.Provider,
		&p.ProviderSKU,
		&p.EAN,
		&p.MPN,
		&p.Brand,
		&p.Name,
		&p.Slug,
		&p.Category,
		&p.URL,
		&p.RawSpecs,
		&p.Images,
		&p.WarrantyMonths,
		&p.WarrantyText,
		&p.RawHash,
		&p.CommercialHash,
		&p.MissingRuns,
		&p.Active,
		&p.Featured,
		&p.CreatedAt,
		&p.UpdatedAt,
	)
	return p, err
}

// productBySlug returns the product with that slug, active or not; the service decides what
// shoppers may see.
func (store) productBySlug(ctx context.Context, q db.DBTX, slug string) (Product, error) {
	query := `SELECT ` + productColumns + ` FROM catalog.products WHERE slug = $1`

	p, err := scanProduct(q.QueryRow(ctx, query, slug))
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return Product{}, ErrNotFound
	case err != nil:
		return Product{}, fmt.Errorf("catalog: product by slug %q: %w", slug, err)
	}
	return p, nil
}

// upsertPrice sets a product's price and reports whether a row was written. An identical price
// (same amounts, VAT and rule) is skipped, like an unchanged product in upsertProduct.
func (store) upsertPrice(ctx context.Context, q db.DBTX, p Price) (bool, error) {
	query := `
		INSERT INTO catalog.prices AS pr (product_id, dealer_cents, rrp_cents, retail_cents, vat_bp, rule_id, computed_at)
		VALUES ($1, $2, $3, $4, $5, $6, now())
		ON CONFLICT (product_id) DO UPDATE SET
			dealer_cents = EXCLUDED.dealer_cents,
			rrp_cents    = EXCLUDED.rrp_cents,
			retail_cents = EXCLUDED.retail_cents,
			vat_bp       = EXCLUDED.vat_bp,
			rule_id      = EXCLUDED.rule_id,
			computed_at  = EXCLUDED.computed_at
		WHERE (pr.dealer_cents, pr.rrp_cents, pr.retail_cents, pr.vat_bp, pr.rule_id)
		      IS DISTINCT FROM
		      (EXCLUDED.dealer_cents, EXCLUDED.rrp_cents, EXCLUDED.retail_cents, EXCLUDED.vat_bp, EXCLUDED.rule_id)`

	args := []any{
		p.ProductID,
		p.DealerCents,
		p.RRPCents,
		p.RetailCents,
		p.VATbp,
		p.RuleID,
	}

	tag, err := q.Exec(ctx, query, args...)
	if err != nil {
		return false, fmt.Errorf("catalog: upsert price %s: %w", p.ProductID, err)
	}
	return tag.RowsAffected() == 1, nil
}

func (store) priceByProduct(ctx context.Context, q db.DBTX, productID uuid.UUID) (Price, error) {
	query := `
		SELECT product_id, dealer_cents, rrp_cents, retail_cents, vat_bp, rule_id, computed_at
		FROM catalog.prices
		WHERE product_id = $1`

	var p Price
	err := q.QueryRow(ctx, query, productID).Scan(
		&p.ProductID,
		&p.DealerCents,
		&p.RRPCents,
		&p.RetailCents,
		&p.VATbp,
		&p.RuleID,
		&p.ComputedAt,
	)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return Price{}, ErrNotFound
	case err != nil:
		return Price{}, fmt.Errorf("catalog: price of %s: %w", productID, err)
	}
	return p, nil
}

// replaceStock makes the product's stock rows exactly qty (warehouse → quantity) and returns how
// many rows it wrote or deleted. Warehouses not in qty are deleted; unchanged quantities are not
// rewritten, so their observed_at keeps the time the quantity was first seen. It is one
// statement, so it's atomic even outside a transaction.
func (store) replaceStock(ctx context.Context, q db.DBTX, productID uuid.UUID, qty map[string]int) (int64, error) {
	// Both CTEs see the table as it was before the statement, and touch different rows (the
	// DELETE only rows not in the input, the INSERT only rows in it), so they don't conflict.
	query := `
		WITH deleted AS (
			DELETE FROM catalog.stock
			WHERE product_id = $1 AND warehouse <> ALL ($2::text[])
			RETURNING 1
		), written AS (
			INSERT INTO catalog.stock AS s (product_id, warehouse, qty, observed_at)
			SELECT $1, w, n, now() FROM unnest($2::text[], $3::int[]) AS input (w, n)
			ON CONFLICT (product_id, warehouse) DO UPDATE SET
				qty         = EXCLUDED.qty,
				observed_at = EXCLUDED.observed_at
			WHERE s.qty <> EXCLUDED.qty
			RETURNING 1
		)
		SELECT (SELECT count(*) FROM deleted) + (SELECT count(*) FROM written)`

	// unnest takes parallel arrays, so split the map into two slices in the same order.
	warehouses := make([]string, 0, len(qty))
	counts := make([]int32, 0, len(qty))
	for w, n := range qty {
		warehouses = append(warehouses, w)
		counts = append(counts, int32(n))
	}
	args := []any{
		productID,
		warehouses,
		counts,
	}

	var n int64
	if err := q.QueryRow(ctx, query, args...).Scan(&n); err != nil {
		return 0, fmt.Errorf("catalog: replace stock of %s: %w", productID, err)
	}
	return n, nil
}

// stockByProduct returns warehouse → quantity. A product with no rows has an empty map.
func (store) stockByProduct(ctx context.Context, q db.DBTX, productID uuid.UUID) (map[string]int, error) {
	query := `SELECT warehouse, qty FROM catalog.stock WHERE product_id = $1`

	rows, err := q.Query(ctx, query, productID)
	if err != nil {
		return nil, fmt.Errorf("catalog: stock of %s: %w", productID, err)
	}
	stock := map[string]int{}
	var (
		w string
		n int
	)
	_, err = pgx.ForEachRow(rows, []any{&w, &n}, func() error {
		stock[w] = n
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("catalog: stock of %s: %w", productID, err)
	}
	return stock, nil
}

// upsertMarkupRule creates the rule for r's target (scope + category/brand/sku) or updates the
// existing one, and returns its id. Empty target fields are stored as NULL; the
// markup_rules_scope_fields CHECK rejects combinations that don't fit the scope.
//
// TODO(phase-6): add a version column and return ErrEditConflict on a stale version (Greenlight's
// optimistic locking), once admins edit rules from two tabs.
func (store) upsertMarkupRule(ctx context.Context, q db.DBTX, r MarkupRule) (int64, error) {
	query := `
		INSERT INTO catalog.markup_rules (scope, category, brand, sku, markup_bp, min_margin_cents, rounding, priority)
		VALUES ($1, NULLIF($2, ''), NULLIF($3, ''), NULLIF($4, ''), $5, $6, $7, $8)
		ON CONFLICT ON CONSTRAINT markup_rules_target_key DO UPDATE SET
			markup_bp        = EXCLUDED.markup_bp,
			min_margin_cents = EXCLUDED.min_margin_cents,
			rounding         = EXCLUDED.rounding,
			priority         = EXCLUDED.priority,
			updated_at       = now()
		RETURNING id`

	args := []any{
		r.Scope,
		r.Category,
		r.Brand,
		r.SKU,
		r.MarkupBP,
		r.MinMarginCents,
		r.Rounding,
		r.Priority,
	}

	var id int64
	if err := q.QueryRow(ctx, query, args...).Scan(&id); err != nil {
		return 0, fmt.Errorf("catalog: upsert markup rule: %w", err)
	}
	return id, nil
}

// markupRules returns every rule. The table is small (tens of rows), so pricing loads them all
// once per sync and resolves in Go.
func (store) markupRules(ctx context.Context, q db.DBTX) ([]MarkupRule, error) {
	query := `
		SELECT id, scope, coalesce(category, ''), coalesce(brand, ''), coalesce(sku, ''),
		       markup_bp, min_margin_cents, rounding, priority, created_at, updated_at
		FROM catalog.markup_rules
		ORDER BY id`

	rows, err := q.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("catalog: markup rules: %w", err)
	}
	rules, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (MarkupRule, error) {
		var r MarkupRule
		err := row.Scan(
			&r.ID,
			&r.Scope,
			&r.Category,
			&r.Brand,
			&r.SKU,
			&r.MarkupBP,
			&r.MinMarginCents,
			&r.Rounding,
			&r.Priority,
			&r.CreatedAt,
			&r.UpdatedAt,
		)
		return r, err
	})
	if err != nil {
		return nil, fmt.Errorf("catalog: markup rules: %w", err)
	}
	return rules, nil
}

// deleteMarkupRule deletes a rule. Prices that used it keep their amounts and get rule_id NULL.
func (store) deleteMarkupRule(ctx context.Context, q db.DBTX, id int64) error {
	query := `DELETE FROM catalog.markup_rules WHERE id = $1`

	tag, err := q.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("catalog: delete markup rule %d: %w", id, err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// startSyncRun records the start of a sync and returns its id.
func (store) startSyncRun(ctx context.Context, q db.DBTX) (int64, error) {
	query := `INSERT INTO catalog.sync_runs DEFAULT VALUES RETURNING id`

	var id int64
	if err := q.QueryRow(ctx, query).Scan(&id); err != nil {
		return 0, fmt.Errorf("catalog: start sync run: %w", err)
	}
	return id, nil
}

// finishSyncRun stores a running sync's counts and final status (succeeded or failed). It
// returns ErrSyncRunNotRunning if the run doesn't exist or was already finished, so a run can't
// be finished twice.
func (store) finishSyncRun(ctx context.Context, q db.DBTX, r SyncRun) error {
	query := `
		UPDATE catalog.sync_runs
		SET finished_at = now(), fetched = $2, changed = $3, errors = $4, status = $5,
		    error_message = NULLIF($6, '')
		WHERE id = $1 AND status = 'running'`

	args := []any{
		r.ID,
		r.Fetched,
		r.Changed,
		r.Errors,
		r.Status,
		r.ErrorMessage,
	}

	tag, err := q.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("catalog: finish sync run %d: %w", r.ID, err)
	}
	if tag.RowsAffected() == 0 {
		return ErrSyncRunNotRunning
	}
	return nil
}

func (store) syncRun(ctx context.Context, q db.DBTX, id int64) (SyncRun, error) {
	query := `
		SELECT id, started_at, finished_at, fetched, changed, errors, status, coalesce(error_message, '')
		FROM catalog.sync_runs
		WHERE id = $1`

	var r SyncRun
	err := q.QueryRow(ctx, query, id).Scan(
		&r.ID,
		&r.StartedAt,
		&r.FinishedAt,
		&r.Fetched,
		&r.Changed,
		&r.Errors,
		&r.Status,
		&r.ErrorMessage,
	)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return SyncRun{}, ErrNotFound
	case err != nil:
		return SyncRun{}, fmt.Errorf("catalog: sync run %d: %w", id, err)
	}
	return r, nil
}
