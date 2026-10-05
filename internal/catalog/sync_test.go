package catalog

import (
	"context"
	"errors"
	"iter"
	"log/slog"
	"maps"
	"slices"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gfotev/loadout/fixtures"
	"github.com/gfotev/loadout/internal/platform/db"
	"github.com/gfotev/loadout/internal/platform/db/dbtest"
	"github.com/gfotev/loadout/internal/vali"
)

func loadFixtures(t *testing.T) []vali.RawProduct {
	t.Helper()
	products, err := vali.LoadFixtures(fixtures.Vali)
	if err != nil {
		t.Fatal(err)
	}
	return products
}

// syncEnv is a catalog service over a fresh database, with a fake Vali whose products a test
// can change between runs.
type syncEnv struct {
	pool *pgxpool.Pool
	svc  *Service
	vali *vali.FakeClient
}

func newSyncEnv(t *testing.T) *syncEnv {
	t.Helper()
	pool := dbtest.New(t)
	fake := &vali.FakeClient{Products: loadFixtures(t)}
	svc := NewService(pool, fake, slog.New(slog.DiscardHandler))
	env := &syncEnv{pool: pool, svc: svc, vali: fake}
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

// versions returns every catalog row's xmin (the transaction that last wrote it), so a test can
// tell whether a run rewrote anything at all.
func (e *syncEnv) versions(t *testing.T) map[string]string {
	t.Helper()
	query := `
		SELECT 'product:' || id, xmin::text FROM catalog.products
		UNION ALL SELECT 'price:' || product_id, xmin::text FROM catalog.prices
		UNION ALL SELECT 'stock:' || product_id || ':' || warehouse, xmin::text FROM catalog.stock`
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

// ryzenIn returns the index of the Ryzen fixture in products.
func ryzenIn(products []vali.RawProduct) int {
	return slices.IndexFunc(products, func(p vali.RawProduct) bool { return p.SKU == ryzen })
}

// withoutRyzen returns a copy of products without the Ryzen fixture.
func withoutRyzen(products []vali.RawProduct) []vali.RawProduct {
	return slices.Delete(slices.Clone(products), ryzenIn(products), ryzenIn(products)+1)
}

func TestSyncFixtures(t *testing.T) {
	env := newSyncEnv(t)
	n := len(env.vali.Products)

	first := env.mustSync(t)
	if first.Status != SyncSucceeded || first.Fetched != n || first.Changed != n || first.Errors != 0 {
		t.Fatalf("first run = %+v, want %d fetched and changed", first, n)
	}

	p := env.product(t, ryzen)
	if p.Category != "cpu" || p.Slug != "amd-am4-r3-ryz-4100-box" || p.MPN != "100-100000510BOX" ||
		p.WarrantyMonths == nil || *p.WarrantyMonths != 36 || p.RawSpecs["Сокет"] != "AM4" {
		t.Errorf("stored product = %+v", p)
	}
	price := env.price(t, ryzen)
	// 52.35 € + 20% = 62.82 €
	if price.DealerCents != 5235 || price.RetailCents != 6282 || price.VATbp != 2000 || price.RuleID == nil {
		t.Errorf("price = %+v", price)
	}
	stock, _ := store{}.stockByProduct(context.Background(), env.pool, p.ID)
	if !maps.Equal(stock, map[string]int{"Plovdiv": 3, "Sofia-Central": 16}) {
		t.Errorf("stock = %v", stock)
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

	t.Run("a price change on Vali updates exactly one product", func(t *testing.T) {
		env.vali.Products = slices.Clone(env.vali.Products)
		i := ryzenIn(env.vali.Products)
		env.vali.Products[i].DealerPriceCents = 5000

		before := env.versions(t)
		run := env.mustSync(t)
		if run.Changed != 1 {
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
		i := ryzenIn(env.vali.Products)
		env.vali.Products[i].Stock = map[string]int{"Plovdiv": 0, "Sofia-Central": 16}

		before := env.versions(t)
		if run := env.mustSync(t); run.Changed != 1 {
			t.Errorf("changed = %d, want 1", run.Changed)
		}
		id := env.product(t, ryzen).ID.String()
		want := []string{"product:" + id, "stock:" + id + ":Plovdiv"}
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

func TestSyncSkipsBadProducts(t *testing.T) {
	env := newSyncEnv(t)
	good := env.vali.Products[0]

	bgn := good
	bgn.SKU, bgn.Currency = "PRICED-IN-BGN", "BGN"
	unknown := good
	unknown.SKU, unknown.CategoryPath = "UNKNOWN-CATEGORY", []string{"Компютърни компоненти", "Твърди дискове (HDD)"}
	free := good
	free.SKU, free.DealerPriceCents = "NO-PRICE", 0
	negative := good
	negative.SKU, negative.Stock = "NEGATIVE-STOCK", map[string]int{"Plovdiv": -1}
	env.vali.Products = []vali.RawProduct{bgn, good, unknown, free, negative}

	run := env.mustSync(t)
	if run.Status != SyncSucceeded || run.Fetched != 5 || run.Changed != 1 || run.Errors != 4 {
		t.Fatalf("run = %+v, want 1 stored and 4 errors", run)
	}
	var n int
	_ = env.pool.QueryRow(context.Background(), `SELECT count(*) FROM catalog.products`).Scan(&n)
	if n != 1 {
		t.Errorf("%d products stored, want only the good one", n)
	}

	for _, tt := range []struct {
		raw  vali.RawProduct
		want error
	}{
		{bgn, ErrUnsupportedCurrency},
		{unknown, ErrUnknownCategory},
		{free, ErrInvalidProduct},
		{negative, ErrInvalidProduct},
	} {
		if _, _, _, err := fromRaw(tt.raw); !errors.Is(err, tt.want) {
			t.Errorf("%s: err = %v, want %v", tt.raw.SKU, err, tt.want)
		}
	}
}

func TestSyncWithoutMarkupRule(t *testing.T) {
	env := newSyncEnv(t)
	if _, err := env.pool.Exec(context.Background(), `DELETE FROM catalog.markup_rules`); err != nil {
		t.Fatal(err)
	}
	env.vali.Products = env.vali.Products[:3]

	run := env.mustSync(t)
	if run.Errors != 3 || run.Changed != 0 {
		t.Errorf("run = %+v, want every product skipped", run)
	}
}

func TestSyncDeactivatesMissingProducts(t *testing.T) {
	env := newSyncEnv(t)
	all := env.vali.Products
	env.mustSync(t)

	// Ryzen disappears from Vali.
	env.vali.Products = withoutRyzen(all)
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
	env.vali.Products = all
	env.mustSync(t)
	if p := env.product(t, ryzen); p.MissingRuns != 0 || !p.Active {
		t.Errorf("after it came back: missing_runs=%d active=%v", p.MissingRuns, p.Active)
	}

	t.Run("a run where it's present resets the count", func(t *testing.T) {
		env.vali.Products = withoutRyzen(all)
		env.mustSync(t)
		env.vali.Products = all
		env.mustSync(t)
		if p := env.product(t, ryzen); p.MissingRuns != 0 || !p.Active {
			t.Errorf("missing_runs=%d active=%v", p.MissingRuns, p.Active)
		}
	})
}

func TestSyncFetchFailure(t *testing.T) {
	env := newSyncEnv(t)
	all := env.vali.Products
	env.mustSync(t)

	// Vali fails after the first 5 products: the run fails, those 5 are still synced, and no
	// product is counted as missing because the list was incomplete.
	env.vali.Products, env.vali.Err = all[:5], errors.New("vali: 503")
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
	var missing int
	_ = env.pool.QueryRow(context.Background(), `SELECT count(*) FROM catalog.products WHERE missing_runs > 0`).Scan(&missing)
	if missing != 0 {
		t.Errorf("%d products marked missing after a failed fetch", missing)
	}
}

// cancelAfter passes through n products from the fake and then cancels the run's context,
// like a SIGTERM arriving mid-sync.
type cancelAfter struct {
	*vali.FakeClient
	n      int
	cancel context.CancelFunc
}

func (c *cancelAfter) ListProducts(ctx context.Context, since time.Time) iter.Seq2[vali.RawProduct, error] {
	return func(yield func(vali.RawProduct, error) bool) {
		i := 0
		for p, err := range c.FakeClient.ListProducts(ctx, since) {
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
	env.svc.supplier = &cancelAfter{FakeClient: env.vali, n: 3, cancel: cancel}

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
	var n int
	_ = env.pool.QueryRow(context.Background(), `SELECT count(*) FROM catalog.products`).Scan(&n)
	if n != 3 {
		t.Errorf("%d products stored, want the 3 synced before the cancel", n)
	}
	// The lock was released, so a new run can start.
	env.svc.supplier = env.vali
	if run := env.mustSync(t); run.Changed != len(env.vali.Products)-3 {
		t.Errorf("next run changed %d, want the remaining %d", run.Changed, len(env.vali.Products)-3)
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
	var runs int
	_ = env.pool.QueryRow(context.Background(), `SELECT count(*) FROM catalog.sync_runs`).Scan(&runs)
	if runs != 0 {
		t.Errorf("%d sync runs recorded, want none", runs)
	}

	release()
	env.mustSync(t)
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
	raw := loadFixtures(t)[0]
	base, _, _, err := fromRaw(raw)
	if err != nil {
		t.Fatal(err)
	}
	hashes := func(change func(*vali.RawProduct)) (content, commercial string) {
		r := raw
		r.Specs, r.Stock = maps.Clone(raw.Specs), maps.Clone(raw.Stock)
		change(&r)
		p, _, _, err := fromRaw(r)
		if err != nil {
			t.Fatal(err)
		}
		return p.RawHash, p.CommercialHash
	}

	tests := []struct {
		name                          string
		change                        func(*vali.RawProduct)
		contentChanges, commercialChg bool
	}{
		{"updated_at is ignored", func(r *vali.RawProduct) { r.UpdatedAt = r.UpdatedAt.Add(time.Hour) }, false, false},
		{"name is content", func(r *vali.RawProduct) { r.Name += " (new)" }, true, false},
		{"a spec is content", func(r *vali.RawProduct) { r.Specs["Тегло"] = "1.000 kg" }, true, false},
		{"category path is content", func(r *vali.RawProduct) { r.CategoryPath = []string{"x", "Видео карти"} }, true, false},
		{"dealer price is commercial", func(r *vali.RawProduct) { r.DealerPriceCents++ }, false, true},
		{"rrp is commercial", func(r *vali.RawProduct) { r.RRPCents++ }, false, true},
		{"stock is commercial", func(r *vali.RawProduct) { r.Stock["Varna"] = 1 }, false, true},
	}
	for _, tt := range tests {
		content, commercial := hashes(tt.change)
		if (content != base.RawHash) != tt.contentChanges || (commercial != base.CommercialHash) != tt.commercialChg {
			t.Errorf("%s: content changed=%v commercial changed=%v", tt.name, content != base.RawHash, commercial != base.CommercialHash)
		}
	}

	_, nilStock := hashes(func(r *vali.RawProduct) { r.Stock = nil })
	_, emptyStock := hashes(func(r *vali.RawProduct) { r.Stock = map[string]int{} })
	if nilStock != emptyStock {
		t.Error("nil and empty stock hash differently")
	}
}
