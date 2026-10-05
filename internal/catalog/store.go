package catalog

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/gfotev/loadout/internal/money"
	"github.com/gfotev/loadout/internal/platform/db"
)

type store struct{}

func (store) upsertProduct(ctx context.Context, q db.DBTX, p Product) (uuid.UUID, bool, error) {
	query := `
		INSERT INTO catalog.products AS p (
			provider, provider_sku, name, brand, slug, category, attributes, images, content_hash, commercial_hash
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10
		)
		ON CONFLICT ON CONSTRAINT products_provider_sku_key DO UPDATE SET
			name            = EXCLUDED.name,
			brand           = EXCLUDED.brand,
			category        = EXCLUDED.category,
			attributes      = EXCLUDED.attributes,
			images          = EXCLUDED.images,
			content_hash    = EXCLUDED.content_hash,
			commercial_hash = EXCLUDED.commercial_hash,
			updated_at      = now()
		WHERE p.content_hash    IS DISTINCT FROM EXCLUDED.content_hash
		   OR p.commercial_hash IS DISTINCT FROM EXCLUDED.commercial_hash
		RETURNING id`

	// pgx encodes maps and slices as JSON for jsonb parameters. Nil would become JSON null,
	// which the NOT NULL columns reject, so send empty values instead.
	attributes, images := p.Attributes, p.Images
	if attributes == nil {
		attributes = map[string]string{}
	}
	if images == nil {
		images = []string{}
	}

	args := []any{
		p.Provider,
		p.ProviderSKU,
		p.Name,
		p.Brand,
		p.Slug,
		p.Category,
		attributes,
		images,
		p.ContentHash,
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
// productColumns (catalog.products aliased as p) and scans the row with scanProduct, so the two
// can't drift apart.
const productColumns = `
	p.id, p.provider, p.provider_sku, p.name, p.brand, p.slug, p.category, p.attributes, p.images,
	p.content_hash, p.commercial_hash, p.missing_runs, p.active, p.created_at, p.updated_at`

// scanProduct scans a row that starts with productColumns; extra receives any columns the
// query selects after them.
func scanProduct(row pgx.Row, extra ...any) (Product, error) {
	var p Product
	dest := []any{
		&p.ID,
		&p.Provider,
		&p.ProviderSKU,
		&p.Name,
		&p.Brand,
		&p.Slug,
		&p.Category,
		&p.Attributes,
		&p.Images,
		&p.ContentHash,
		&p.CommercialHash,
		&p.MissingRuns,
		&p.Active,
		&p.CreatedAt,
		&p.UpdatedAt,
	}
	err := row.Scan(append(dest, extra...)...)
	return p, err
}

// productBySlug returns the product with that slug, active or not; the service decides what
// shoppers may see.
func (store) productBySlug(ctx context.Context, q db.DBTX, slug string) (Product, error) {
	query := `SELECT ` + productColumns + ` FROM catalog.products p WHERE p.slug = $1`

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
		INSERT INTO catalog.prices AS pr (product_id, dealer_cents, retail_cents, vat_bp, rule_id, computed_at)
		VALUES ($1, $2, $3, $4, $5, now())
		ON CONFLICT (product_id) DO UPDATE SET
			dealer_cents = EXCLUDED.dealer_cents,
			retail_cents = EXCLUDED.retail_cents,
			vat_bp       = EXCLUDED.vat_bp,
			rule_id      = EXCLUDED.rule_id,
			computed_at  = EXCLUDED.computed_at
		WHERE (pr.dealer_cents, pr.retail_cents, pr.vat_bp, pr.rule_id)
		      IS DISTINCT FROM
		      (EXCLUDED.dealer_cents, EXCLUDED.retail_cents, EXCLUDED.vat_bp, EXCLUDED.rule_id)`

	args := []any{
		p.ProductID,
		p.DealerCents,
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
		SELECT product_id, dealer_cents, retail_cents, vat_bp, rule_id, computed_at
		FROM catalog.prices
		WHERE product_id = $1`

	var p Price
	err := q.QueryRow(ctx, query, productID).Scan(
		&p.ProductID,
		&p.DealerCents,
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

// setStock sets a product's stock quantity and reports whether it changed. An unchanged
// quantity isn't rewritten, so updated_at keeps the time it last changed.
func (store) setStock(ctx context.Context, q db.DBTX, productID uuid.UUID, qty int) (bool, error) {
	query := `
		INSERT INTO catalog.stock AS s (product_id, qty)
		VALUES ($1, $2)
		ON CONFLICT (product_id) DO UPDATE SET
			qty        = EXCLUDED.qty,
			updated_at = now()
		WHERE s.qty <> EXCLUDED.qty`

	tag, err := q.Exec(ctx, query, productID, qty)
	if err != nil {
		return false, fmt.Errorf("catalog: set stock of %s: %w", productID, err)
	}
	return tag.RowsAffected() == 1, nil
}

// stockByProduct returns a product's stock quantity, or ErrNotFound if it has none recorded.
func (store) stockByProduct(ctx context.Context, q db.DBTX, productID uuid.UUID) (int, error) {
	var qty int
	err := q.QueryRow(ctx, `SELECT qty FROM catalog.stock WHERE product_id = $1`, productID).Scan(&qty)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return 0, ErrNotFound
	case err != nil:
		return 0, fmt.Errorf("catalog: stock of %s: %w", productID, err)
	}
	return qty, nil
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

// productState is what the sync needs to know about a stored product to tell whether a fetched
// one changed, and whether its price needs recomputing.
type productState struct {
	ID             uuid.UUID
	ContentHash    string
	CommercialHash string
	RetailCents    money.Cents // 0 and RuleID nil if the product has no price yet
	RuleID         *int64
}

// productStates returns provider SKU → state for every product of the provider. One query per
// sync run instead of one per product.
func (store) productStates(ctx context.Context, q db.DBTX, provider string) (map[string]productState, error) {
	query := `
		SELECT p.provider_sku, p.id, p.content_hash, p.commercial_hash, coalesce(pr.retail_cents, 0), pr.rule_id
		FROM catalog.products p
		LEFT JOIN catalog.prices pr ON pr.product_id = p.id
		WHERE p.provider = $1`

	rows, err := q.Query(ctx, query, provider)
	if err != nil {
		return nil, fmt.Errorf("catalog: product states: %w", err)
	}
	states := map[string]productState{}
	var (
		sku string
		st  productState
	)
	scan := []any{
		&sku,
		&st.ID,
		&st.ContentHash,
		&st.CommercialHash,
		&st.RetailCents,
		&st.RuleID,
	}
	_, err = pgx.ForEachRow(rows, scan, func() error {
		states[sku] = st
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("catalog: product states: %w", err)
	}
	return states, nil
}

// markSeen resets missing_runs and reactivates the provider's products in skus that a previous
// run counted as missing. Products that were never missing aren't rewritten.
func (store) markSeen(ctx context.Context, q db.DBTX, provider string, skus []string) (int64, error) {
	query := `
		UPDATE catalog.products
		SET missing_runs = 0, active = true, updated_at = now()
		WHERE provider = $1 AND provider_sku = ANY ($2::text[])
		  AND (missing_runs <> 0 OR NOT active)`

	args := []any{
		provider,
		skus,
	}

	tag, err := q.Exec(ctx, query, args...)
	if err != nil {
		return 0, fmt.Errorf("catalog: mark seen: %w", err)
	}
	return tag.RowsAffected(), nil
}

// markMissing bumps missing_runs for the provider's products NOT in seen, and deactivates those
// that have now been missing for threshold runs in a row. It returns how many it deactivated.
// Products are never deleted: order lines and saved builds still point at them.
func (store) markMissing(ctx context.Context, q db.DBTX, provider string, seen []string, threshold int) (int64, error) {
	// SET expressions see the row as it was, so missing_runs + 1 is the new count.
	query := `
		WITH bumped AS (
			UPDATE catalog.products
			SET missing_runs = missing_runs + 1,
			    active       = active AND missing_runs + 1 < $3,
			    updated_at   = now()
			WHERE provider = $1 AND provider_sku <> ALL ($2::text[])
			RETURNING missing_runs
		)
		SELECT count(*) FROM bumped WHERE missing_runs = $3`

	args := []any{
		provider,
		seen,
		threshold,
	}

	var n int64
	if err := q.QueryRow(ctx, query, args...).Scan(&n); err != nil {
		return 0, fmt.Errorf("catalog: mark missing: %w", err)
	}
	return n, nil
}

// visibleProducts is the FROM and WHERE of every shopper-facing query: a product is visible
// when it's active and has a price (an unpriced product can't be sold). Add new visibility
// conditions (e.g. hidden by an admin) here only.
const visibleProducts = `
	FROM catalog.products p
	JOIN catalog.prices pr ON pr.product_id = p.id
	WHERE p.active`

// stockQty is a product's stock quantity, for a query over visibleProducts.
const stockQty = `coalesce((SELECT s.qty FROM catalog.stock s WHERE s.product_id = p.id), 0)`

// listProducts returns one page of visible products by name, and the
// number of matching products over all pages. A page past the end has no rows, so its total
// is 0.
func (store) listProducts(ctx context.Context, q db.DBTX, category string, limit, offset int) ([]ProductSummary, int, error) {
	query := `
		SELECT p.slug, p.name, p.brand, p.category, coalesce(p.images ->> 0, ''), pr.retail_cents,
		       ` + stockQty + `, count(*) OVER ()
		` + visibleProducts + ` AND ($1 = '' OR p.category = $1)
		ORDER BY p.name, p.id
		LIMIT $2 OFFSET $3`

	args := []any{
		category,
		limit,
		offset,
	}

	rows, err := q.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("catalog: list products: %w", err)
	}
	var total int
	products, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (ProductSummary, error) {
		var p ProductSummary
		err := row.Scan(
			&p.Slug,
			&p.Name,
			&p.Brand,
			&p.Category,
			&p.Image,
			&p.RetailCents,
			&p.StockQty,
			&total,
		)
		return p, err
	})
	if err != nil {
		return nil, 0, fmt.Errorf("catalog: list products: %w", err)
	}
	return products, total, nil
}

// visibleParts returns the visible products whose ID is in ids or whose slug is in slugs.
func (store) visibleParts(ctx context.Context, q db.DBTX, ids []uuid.UUID, slugs []string) ([]Part, error) {
	query := `
		SELECT p.id, p.slug, p.name, p.brand, p.category, coalesce(p.images ->> 0, ''), pr.retail_cents,
		       ` + stockQty + `
		` + visibleProducts + ` AND (p.id = ANY ($1) OR p.slug = ANY ($2))`

	rows, err := q.Query(ctx, query, ids, slugs)
	if err != nil {
		return nil, fmt.Errorf("catalog: visible parts: %w", err)
	}
	parts, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (Part, error) {
		var p Part
		err := row.Scan(
			&p.ID,
			&p.Slug,
			&p.Name,
			&p.Brand,
			&p.Category,
			&p.Image,
			&p.RetailCents,
			&p.StockQty,
		)
		return p, err
	})
	if err != nil {
		return nil, fmt.Errorf("catalog: visible parts: %w", err)
	}
	return parts, nil
}

func (store) productIDsBySKU(ctx context.Context, q db.DBTX, provider string, skus []string) (map[string]uuid.UUID, error) {
	query := `SELECT provider_sku, id FROM catalog.products WHERE provider = $1 AND provider_sku = ANY ($2)`

	rows, err := q.Query(ctx, query, provider, skus)
	if err != nil {
		return nil, fmt.Errorf("catalog: product ids by sku: %w", err)
	}
	defer rows.Close()
	ids := make(map[string]uuid.UUID, len(skus))
	for rows.Next() {
		var sku string
		var id uuid.UUID
		if err := rows.Scan(
			&sku,
			&id,
		); err != nil {
			return nil, fmt.Errorf("catalog: product ids by sku: %w", err)
		}
		ids[sku] = id
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("catalog: product ids by sku: %w", err)
	}
	return ids, nil
}

// visibleProductBySlug returns a visible product with its retail price and stock in one
// query. Hidden products are ErrNotFound, the same as a slug that doesn't exist.
func (store) visibleProductBySlug(ctx context.Context, q db.DBTX, slug string) (ProductDetail, error) {
	query := `SELECT ` + productColumns + `, pr.retail_cents, ` + stockQty + `
		` + visibleProducts + ` AND p.slug = $1`

	var d ProductDetail
	p, err := scanProduct(q.QueryRow(ctx, query, slug), &d.RetailCents, &d.StockQty)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return ProductDetail{}, ErrNotFound
	case err != nil:
		return ProductDetail{}, fmt.Errorf("catalog: visible product %q: %w", slug, err)
	}
	d.Product = p
	return d, nil
}

// searchBase starts both search queries: the visible products that match the text, with one
// flag per filter. The page wants every flag; each facet ignores its own, so it can count the
// values the shopper could switch to. searchArgs supplies $1–$5.
//
// The text matches the search vector (whole words in name, brand and SKU) or, for typos and
// half-typed words, a trigram word similarity against the name ("rizen" finds "Ryzen"). Both
// are served by the indexes on catalog.products.
const searchBase = `
	WITH base AS (
		SELECT p.id, p.slug, p.name, p.brand, p.category, coalesce(p.images ->> 0, '') AS image,
		       pr.retail_cents, ` + stockQty + ` AS stock_qty,
		       CASE WHEN $1 = '' THEN 0
		            ELSE ts_rank(p.search, websearch_to_tsquery('simple', $1)) + word_similarity($1, p.name)
		       END AS rank,
		       ($2 = '' OR p.category = $2)                                          AS in_category,
		       (cardinality($3::text[]) = 0 OR p.brand = ANY ($3))                   AS in_brand,
		       (pr.retail_cents >= $4::bigint AND ($5::bigint = 0 OR pr.retail_cents < $5)) AS in_price
		` + visibleProducts + `
		  AND ($1 = '' OR p.search @@ websearch_to_tsquery('simple', $1) OR $1 <% p.name)
	)`

func searchArgs(q SearchQuery) []any {
	brands := q.Brands
	if brands == nil {
		brands = []string{} // nil would be NULL, and cardinality(NULL) isn't 0
	}
	return []any{
		q.Text,
		q.Category,
		brands,
		q.MinCents,
		q.MaxCents,
	}
}

// searchOrder maps each Sort to its ORDER BY. Name and id break ties so pages are stable.
var searchOrder = map[Sort]string{
	SortRelevance: `rank DESC, name, id`,
	SortPriceAsc:  `retail_cents, name, id`,
	SortPriceDesc: `retail_cents DESC, name, id`,
}

// searchProducts returns one page of matches and the number of matches over all pages. Like
// listProducts, a page past the end has no rows, so its total is 0.
func (store) searchProducts(ctx context.Context, q db.DBTX, sq SearchQuery) ([]ProductSummary, int, error) {
	order, ok := searchOrder[sq.Sort]
	if !ok {
		return nil, 0, fmt.Errorf("catalog: search: unknown sort %q", sq.Sort)
	}
	query := searchBase + `
		SELECT slug, name, brand, category, image, retail_cents, stock_qty, count(*) OVER ()
		FROM base
		WHERE in_category AND in_brand AND in_price
		ORDER BY ` + order + `
		LIMIT $6 OFFSET $7`

	args := append(searchArgs(sq),
		sq.PageSize,
		(sq.Page-1)*sq.PageSize,
	)

	rows, err := q.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("catalog: search: %w", err)
	}
	var total int
	products, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (ProductSummary, error) {
		var p ProductSummary
		err := row.Scan(
			&p.Slug,
			&p.Name,
			&p.Brand,
			&p.Category,
			&p.Image,
			&p.RetailCents,
			&p.StockQty,
			&total,
		)
		return p, err
	})
	if err != nil {
		return nil, 0, fmt.Errorf("catalog: search: %w", err)
	}
	return products, total, nil
}

// searchFacets counts the matches per category, brand and price bucket in one query. Each
// facet's WHERE leaves out its own flag.
func (store) searchFacets(ctx context.Context, q db.DBTX, sq SearchQuery) (Facets, error) {
	query := searchBase + `
		SELECT 'category', category, 0, count(*) FROM base WHERE in_brand AND in_price GROUP BY category
		UNION ALL
		SELECT 'brand', brand, 0, count(*) FROM base WHERE in_category AND in_price GROUP BY brand
		UNION ALL
		SELECT 'price', '', width_bucket(retail_cents, $6::bigint[]), count(*) FROM base
		WHERE in_category AND in_brand GROUP BY 3
		ORDER BY 1, 3, 4 DESC, 2`

	edges := make([]int64, len(priceEdges))
	for i, e := range priceEdges {
		edges[i] = int64(e)
	}
	args := append(searchArgs(sq), edges)

	rows, err := q.Query(ctx, query, args...)
	if err != nil {
		return Facets{}, fmt.Errorf("catalog: search facets: %w", err)
	}
	f := Facets{
		Categories: []FacetCount{},
		Brands:     []FacetCount{},
		Prices:     []PriceBucket{},
	}
	var (
		kind, value   string
		bucket, count int
	)
	scan := []any{
		&kind,
		&value,
		&bucket,
		&count,
	}
	_, err = pgx.ForEachRow(rows, scan, func() error {
		switch kind {
		case "category":
			f.Categories = append(f.Categories, FacetCount{Value: value, Count: count})
		case "brand":
			f.Brands = append(f.Brands, FacetCount{Value: value, Count: count})
		case "price":
			f.Prices = append(f.Prices, priceBucket(bucket, count))
		}
		return nil
	})
	if err != nil {
		return Facets{}, fmt.Errorf("catalog: search facets: %w", err)
	}
	return f, nil
}
