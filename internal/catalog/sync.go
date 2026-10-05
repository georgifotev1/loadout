package catalog

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/gfotev/loadout/internal/money"
	"github.com/gfotev/loadout/internal/platform/db"
	"github.com/gfotev/loadout/internal/vali"
)

var (
	// ErrSyncInProgress is returned by Sync when another sync (in this or another process)
	// holds the sync lock.
	ErrSyncInProgress = errors.New("catalog: a sync is already running")

	// Per-product problems. The sync skips the product, counts an error and carries on.
	ErrUnsupportedCurrency = errors.New("catalog: unsupported currency (only EUR)")
	ErrUnknownCategory     = errors.New("catalog: unknown category")
	ErrInvalidProduct      = errors.New("catalog: invalid product")
	ErrNoMarkupRule        = errors.New("catalog: no markup rule matches")
)

const (
	// syncLockKey is the Postgres advisory lock key that keeps two syncs from overlapping.
	// Advisory lock keys are global to the database: keep every key the app uses distinct.
	syncLockKey int64 = 0x4c4f41444f5554 // "LOADOUT"

	// missingRunsThreshold is how many full syncs in a row a product can be absent from Vali
	// before it's deactivated. More than one, so a single bad response doesn't hide products.
	missingRunsThreshold = 3
)

// Sync pulls every product from the supplier and stores what changed. It holds an advisory
// lock for the whole run and writes each changed product in its own short transaction (prices
// and stock included), so a crash mid-run loses nothing that was already written and the next
// run simply picks up the rest.
//
// A product that can't be stored (not in euros, unknown category, no markup rule, a DB error)
// is skipped and counted in SyncRun.Errors. A failed fetch fails the whole run and skips
// missing-product detection, since an incomplete list would make products look gone.
func (s *Service) Sync(ctx context.Context) (SyncRun, error) {
	release, err := db.TryAdvisoryLock(ctx, s.pool, syncLockKey)
	if errors.Is(err, db.ErrLockHeld) {
		return SyncRun{}, ErrSyncInProgress
	}
	if err != nil {
		return SyncRun{}, err
	}
	defer release()

	runID, err := s.store.startSyncRun(ctx, s.pool)
	if err != nil {
		return SyncRun{}, err
	}
	run := SyncRun{ID: runID, Status: SyncSucceeded}
	start := time.Now()

	runErr := s.syncAll(ctx, &run)
	if runErr != nil {
		run.Status, run.ErrorMessage = SyncFailed, runErr.Error()
	}
	// Record the outcome even if ctx was cancelled (Ctrl-C, SIGTERM): that's when it matters.
	if err := s.store.finishSyncRun(context.WithoutCancel(ctx), s.pool, run); err != nil {
		return run, errors.Join(runErr, err)
	}

	s.logger.Info("sync finished",
		"run_id", run.ID,
		"status", run.Status,
		"fetched", run.Fetched,
		"changed", run.Changed,
		"errors", run.Errors,
		"duration", time.Since(start),
	)
	if runErr != nil {
		return run, fmt.Errorf("catalog: sync run %d: %w", run.ID, runErr)
	}
	return run, nil
}

// syncAll does the work of one run and fills in run's counts.
func (s *Service) syncAll(ctx context.Context, run *SyncRun) error {
	// Rules are loaded once: a rule edited mid-run applies from the next run.
	rules, err := s.store.markupRules(ctx, s.pool)
	if err != nil {
		return err
	}
	known, err := s.store.productStates(ctx, s.pool, ProviderVali)
	if err != nil {
		return err
	}

	var seen []string
	// Always a full list (zero since): missing-product detection needs every SKU.
	// TODO(phase-3): incremental syncs with since, plus a periodic full one.
	for raw, err := range s.supplier.ListProducts(ctx, time.Time{}) {
		if err != nil {
			return fmt.Errorf("fetch products: %w", err)
		}
		run.Fetched++
		seen = append(seen, raw.SKU)

		changed, err := s.syncProduct(ctx, raw, rules, known)
		switch {
		case ctx.Err() != nil:
			return ctx.Err()
		case err != nil:
			run.Errors++
			s.logger.Warn("sync: product skipped", "run_id", run.ID, "sku", raw.SKU, "err", err)
		case changed:
			run.Changed++
		}
	}

	// ASSUMPTION: Vali pages by offset. A product delisted mid-run can shift a page and make
	// another one look missing once; the threshold absorbs that.
	reactivated, err := s.store.markSeen(ctx, s.pool, ProviderVali, seen)
	if err != nil {
		return err
	}
	deactivated, err := s.store.markMissing(ctx, s.pool, ProviderVali, seen, missingRunsThreshold)
	if err != nil {
		return err
	}
	if reactivated > 0 || deactivated > 0 {
		s.logger.Info("sync: catalog membership changed", "run_id", run.ID,
			"reactivated", reactivated, "deactivated", deactivated)
	}
	return nil
}

// syncProduct stores one fetched product and reports whether anything was written.
func (s *Service) syncProduct(ctx context.Context, raw vali.RawProduct, rules []MarkupRule, known map[string]productState) (bool, error) {
	p, price, stock, err := fromRaw(raw)
	if err != nil {
		return false, err
	}
	rule, ok := ResolveRule(rules, p)
	if !ok {
		return false, fmt.Errorf("%w %s (%s, %s)", ErrNoMarkupRule, p.ProviderSKU, p.Category, p.Brand)
	}
	price.RetailCents = RetailPrice(price.DealerCents, rule)
	price.RuleID = &rule.ID

	prev, exists := known[p.ProviderSKU]
	if exists && prev.RawHash == p.RawHash && prev.CommercialHash == p.CommercialHash {
		// Vali's data didn't change, but a markup rule may have: store the recomputed price if
		// it differs. One statement needs no transaction.
		if prev.RetailCents == price.RetailCents && prev.RuleID != nil && *prev.RuleID == rule.ID {
			return false, nil
		}
		price.ProductID = prev.ID
		return s.store.upsertPrice(ctx, s.pool, price)
	}

	err = db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		id, written, err := s.store.upsertProduct(ctx, tx, p)
		if err != nil {
			return err
		}
		if !written {
			// Only possible if the row changed since productStates (the sync lock rules out
			// another sync, so someone edited it by hand): it already has these hashes.
			if !exists {
				return fmt.Errorf("catalog: product %s neither inserted nor found", p.ProviderSKU)
			}
			id = prev.ID
		}
		price.ProductID = id
		if _, err := s.store.upsertPrice(ctx, tx, price); err != nil {
			return err
		}
		// TODO(phase-2): specs.NormalizeTx(ctx, tx, ...) when the content hash changed.
		_, err = s.store.replaceStock(ctx, tx, id, stock)
		return err
	})
	if err != nil {
		return false, err
	}
	return true, nil
}

// fromRaw checks a supplier product and maps it onto ours.
func fromRaw(raw vali.RawProduct) (Product, Price, map[string]int, error) {
	category, err := checkRaw(raw)
	if err != nil {
		return Product{}, Price{}, nil, err
	}

	p := Product{
		Provider:       ProviderVali,
		ProviderSKU:    raw.SKU,
		EAN:            raw.EAN,
		MPN:            raw.ManufacturerCode,
		Brand:          raw.Brand,
		Name:           raw.Name,
		Slug:           slugFor(raw.SKU),
		Category:       category,
		URL:            raw.URL,
		RawSpecs:       raw.Specs,
		Images:         raw.Images,
		WarrantyMonths: valiWarrantyMonths(raw.Warranty),
		WarrantyText:   raw.Warranty,
	}
	price := Price{
		DealerCents: money.Cents(raw.DealerPriceCents),
		RRPCents:    money.Cents(raw.RRPCents),
		VATbp:       money.VATStandardBP,
	}
	stock := raw.Stock
	if stock == nil {
		stock = map[string]int{} // so "no stock key" and "empty stock" hash the same
	}
	p.RawHash = contentHash(p)
	p.CommercialHash = commercialHash(price, stock)
	return p, price, stock, nil
}

// checkRaw rejects what the database can't, and returns the product's canonical category.
func checkRaw(raw vali.RawProduct) (category string, err error) {
	if raw.Currency != money.Currency {
		return "", fmt.Errorf("%w: %s is priced in %q", ErrUnsupportedCurrency, raw.SKU, raw.Currency)
	}
	category, ok := valiCategory(raw)
	if !ok {
		return "", fmt.Errorf("%w: %s has category path %q", ErrUnknownCategory, raw.SKU, raw.CategoryPath)
	}
	switch {
	case strings.TrimSpace(raw.SKU) == "":
		return "", fmt.Errorf("%w: empty sku", ErrInvalidProduct)
	case strings.TrimSpace(raw.Name) == "", strings.TrimSpace(raw.Brand) == "":
		return "", fmt.Errorf("%w: %s has no name or brand", ErrInvalidProduct, raw.SKU)
	case raw.DealerPriceCents <= 0 || raw.RRPCents < 0:
		return "", fmt.Errorf("%w: %s has dealer price %d, rrp %d", ErrInvalidProduct, raw.SKU, raw.DealerPriceCents, raw.RRPCents)
	}
	for w, q := range raw.Stock {
		if q < 0 {
			return "", fmt.Errorf("%w: %s has stock %d in %q", ErrInvalidProduct, raw.SKU, q, w)
		}
	}
	return category, nil
}

// slugFor makes the product's URL slug from its SKU: unique, stable, ASCII. Sync sets it only
// on insert, so a nicer slug set later is never overwritten.
// TODO(phase-4): readable slugs from brand + model for SEO.
func slugFor(sku string) string {
	return strings.Trim(strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			return r
		case r >= 'A' && r <= 'Z':
			return r + ('a' - 'A')
		default:
			return '-'
		}
	}, sku), "-")
}

// contentHash covers every content field we store (not the slug, which never changes). It hashes
// our mapped values rather than Vali's payload, so a change in our mapping (say, a new category)
// also counts as a change.
func contentHash(p Product) string {
	return hashJSON(struct {
		EAN            string            `json:"ean"`
		MPN            string            `json:"mpn"`
		Brand          string            `json:"brand"`
		Name           string            `json:"name"`
		Category       string            `json:"category"`
		URL            string            `json:"url"`
		Specs          map[string]string `json:"specs"`
		Images         []string          `json:"images"`
		WarrantyMonths *int              `json:"warranty_months"`
		WarrantyText   string            `json:"warranty_text"`
	}{p.EAN, p.MPN, p.Brand, p.Name, p.Category, p.URL, p.RawSpecs, p.Images, p.WarrantyMonths, p.WarrantyText})
}

// commercialHash covers what changes on almost every sync: prices and stock. Currency is left
// out because it's always EUR (anything else was rejected).
func commercialHash(price Price, stock map[string]int) string {
	return hashJSON(struct {
		Dealer money.Cents    `json:"dealer_cents"`
		RRP    money.Cents    `json:"rrp_cents"`
		Stock  map[string]int `json:"stock"`
	}{price.DealerCents, price.RRPCents, stock})
}

// hashJSON is sha256 over v's JSON. encoding/json writes struct fields in declaration order
// and sorts map keys, so equal values always give the same bytes.
func hashJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("catalog: hash: %v", err)) // can't happen for strings, ints, maps and slices
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
