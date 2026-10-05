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
	"github.com/gfotev/loadout/internal/specs"
	"github.com/gfotev/loadout/internal/supplier"
)

var (
	// ErrSyncInProgress is returned by Sync when another sync (in this or another process)
	// holds the sync lock.
	ErrSyncInProgress = errors.New("catalog: a sync is already running")

	// Per-product problems. The sync skips the product, counts an error and carries on.
	ErrInvalidProduct = errors.New("catalog: invalid product")
	ErrNoMarkupRule   = errors.New("catalog: no markup rule matches")
)

const (
	// syncLockKey is the Postgres advisory lock key that keeps two syncs from overlapping.
	// Advisory lock keys are global to the database: keep every key the app uses distinct.
	syncLockKey int64 = 0x4c4f41444f5554 // "LOADOUT"

	// missingRunsThreshold is how many full syncs in a row a product can be absent from the
	// supplier before it's deactivated. More than one, so a single bad response doesn't hide products.
	missingRunsThreshold = 3
)

// Sync pulls every product from the supplier and stores what changed. It holds an advisory
// lock for the whole run and writes each changed product in its own short transaction (prices
// and stock included), so a crash mid-run loses nothing that was already written and the next
// run simply picks up the rest.
//
// A product that can't be stored (the adapter couldn't map it, it's invalid, no markup rule
// matches, a DB error) is skipped and counted in SyncRun.Errors. A failed fetch fails the whole run and skips
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
	provider := s.supplier.Name()
	// Rules are loaded once: a rule edited mid-run applies from the next run.
	rules, err := s.store.markupRules(ctx, s.pool)
	if err != nil {
		return err
	}
	known, err := s.store.productStates(ctx, s.pool, provider)
	if err != nil {
		return err
	}

	var seen []string
	for sp, err := range s.supplier.Products(ctx) {
		var itemErr *supplier.ItemError
		switch {
		case errors.As(err, &itemErr):
			// Still on the supplier's list, so not missing: it just couldn't be read this time.
			run.Fetched++
			run.Errors++
			seen = append(seen, itemErr.SKU)
			s.logger.Warn("sync: product skipped", "run_id", run.ID, "sku", itemErr.SKU, "err", itemErr.Err)
			continue
		case err != nil:
			return fmt.Errorf("fetch products: %w", err)
		}
		run.Fetched++
		seen = append(seen, sp.SKU)

		changed, err := s.syncProduct(ctx, provider, sp, rules, known)
		switch {
		case ctx.Err() != nil:
			return ctx.Err()
		case err != nil:
			run.Errors++
			s.logger.Warn("sync: product skipped", "run_id", run.ID, "sku", sp.SKU, "err", err)
		case changed:
			run.Changed++
		}
	}

	// A product delisted mid-run can shift a supplier's pages and make another one look missing
	// once; the threshold absorbs that.
	reactivated, err := s.store.markSeen(ctx, s.pool, provider, seen)
	if err != nil {
		return err
	}
	deactivated, err := s.store.markMissing(ctx, s.pool, provider, seen, missingRunsThreshold)
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
func (s *Service) syncProduct(ctx context.Context, provider string, sp supplier.Product, rules []MarkupRule, known map[string]productState) (bool, error) {
	if err := checkProduct(sp); err != nil {
		return false, err
	}
	p := Product{
		Provider:    provider,
		ProviderSKU: sp.SKU,
		Name:        sp.Name,
		Brand:       sp.Brand,
		Slug:        slugFor(sp.SKU),
		Category:    sp.Category,
		Attributes:  sp.Attributes,
		Images:      sp.Images,
	}
	price := Price{
		DealerCents: sp.CostCents,
		VATbp:       money.VATStandardBP,
	}
	var err error
	if p.ContentHash, err = contentHash(sp); err != nil {
		return false, fmt.Errorf("%w: %s: %v", ErrInvalidProduct, sp.SKU, err)
	}
	if p.CommercialHash, err = commercialHash(sp); err != nil {
		return false, fmt.Errorf("%w: %s: %v", ErrInvalidProduct, sp.SKU, err)
	}

	rule, ok := ResolveRule(rules, p)
	if !ok {
		return false, fmt.Errorf("%w %s (%s, %s)", ErrNoMarkupRule, p.ProviderSKU, p.Category, p.Brand)
	}
	price.RetailCents = RetailPrice(price.DealerCents, rule)
	price.RuleID = &rule.ID

	prev, exists := known[p.ProviderSKU]
	if exists && prev.ContentHash == p.ContentHash && prev.CommercialHash == p.CommercialHash {
		// The supplier's data didn't change, but a markup rule may have: store the recomputed
		// price if it differs. One statement needs no transaction.
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
		if _, err := s.store.setStock(ctx, tx, id, sp.StockQty); err != nil {
			return err
		}
		// Specs are content: a price or stock change leaves them alone.
		if written && (!exists || prev.ContentHash != p.ContentHash) {
			_, err = s.specs.SaveTx(ctx, tx, specs.Input{
				ProductID: id,
				Category:  p.Category,
				Name:      p.Name,
				Values:    sp.Specs,
			})
		}
		return err
	})
	if err != nil {
		return false, err
	}
	return true, nil
}

// checkProduct rejects what no supplier may send, whatever its format: the adapter has already
// turned the supplier's data into our shape.
func checkProduct(sp supplier.Product) error {
	switch {
	case strings.TrimSpace(sp.SKU) == "":
		return fmt.Errorf("%w: empty sku", ErrInvalidProduct)
	case strings.TrimSpace(sp.Name) == "", strings.TrimSpace(sp.Brand) == "":
		return fmt.Errorf("%w: %s has no name or brand", ErrInvalidProduct, sp.SKU)
	case !IsCategory(sp.Category):
		return fmt.Errorf("%w: %s has category %q", ErrInvalidProduct, sp.SKU, sp.Category)
	case sp.CostCents <= 0:
		return fmt.Errorf("%w: %s costs %d", ErrInvalidProduct, sp.SKU, sp.CostCents)
	case sp.StockQty < 0:
		return fmt.Errorf("%w: %s has stock %d", ErrInvalidProduct, sp.SKU, sp.StockQty)
	}
	return nil
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

// contentHash covers every content field (not the slug, which never changes), including the
// canonical specs, so a change in what the adapter extracts also counts as a change.
func contentHash(sp supplier.Product) (string, error) {
	return hashJSON(struct {
		Name       string            `json:"name"`
		Brand      string            `json:"brand"`
		Category   string            `json:"category"`
		Attributes map[string]string `json:"attributes"`
		Images     []string          `json:"images"`
		Specs      map[string]any    `json:"specs"`
	}{sp.Name, sp.Brand, sp.Category, sp.Attributes, sp.Images, sp.Specs})
}

// commercialHash covers what changes on almost every sync: cost and stock.
func commercialHash(sp supplier.Product) (string, error) {
	return hashJSON(struct {
		Cost  money.Cents `json:"cost_cents"`
		Stock int         `json:"stock"`
	}{sp.CostCents, sp.StockQty})
}

// hashJSON is sha256 over v's JSON. encoding/json writes struct fields in declaration order
// and sorts map keys, so equal values always give the same bytes. It fails only on values JSON
// can't hold (a NaN in an adapter's specs).
func hashJSON(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}
