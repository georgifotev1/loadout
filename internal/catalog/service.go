package catalog

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gfotev/loadout/internal/platform/validate"
	"github.com/gfotev/loadout/internal/specs"
	"github.com/gfotev/loadout/internal/supplier"
)

// Specs is what the catalog needs from the specs module. It's declared here, by the consumer,
// so the catalog depends on these methods rather than on everything *specs.Service does.
type Specs interface {
	// SaveTx stores a product's supplier spec values inside the catalog's sync transaction.
	SaveTx(ctx context.Context, tx pgx.Tx, in specs.Input) (specs.Status, error)
	// Specs returns a product's canonical specs, or specs.ErrNotFound.
	Specs(ctx context.Context, productID uuid.UUID) (specs.ProductSpecs, error)
	// SpecsByIDs returns the canonical specs of the products that have them.
	SpecsByIDs(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]specs.ProductSpecs, error)
	// FacetIndex loads a category's spec facet values, to filter and count a listing.
	FacetIndex(ctx context.Context, category string) (*specs.FacetIndex, error)
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

	parts, total, err := s.store.listProducts(ctx, s.pool, f.Category, f.PageSize, (f.Page-1)*f.PageSize)
	if err != nil {
		return ProductPage{}, err
	}
	products, err := s.withSpecs(ctx, parts)
	if err != nil {
		return ProductPage{}, err
	}
	return ProductPage{Products: products, Total: total}, nil
}

// Search returns one page of the visible products matching q, best first, and the facets over
// all matches. The text matches words in the name, brand and SKU, and tolerates typos in the
// name. With a builder category, the facets include its spec fields and q.Specs filters on them.
func (s *Service) Search(ctx context.Context, q SearchQuery) (SearchResult, error) {
	q.Text = strings.TrimSpace(q.Text)
	if q.Sort == "" {
		q.Sort = SortRelevance
	}

	var v validate.Validator
	v.Check(utf8.RuneCountInString(q.Text) <= MaxSearchTextLen, "q", "too_long")
	v.Check(q.Category == "" || IsCategory(q.Category), "category", "unknown")
	v.Check(len(q.Brands) <= MaxSearchBrands, "brand", "too_many")
	specFields := specs.FacetFields(q.Category)
	values := 0
	for field, want := range q.Specs {
		v.Check(slices.Contains(specFields, field), "spec", "unknown")
		values += len(want)
	}
	v.Check(values <= MaxSearchSpecs, "spec", "too_many")
	v.Check(q.MinCents >= 0, "price_min", "out_of_range")
	v.Check(q.MaxCents == 0 || q.MaxCents > q.MinCents, "price_max", "out_of_range")
	v.Check(slices.Contains([]Sort{SortRelevance, SortPriceAsc, SortPriceDesc}, q.Sort), "sort", "unknown")
	v.Check(q.Page >= 1, "page", "out_of_range")
	v.Check(q.PageSize >= 1 && q.PageSize <= MaxPageSize, "page_size", "out_of_range")
	if err := v.Err(); err != nil {
		return SearchResult{}, err
	}

	// The spec values live in the specs module, so it picks the products that match the spec
	// filters, and the queries here take those as a list of IDs.
	var (
		index   *specs.FacetIndex
		specIDs []uuid.UUID
	)
	if len(specFields) > 0 {
		var err error
		if index, err = s.specs.FacetIndex(ctx, q.Category); err != nil {
			return SearchResult{}, err
		}
		if len(q.Specs) > 0 {
			specIDs = index.Match(q.Specs)
		}
	}

	parts, total, err := s.store.searchProducts(ctx, s.pool, q, specIDs)
	if err != nil {
		return SearchResult{}, err
	}
	products, err := s.withSpecs(ctx, parts)
	if err != nil {
		return SearchResult{}, err
	}
	facets, err := s.store.searchFacets(ctx, s.pool, q, specIDs)
	if err != nil {
		return SearchResult{}, err
	}
	facets.Specs = []specs.Facet{}
	if index != nil {
		among, err := s.store.searchIDs(ctx, s.pool, q)
		if err != nil {
			return SearchResult{}, err
		}
		facets.Specs = index.Facets(among, q.Specs)
	}
	return SearchResult{Products: products, Total: total, Facets: facets}, nil
}

// withSpecs turns parts into summaries with their canonical specs, for the cards' key specs.
func (s *Service) withSpecs(ctx context.Context, parts []Part) ([]ProductSummary, error) {
	ids := make([]uuid.UUID, len(parts))
	for i, p := range parts {
		ids[i] = p.ID
	}
	byID, err := s.specs.SpecsByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make([]ProductSummary, len(parts))
	for i, p := range parts {
		out[i] = p.ProductSummary
		out[i].Specs = map[string]any{}
		if ps, ok := byID[p.ID]; ok {
			out[i].Specs = ps.Specs
		}
	}
	return out, nil
}

// PartsByIDs returns the visible products among ids, in no particular order. Hidden and unknown
// IDs are left out.
func (s *Service) PartsByIDs(ctx context.Context, ids []uuid.UUID) ([]Part, error) {
	return s.store.visibleParts(ctx, s.pool, ids, []string{})
}

// PartsBySlugs returns the visible products among slugs, in no particular order. Hidden and
// unknown slugs are left out.
func (s *Service) PartsBySlugs(ctx context.Context, slugs []string) ([]Part, error) {
	return s.store.visibleParts(ctx, s.pool, []uuid.UUID{}, slugs)
}

// ProductIDsBySKU maps a supplier's SKUs to our product IDs, active or not. Unknown SKUs are
// left out.
func (s *Service) ProductIDsBySKU(ctx context.Context, provider string, skus []string) (map[string]uuid.UUID, error) {
	return s.store.productIDsBySKU(ctx, s.pool, provider, skus)
}

// ProductByID returns a product as stored, active or not, for admins. An unknown ID is
// ErrNotFound.
func (s *Service) ProductByID(ctx context.Context, id uuid.UUID) (Product, error) {
	return s.store.productByID(ctx, s.pool, id)
}

// AdminProducts returns one page of every product, active or not and priced or not, by name,
// with its dealer price and the rule that priced it. Only admins may see this.
func (s *Service) AdminProducts(ctx context.Context, f AdminProductFilter) (AdminProductPage, error) {
	f.Text = strings.TrimSpace(f.Text)

	var v validate.Validator
	v.Check(utf8.RuneCountInString(f.Text) <= MaxSearchTextLen, "q", "too_long")
	v.Check(f.Category == "" || IsCategory(f.Category), "category", "unknown")
	v.Check(slices.Contains([]ProductStatus{"", StatusActive, StatusInactive}, f.Status), "status", "unknown")
	v.Check(f.Page >= 1, "page", "out_of_range")
	v.Check(f.PageSize >= 1 && f.PageSize <= MaxPageSize, "page_size", "out_of_range")
	if err := v.Err(); err != nil {
		return AdminProductPage{}, err
	}

	products, total, err := s.store.adminProducts(ctx, s.pool, f, f.PageSize, (f.Page-1)*f.PageSize)
	if err != nil {
		return AdminProductPage{}, err
	}
	return AdminProductPage{Products: products, Total: total}, nil
}

// SyncRuns returns one page of sync runs, newest first.
func (s *Service) SyncRuns(ctx context.Context, page, pageSize int) (SyncRunPage, error) {
	var v validate.Validator
	v.Check(page >= 1, "page", "out_of_range")
	v.Check(pageSize >= 1 && pageSize <= MaxPageSize, "page_size", "out_of_range")
	if err := v.Err(); err != nil {
		return SyncRunPage{}, err
	}

	runs, total, err := s.store.syncRuns(ctx, s.pool, pageSize, (page-1)*pageSize)
	if err != nil {
		return SyncRunPage{}, err
	}
	return SyncRunPage{Runs: runs, Total: total}, nil
}

// ProductCounts counts the active and inactive products.
func (s *Service) ProductCounts(ctx context.Context) (ProductCounts, error) {
	return s.store.productCounts(ctx, s.pool)
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
