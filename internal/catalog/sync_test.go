package catalog

import (
	"context"
	"errors"
	"iter"
	"log/slog"
	"maps"
	"slices"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gfotev/loadout/fixtures"
	"github.com/gfotev/loadout/internal/platform/db"
	"github.com/gfotev/loadout/internal/platform/db/dbtest"
	"github.com/gfotev/loadout/internal/specs"
	"github.com/gfotev/loadout/internal/supplier"
	"github.com/gfotev/loadout/internal/vali"
)

// loadFixtures returns the fixtures as the vali adapter maps them: realistic supplier data.
func loadFixtures(t *testing.T) []supplier.Product {
	t.Helper()
	products, err := vali.FixtureProducts(fixtures.Vali)
	if err != nil {
		t.Fatal(err)
	}
	return products
}

// syncEnv is a catalog service over a fresh database, with a fake supplier whose products a
// test can change between runs.
type syncEnv struct {
	pool *pgxpool.Pool
	svc  *Service
	src  *supplier.Fake
}

func newSyncEnv(t *testing.T) *syncEnv {
	t.Helper()
	pool := dbtest.New(t)
	return newSyncEnvWith(t, pool, specs.NewService(pool))
}

func newSyncEnvWith(t *testing.T, pool *pgxpool.Pool, sp Specs) *syncEnv {
	t.Helper()
	src := &supplier.Fake{Items: loadFixtures(t)}
	env := &syncEnv{pool: pool, svc: NewService(pool, src, sp, slog.New(slog.DiscardHandler)), src: src}
	// A plain rule instead of the seeded one, so expected prices are easy to compute.
	env.setDefaultRule(t, 2000)
	return env
}

func (e *syncEnv) setDefaultRule(t *testing.T, markupBP int) {
	t.Helper()
	_, err := store{}.upsertMarkupRule(context.Background(), e.pool, MarkupRule{Scope: ScopeDefault, MarkupBP: markupBP, Rounding: RoundNone})
	if err != nil {
		t.Fatal(err)
	}
}

func (e *syncEnv) mustSync(t *testing.T) SyncRun {
	t.Helper()
	run, err := e.svc.Sync(context.Background())
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	return run
}

// edit replaces the fake's product sku with change applied to a copy, so earlier runs' data
// isn't touched.
func (e *syncEnv) edit(t *testing.T, sku string, change func(*supplier.Product)) {
	t.Helper()
	e.src.Items = slices.Clone(e.src.Items)
	i := slices.IndexFunc(e.src.Items, func(p supplier.Product) bool { return p.SKU == sku })
	if i < 0 {
		t.Fatalf("no product %s", sku)
	}
	p := e.src.Items[i]
	p.Attributes, p.Specs = maps.Clone(p.Attributes), maps.Clone(p.Specs)
	change(&p)
	e.src.Items[i] = p
}

// versions returns every catalog and specs row's xmin (the transaction that last wrote it), so a
// test can tell whether a run rewrote anything at all.
func (e *syncEnv) versions(t *testing.T) map[string]string {
	t.Helper()
	query := `
		SELECT 'product:' || id, xmin::text FROM catalog.products
		UNION ALL SELECT 'price:' || product_id, xmin::text FROM catalog.prices
		UNION ALL SELECT 'stock:' || product_id, xmin::text FROM catalog.stock
		UNION ALL SELECT 'specs:' || product_id, xmin::text FROM specs.product_specs`
	rows, err := e.pool.Query(context.Background(), query)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	var k, v string
	if _, err := pgx.ForEachRow(rows, []any{&k, &v}, func() error {
		got[k] = v
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return got
}

func (e *syncEnv) product(t *testing.T, sku string) Product {
	t.Helper()
	p, err := store{}.productBySlug(context.Background(), e.pool, slugFor(sku))
	if err != nil {
		t.Fatalf("product %s: %v", sku, err)
	}
	return p
}

func (e *syncEnv) price(t *testing.T, sku string) Price {
	t.Helper()
	pr, err := store{}.priceByProduct(context.Background(), e.pool, e.product(t, sku).ID)
	if err != nil {
		t.Fatal(err)
	}
	return pr
}

func (e *syncEnv) count(t *testing.T, query string) int {
	t.Helper()
	var n int
	if err := e.pool.QueryRow(context.Background(), query).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func changedKeys(before, after map[string]string) []string {
	var keys []string
	for k, v := range after {
		if before[k] != v {
			keys = append(keys, k)
		}
	}
	for k := range before {
		if _, ok := after[k]; !ok {
			keys = append(keys, k)
		}
	}
	slices.Sort(keys)
	return keys
}

const ryzen = "AMD-AM4-R3-RYZ-4100-BOX"

// without returns a copy of products without sku.
func without(products []supplier.Product, sku string) []supplier.Product {
	return slices.DeleteFunc(slices.Clone(products), func(p supplier.Product) bool { return p.SKU == sku })
}

func TestSyncFixtures(t *testing.T) {
	env := newSyncEnv(t)
	n := len(env.src.Items)

	first := env.mustSync(t)
	if first.Status != SyncSucceeded || first.Fetched != n || first.Changed != n || first.Errors != 0 {
		t.Fatalf("first run = %+v, want %d fetched and changed", first, n)
	}

	p := env.product(t, ryzen)
	if p.Provider != "fake" || p.Category != "cpu" || p.Slug != "amd-am4-r3-ryz-4100-box" || p.Attributes["Сокет"] != "AM4" {
		t.Errorf("stored product = %+v", p)
	}
	price := env.price(t, ryzen)
	// 52.35 € + 20% = 62.82 €
	if price.DealerCents != 5235 || price.RetailCents != 6282 || price.VATbp != 2000 || price.RuleID == nil {
		t.Errorf("price = %+v", price)
	}
	if qty, _ := (store{}).stockByProduct(context.Background(), env.pool, p.ID); qty != 19 {
		t.Errorf("stock = %d, want 19", qty)
	}

	t.Run("a second run with no changes writes nothing", func(t *testing.T) {
		before := env.versions(t)
		run := env.mustSync(t)
		if run.Changed != 0 || run.Fetched != n || run.Errors != 0 {
			t.Errorf("run = %+v, want 0 changed", run)
		}
		if diff := changedKeys(before, env.versions(t)); len(diff) != 0 {
			t.Errorf("rows rewritten: %v", diff)
		}
	})

	t.Run("a price change updates exactly one product, specs untouched", func(t *testing.T) {
		env.edit(t, ryzen, func(p *supplier.Product) { p.CostCents = 5000 })

		before := env.versions(t)
		if run := env.mustSync(t); run.Changed != 1 {
			t.Errorf("changed = %d, want 1", run.Changed)
		}
		id := env.product(t, ryzen).ID.String()
		want := []string{"price:" + id, "product:" + id} // the product row holds commercial_hash
		if diff := changedKeys(before, env.versions(t)); !slices.Equal(diff, want) {
			t.Errorf("rows rewritten: %v, want %v", diff, want)
		}
		if got := env.price(t, ryzen); got.DealerCents != 5000 || got.RetailCents != 6000 {
			t.Errorf("price = %+v, want 50.00 € → 60.00 €", got)
		}
	})

	t.Run("a stock change only touches that product's stock", func(t *testing.T) {
		env.edit(t, ryzen, func(p *supplier.Product) { p.StockQty = 16 })

		before := env.versions(t)
		if run := env.mustSync(t); run.Changed != 1 {
			t.Errorf("changed = %d, want 1", run.Changed)
		}
		id := env.product(t, ryzen).ID.String()
		want := []string{"product:" + id, "stock:" + id}
		if diff := changedKeys(before, env.versions(t)); !slices.Equal(diff, want) {
			t.Errorf("rows rewritten: %v, want %v", diff, want)
		}
	})

	t.Run("a content change rewrites the product and its specs", func(t *testing.T) {
		env.edit(t, ryzen, func(p *supplier.Product) { p.Name += " (new)" })

		before := env.versions(t)
		if run := env.mustSync(t); run.Changed != 1 {
			t.Errorf("changed = %d, want 1", run.Changed)
		}
		id := env.product(t, ryzen).ID.String()
		want := []string{"product:" + id, "specs:" + id}
		if diff := changedKeys(before, env.versions(t)); !slices.Equal(diff, want) {
			t.Errorf("rows rewritten: %v, want %v", diff, want)
		}
	})

	t.Run("a markup rule change reprices without touching products", func(t *testing.T) {
		env.setDefaultRule(t, 2500)
		run := env.mustSync(t)
		if run.Changed != n {
			t.Errorf("changed = %d, want all %d repriced", run.Changed, n)
		}
		if got := env.price(t, ryzen); got.RetailCents != 6250 {
			t.Errorf("retail = %d, want 6250 (50.00 € + 25%%)", got.RetailCents)
		}
	})
}

func TestSyncStoresSpecs(t *testing.T) {
	env := newSyncEnv(t)
	env.mustSync(t)

	builder := 0
	for _, p := range env.src.Items {
		if slices.Contains(specs.Categories(), p.Category) {
			builder++
		}
	}
	if got := env.count(t, `SELECT count(*) FROM specs.product_specs`); got != builder {
		t.Errorf("%d products have specs, want the %d builder fixtures", got, builder)
	}
	// The vali adapter extracts no specs yet, so every builder part waits in the review queue.
	if got := env.count(t, `SELECT count(*) FROM specs.product_specs WHERE spec_status = 'incomplete'`); got != builder {
		t.Errorf("%d incomplete, want all %d", got, builder)
	}

	t.Run("values from the supplier reach specs", func(t *testing.T) {
		env.edit(t, ryzen, func(p *supplier.Product) {
			p.Specs = map[string]any{"socket": "AM4", "tdp_w": 65, "igpu": false, "cooler_included": true}
		})
		env.mustSync(t)
		ps, err := env.svc.specs.Specs(context.Background(), env.product(t, ryzen).ID)
		if err != nil {
			t.Fatal(err)
		}
		if ps.Status != specs.StatusApproved || ps.Specs["socket"] != "AM4" {
			t.Errorf("specs = %+v", ps)
		}
		detail, err := env.svc.Product(context.Background(), slugFor(ryzen))
		if err != nil {
			t.Fatal(err)
		}
		if detail.Specs["tdp_w"] != 65.0 {
			t.Errorf("product detail specs = %v", detail.Specs)
		}
	})

	t.Run("an invalid value from the adapter skips the product", func(t *testing.T) {
		env.edit(t, ryzen, func(p *supplier.Product) { p.Specs["tdp_w"] = "65 W" })
		if run := env.mustSync(t); run.Errors != 1 || run.Changed != 0 {
			t.Errorf("run = %+v, want 1 error", run)
		}
	})
}

func TestSyncSkipsBadProducts(t *testing.T) {
	env := newSyncEnv(t)
	good := env.src.Items[0]

	noName := good
	noName.SKU, noName.Name = "NO-NAME", " "
	badCategory := good
	badCategory.SKU, badCategory.Category = "BAD-CATEGORY", "toaster"
	free := good
	free.SKU, free.CostCents = "NO-PRICE", 0
	negative := good
	negative.SKU, negative.StockQty = "NEGATIVE-STOCK", -1
	env.src.Items = []supplier.Product{noName, good, badCategory, free, negative}
	env.src.ItemErrors = []*supplier.ItemError{{SKU: "UNMAPPABLE", Err: errors.New("unknown category")}}

	run := env.mustSync(t)
	if run.Status != SyncSucceeded || run.Fetched != 6 || run.Changed != 1 || run.Errors != 5 {
		t.Fatalf("run = %+v, want 1 stored and 5 errors", run)
	}
	if n := env.count(t, `SELECT count(*) FROM catalog.products`); n != 1 {
		t.Errorf("%d products stored, want only the good one", n)
	}
	for _, p := range []supplier.Product{noName, badCategory, free, negative} {
		if err := checkProduct(p); !errors.Is(err, ErrInvalidProduct) {
			t.Errorf("%s: err = %v, want ErrInvalidProduct", p.SKU, err)
		}
	}
}

func TestSyncWithoutMarkupRule(t *testing.T) {
	env := newSyncEnv(t)
	if _, err := env.pool.Exec(context.Background(), `DELETE FROM catalog.markup_rules`); err != nil {
		t.Fatal(err)
	}
	env.src.Items = env.src.Items[:3]

	run := env.mustSync(t)
	if run.Errors != 3 || run.Changed != 0 {
		t.Errorf("run = %+v, want every product skipped", run)
	}
}

func TestSyncDeactivatesMissingProducts(t *testing.T) {
	env := newSyncEnv(t)
	all := env.src.Items
	env.mustSync(t)

	// Ryzen disappears from the supplier.
	env.src.Items = without(all, ryzen)
	for i := 1; i <= missingRunsThreshold; i++ {
		env.mustSync(t)
		p := env.product(t, ryzen)
		wantActive := i < missingRunsThreshold
		if p.MissingRuns != i || p.Active != wantActive {
			t.Fatalf("after %d missing runs: missing_runs=%d active=%v, want %d %v", i, p.MissingRuns, p.Active, i, wantActive)
		}
	}
	if _, err := env.svc.Product(context.Background(), slugFor(ryzen)); !errors.Is(err, ErrNotFound) {
		t.Errorf("inactive product is visible: err = %v", err)
	}

	// It comes back.
	env.src.Items = all
	env.mustSync(t)
	if p := env.product(t, ryzen); p.MissingRuns != 0 || !p.Active {
		t.Errorf("after it came back: missing_runs=%d active=%v", p.MissingRuns, p.Active)
	}

	t.Run("an item the adapter can't read isn't missing", func(t *testing.T) {
		env.src.Items = without(all, ryzen)
		env.src.ItemErrors = []*supplier.ItemError{{SKU: ryzen, Err: errors.New("unreadable")}}
		env.mustSync(t)
		if p := env.product(t, ryzen); p.MissingRuns != 0 || !p.Active {
			t.Errorf("missing_runs=%d active=%v", p.MissingRuns, p.Active)
		}
		env.src.ItemErrors = nil
	})

	t.Run("a run where it's present resets the count", func(t *testing.T) {
		env.src.Items = without(all, ryzen)
		env.mustSync(t)
		env.src.Items = all
		env.mustSync(t)
		if p := env.product(t, ryzen); p.MissingRuns != 0 || !p.Active {
			t.Errorf("missing_runs=%d active=%v", p.MissingRuns, p.Active)
		}
	})
}

func TestSyncFetchFailure(t *testing.T) {
	env := newSyncEnv(t)
	all := env.src.Items
	env.mustSync(t)

	// The supplier fails after the first 5 products: the run fails, those 5 are still synced,
	// and no product is counted as missing because the list was incomplete.
	env.src.Items, env.src.Err = all[:5], errors.New("supplier: 503")
	run, err := env.svc.Sync(context.Background())
	if err == nil {
		t.Fatal("want an error")
	}
	if run.Status != SyncFailed || run.Fetched != 5 || run.ErrorMessage == "" {
		t.Errorf("run = %+v", run)
	}
	stored, _ := store{}.syncRun(context.Background(), env.pool, run.ID)
	if stored.Status != SyncFailed || stored.FinishedAt == nil {
		t.Errorf("stored run = %+v", stored)
	}
	if missing := env.count(t, `SELECT count(*) FROM catalog.products WHERE missing_runs > 0`); missing != 0 {
		t.Errorf("%d products marked missing after a failed fetch", missing)
	}
}

// cancelAfter passes through n products from the fake and then cancels the run's context,
// like a SIGTERM arriving mid-sync.
type cancelAfter struct {
	*supplier.Fake
	n      int
	cancel context.CancelFunc
}

func (c *cancelAfter) Products(ctx context.Context) iter.Seq2[supplier.Product, error] {
	return func(yield func(supplier.Product, error) bool) {
		i := 0
		for p, err := range c.Fake.Products(ctx) {
			if i == c.n {
				c.cancel()
				err = ctx.Err()
			}
			i++
			if !yield(p, err) || err != nil {
				return
			}
		}
	}
}

func TestSyncCancelledMidRun(t *testing.T) {
	env := newSyncEnv(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	env.svc.supplier = &cancelAfter{Fake: env.src, n: 3, cancel: cancel}

	run, err := env.svc.Sync(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want Canceled", err)
	}
	// Recorded as failed even though ctx was cancelled, rather than left "running".
	stored, _ := store{}.syncRun(context.Background(), env.pool, run.ID)
	if stored.Status != SyncFailed || stored.FinishedAt == nil || stored.Fetched != 3 {
		t.Errorf("stored run = %+v, want failed after 3", stored)
	}
	// What was written before the cancel stays; the next run finishes the job.
	if n := env.count(t, `SELECT count(*) FROM catalog.products`); n != 3 {
		t.Errorf("%d products stored, want the 3 synced before the cancel", n)
	}
	// The lock was released, so a new run can start.
	env.svc.supplier = env.src
	if run := env.mustSync(t); run.Changed != len(env.src.Items)-3 {
		t.Errorf("next run changed %d, want the remaining %d", run.Changed, len(env.src.Items)-3)
	}
}

func TestSyncRefusesToOverlap(t *testing.T) {
	env := newSyncEnv(t)

	// Another process holds the sync lock.
	release, err := db.TryAdvisoryLock(context.Background(), env.pool, syncLockKey)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := env.svc.Sync(context.Background()); !errors.Is(err, ErrSyncInProgress) {
		t.Fatalf("err = %v, want ErrSyncInProgress", err)
	}
	if runs := env.count(t, `SELECT count(*) FROM catalog.sync_runs`); runs != 0 {
		t.Errorf("%d sync runs recorded, want none", runs)
	}

	release()
	env.mustSync(t)
}

// failingSpecs fails every save.
type failingSpecs struct{}

func (failingSpecs) SaveTx(context.Context, pgx.Tx, specs.Input) (specs.Status, error) {
	return "", errors.New("specs failed")
}

func (failingSpecs) Specs(context.Context, uuid.UUID) (specs.ProductSpecs, error) {
	return specs.ProductSpecs{}, specs.ErrNotFound
}

// Saving specs is the last step of a product's sync transaction: when it fails, the product,
// its price and its stock must not be stored either.
func TestSyncRollsBackWhenSpecsFail(t *testing.T) {
	pool := dbtest.New(t)
	env := newSyncEnvWith(t, pool, failingSpecs{})

	run := env.mustSync(t)
	if run.Errors != len(env.src.Items) || run.Changed != 0 {
		t.Errorf("run = %+v, want every product to fail", run)
	}
	if v := env.versions(t); len(v) != 0 {
		t.Errorf("rows left behind: %v", v)
	}
}

func TestSlugFor(t *testing.T) {
	for in, want := range map[string]string{
		"AMD-AM4-R3-RYZ-4100-BOX": "amd-am4-r3-ryz-4100-box",
		"KIN-RAM-KF432C16RB2A-8":  "kin-ram-kf432c16rb2a-8",
		"A.B_C":                   "a-b-c",
		"-X-":                     "x",
	} {
		if got := slugFor(in); got != want {
			t.Errorf("slugFor(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestHashes(t *testing.T) {
	base := loadFixtures(t)[0]
	hashes := func(change func(*supplier.Product)) (content, commercial string) {
		p := base
		p.Attributes, p.Specs = maps.Clone(base.Attributes), maps.Clone(base.Specs)
		change(&p)
		content, err := contentHash(p)
		if err != nil {
			t.Fatal(err)
		}
		commercial, err = commercialHash(p)
		if err != nil {
			t.Fatal(err)
		}
		return content, commercial
	}
	baseContent, baseCommercial := hashes(func(*supplier.Product) {})

	tests := []struct {
		name                          string
		change                        func(*supplier.Product)
		contentChanges, commercialChg bool
	}{
		{"name is content", func(p *supplier.Product) { p.Name += " (new)" }, true, false},
		{"an attribute is content", func(p *supplier.Product) { p.Attributes["Тегло"] = "1.000 kg" }, true, false},
		{"a spec is content", func(p *supplier.Product) { p.Specs = map[string]any{"socket": "AM5"} }, true, false},
		{"category is content", func(p *supplier.Product) { p.Category = "gpu" }, true, false},
		{"cost is commercial", func(p *supplier.Product) { p.CostCents++ }, false, true},
		{"stock is commercial", func(p *supplier.Product) { p.StockQty++ }, false, true},
	}
	for _, tt := range tests {
		content, commercial := hashes(tt.change)
		if (content != baseContent) != tt.contentChanges || (commercial != baseCommercial) != tt.commercialChg {
			t.Errorf("%s: content changed=%v commercial changed=%v", tt.name, content != baseContent, commercial != baseCommercial)
		}
	}

	if _, err := contentHash(supplier.Product{Specs: map[string]any{"x": func() {}}}); err == nil {
		t.Error("a spec JSON can't hold should fail the hash")
	}
}
