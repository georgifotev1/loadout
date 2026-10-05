package catalog

import (
	"context"
	"errors"
	"maps"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gfotev/loadout/internal/platform/db/dbtest"
)

func testProduct() Product {
	return Product{
		Provider:       ProviderVali,
		ProviderSKU:    "AMD-AM4-R3-RYZ-4100-BOX",
		MPN:            "100-100000510BOX",
		Brand:          "AMD",
		Name:           "Процесор AMD Ryzen 3 4100",
		Slug:           "amd-ryzen-3-4100",
		Category:       "cpu",
		RawSpecs:       map[string]string{"Сокет": "AM4"},
		Images:         []string{"https://www.vali.bg/a.jpg"},
		WarrantyText:   "36 месеца",
		RawHash:        "raw-1",
		CommercialHash: "com-1",
	}
}

func TestUpsertProduct(t *testing.T) {
	pool := dbtest.New(t)
	ctx := context.Background()
	var s store

	reset := func(t *testing.T) {
		t.Cleanup(func() {
			// CASCADE: prices and stock reference products.
			if _, err := pool.Exec(ctx, `TRUNCATE catalog.products CASCADE`); err != nil {
				t.Fatalf("truncate: %v", err)
			}
		})
	}

	t.Run("inserts a new product", func(t *testing.T) {
		reset(t)
		id, written, err := s.upsertProduct(ctx, pool, testProduct())
		if err != nil {
			t.Fatal(err)
		}
		if !written || id == uuid.Nil {
			t.Fatalf("written=%v id=%v, want a new row", written, id)
		}

		got, err := s.productBySlug(ctx, pool, "amd-ryzen-3-4100")
		if err != nil {
			t.Fatal(err)
		}
		if got.ID != id || got.RawSpecs["Сокет"] != "AM4" || len(got.Images) != 1 || !got.Active {
			t.Errorf("read back %+v", got)
		}
		if got.EAN != "" || got.WarrantyMonths != nil {
			t.Errorf("empty optional fields: ean=%q warranty_months=%v", got.EAN, got.WarrantyMonths)
		}
	})

	t.Run("same sku from another provider is a separate product", func(t *testing.T) {
		reset(t)
		first := mustUpsert(t, s, pool, testProduct())

		other := testProduct()
		other.Provider = "other"
		other.Slug = "amd-ryzen-3-4100-other"
		id, written, err := s.upsertProduct(ctx, pool, other)
		if err != nil {
			t.Fatal(err)
		}
		if !written || id == first {
			t.Fatalf("written=%v id=%v (first %v), want a second row", written, id, first)
		}
	})

	t.Run("same hashes write nothing", func(t *testing.T) {
		reset(t)
		mustUpsert(t, s, pool, testProduct())
		before := xmin(t, pool)

		_, written, err := s.upsertProduct(ctx, pool, testProduct())
		if err != nil {
			t.Fatal(err)
		}
		if written {
			t.Error("written = true, want false for an unchanged product")
		}
		// xmin is the id of the transaction that wrote the current row version. If it didn't
		// change, Postgres really didn't rewrite the row.
		if after := xmin(t, pool); after != before {
			t.Errorf("row was rewritten: xmin %s → %s", before, after)
		}
	})

	t.Run("either hash changing updates the row", func(t *testing.T) {
		for name, change := range map[string]func(*Product){
			"content":    func(p *Product) { p.RawHash = "raw-2"; p.Name = "Ryzen 3 4100 (new name)" },
			"commercial": func(p *Product) { p.CommercialHash = "com-2" },
		} {
			t.Run(name, func(t *testing.T) {
				reset(t)
				first := mustUpsert(t, s, pool, testProduct())

				p := testProduct()
				change(&p)
				id, written, err := s.upsertProduct(ctx, pool, p)
				if err != nil {
					t.Fatal(err)
				}
				if !written || id != first {
					t.Fatalf("written=%v id=%v, want an update of %v", written, id, first)
				}
				got, _ := s.productBySlug(ctx, pool, p.Slug)
				if got.Name != p.Name || got.RawHash != p.RawHash || got.CommercialHash != p.CommercialHash {
					t.Errorf("not updated: %+v", got)
				}
				if !got.UpdatedAt.After(got.CreatedAt) {
					t.Errorf("updated_at %v not after created_at %v", got.UpdatedAt, got.CreatedAt)
				}
			})
		}
	})

	t.Run("update keeps slug and featured", func(t *testing.T) {
		reset(t)
		mustUpsert(t, s, pool, testProduct())
		if _, err := pool.Exec(ctx, `UPDATE catalog.products SET featured = true`); err != nil {
			t.Fatal(err)
		}

		p := testProduct()
		p.Slug = "something-else"
		p.RawHash = "raw-2"
		mustUpsert(t, s, pool, p)

		got, err := s.productBySlug(ctx, pool, "amd-ryzen-3-4100")
		if err != nil {
			t.Fatalf("original slug: %v", err)
		}
		if !got.Featured {
			t.Error("featured was reset by sync")
		}
	})

	t.Run("unknown slug is ErrNotFound", func(t *testing.T) {
		if _, err := s.productBySlug(ctx, pool, "nope"); !errors.Is(err, ErrNotFound) {
			t.Errorf("err = %v, want ErrNotFound", err)
		}
	})
}

func mustUpsert(t *testing.T, s store, pool *pgxpool.Pool, p Product) uuid.UUID {
	t.Helper()
	id, _, err := s.upsertProduct(context.Background(), pool, p)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func xmin(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	var x string
	if err := pool.QueryRow(context.Background(), `SELECT xmin::text FROM catalog.products`).Scan(&x); err != nil {
		t.Fatal(err)
	}
	return x
}

func TestPrices(t *testing.T) {
	pool := dbtest.New(t)
	ctx := context.Background()
	var s store
	productID := mustUpsert(t, s, pool, testProduct())
	ruleID, err := s.upsertMarkupRule(ctx, pool, MarkupRule{Scope: ScopeDefault, MarkupBP: 2000, Rounding: RoundNone})
	if err != nil {
		t.Fatal(err)
	}
	price := Price{ProductID: productID, DealerCents: 5235, RRPCents: 6233, RetailCents: 6282, VATbp: 2000, RuleID: &ruleID}

	if written, err := s.upsertPrice(ctx, pool, price); err != nil || !written {
		t.Fatalf("first upsert: written=%v err=%v", written, err)
	}
	if written, err := s.upsertPrice(ctx, pool, price); err != nil || written {
		t.Fatalf("same price: written=%v err=%v, want no write", written, err)
	}

	price.DealerCents = 4999
	if written, err := s.upsertPrice(ctx, pool, price); err != nil || !written {
		t.Fatalf("changed price: written=%v err=%v", written, err)
	}
	got, err := s.priceByProduct(ctx, pool, productID)
	if err != nil {
		t.Fatal(err)
	}
	if got.DealerCents != 4999 || got.RuleID == nil || *got.RuleID != ruleID {
		t.Errorf("got %+v", got)
	}

	t.Run("deleting the rule keeps the price", func(t *testing.T) {
		if err := s.deleteMarkupRule(ctx, pool, ruleID); err != nil {
			t.Fatal(err)
		}
		got, err := s.priceByProduct(ctx, pool, productID)
		if err != nil {
			t.Fatal(err)
		}
		if got.RuleID != nil || got.RetailCents != 6282 {
			t.Errorf("got %+v, want same amounts and rule_id nil", got)
		}
	})

	t.Run("rejects negative amounts", func(t *testing.T) {
		bad := price
		bad.RuleID, bad.RetailCents = nil, -1
		if _, err := s.upsertPrice(ctx, pool, bad); err == nil {
			t.Error("want a CHECK violation")
		}
	})

	t.Run("unknown product is ErrNotFound", func(t *testing.T) {
		if _, err := s.priceByProduct(ctx, pool, uuid.New()); !errors.Is(err, ErrNotFound) {
			t.Errorf("err = %v", err)
		}
	})
}

func TestReplaceStock(t *testing.T) {
	pool := dbtest.New(t)
	ctx := context.Background()
	var s store
	productID := mustUpsert(t, s, pool, testProduct())

	steps := []struct {
		name        string
		qty         map[string]int
		wantChanged int64
	}{
		{"initial", map[string]int{"Sofia-Central": 16, "Plovdiv": 3}, 2},
		{"unchanged writes nothing", map[string]int{"Sofia-Central": 16, "Plovdiv": 3}, 0},
		{"one qty changes", map[string]int{"Sofia-Central": 15, "Plovdiv": 3}, 1},
		{"warehouse disappears, another appears", map[string]int{"Sofia-Central": 15, "Varna": 1}, 2},
		{"no stock anywhere", map[string]int{}, 2},
	}
	for _, st := range steps {
		n, err := s.replaceStock(ctx, pool, productID, st.qty)
		if err != nil {
			t.Fatalf("%s: %v", st.name, err)
		}
		if n != st.wantChanged {
			t.Errorf("%s: changed %d rows, want %d", st.name, n, st.wantChanged)
		}
		got, err := s.stockByProduct(ctx, pool, productID)
		if err != nil {
			t.Fatal(err)
		}
		if !maps.Equal(got, st.qty) {
			t.Errorf("%s: stock = %v, want %v", st.name, got, st.qty)
		}
	}
}

func TestMarkupRules(t *testing.T) {
	pool := dbtest.New(t)
	ctx := context.Background()
	var s store
	// Start without the default rule that migration 00007 seeds.
	if _, err := pool.Exec(ctx, `DELETE FROM catalog.markup_rules`); err != nil {
		t.Fatal(err)
	}

	t.Run("one rule per target, upsert updates it", func(t *testing.T) {
		r := MarkupRule{Scope: ScopeBrandCategory, Category: "gpu", Brand: "Gigabyte", MarkupBP: 1500, Rounding: Round99}
		id1, err := s.upsertMarkupRule(ctx, pool, r)
		if err != nil {
			t.Fatal(err)
		}
		r.MarkupBP, r.MinMarginCents = 1800, 2000
		id2, err := s.upsertMarkupRule(ctx, pool, r)
		if err != nil {
			t.Fatal(err)
		}
		if id1 != id2 {
			t.Errorf("ids %d and %d, want the same rule updated", id1, id2)
		}

		// Two default rules collide too, thanks to NULLS NOT DISTINCT.
		d1, _ := s.upsertMarkupRule(ctx, pool, MarkupRule{Scope: ScopeDefault, MarkupBP: 2000, Rounding: RoundNone})
		d2, _ := s.upsertMarkupRule(ctx, pool, MarkupRule{Scope: ScopeDefault, MarkupBP: 2200, Rounding: RoundNone})
		if d1 != d2 {
			t.Errorf("default rules %d and %d, want one", d1, d2)
		}

		rules, err := s.markupRules(ctx, pool)
		if err != nil {
			t.Fatal(err)
		}
		if len(rules) != 2 {
			t.Fatalf("got %d rules, want 2: %+v", len(rules), rules)
		}
		if g := rules[0]; g.MarkupBP != 1800 || g.MinMarginCents != 2000 || g.Brand != "Gigabyte" || g.SKU != "" {
			t.Errorf("got %+v", g)
		}
	})

	t.Run("fields must fit the scope", func(t *testing.T) {
		for name, r := range map[string]MarkupRule{
			"default with a category":      {Scope: ScopeDefault, Category: "gpu"},
			"category without a category":  {Scope: ScopeCategory},
			"brand_category without brand": {Scope: ScopeBrandCategory, Category: "gpu"},
			"sku with a brand":             {Scope: ScopeSKU, SKU: "X", Brand: "AMD"},
			"unknown scope":                {Scope: "everything"},
			"unknown rounding":             {Scope: ScopeSKU, SKU: "X", Rounding: "95"},
		} {
			if r.Rounding == "" {
				r.Rounding = RoundNone
			}
			if _, err := s.upsertMarkupRule(ctx, pool, r); err == nil {
				t.Errorf("%s: accepted, want a CHECK violation", name)
			}
		}
	})

	t.Run("delete unknown is ErrNotFound", func(t *testing.T) {
		if err := s.deleteMarkupRule(ctx, pool, 999999); !errors.Is(err, ErrNotFound) {
			t.Errorf("err = %v", err)
		}
	})
}

func TestSyncRuns(t *testing.T) {
	pool := dbtest.New(t)
	ctx := context.Background()
	var s store

	id, err := s.startSyncRun(ctx, pool)
	if err != nil {
		t.Fatal(err)
	}
	run, err := s.syncRun(ctx, pool, id)
	if err != nil {
		t.Fatal(err)
	}
	if run.Status != SyncRunning || run.FinishedAt != nil {
		t.Errorf("new run = %+v", run)
	}

	done := SyncRun{ID: id, Fetched: 60, Changed: 1, Status: SyncSucceeded}
	if err := s.finishSyncRun(ctx, pool, done); err != nil {
		t.Fatal(err)
	}
	run, _ = s.syncRun(ctx, pool, id)
	if run.Status != SyncSucceeded || run.FinishedAt == nil || run.Fetched != 60 || run.Changed != 1 {
		t.Errorf("finished run = %+v", run)
	}

	if err := s.finishSyncRun(ctx, pool, done); !errors.Is(err, ErrSyncRunNotRunning) {
		t.Errorf("finishing twice: err = %v, want ErrSyncRunNotRunning", err)
	}

	failed, _ := s.startSyncRun(ctx, pool)
	if err := s.finishSyncRun(ctx, pool, SyncRun{ID: failed, Status: SyncFailed, ErrorMessage: "vali: 503"}); err != nil {
		t.Fatal(err)
	}
	if run, _ := s.syncRun(ctx, pool, failed); run.ErrorMessage != "vali: 503" {
		t.Errorf("failed run = %+v", run)
	}

	// finished_at and status must agree: "running" can't be a final status.
	other, _ := s.startSyncRun(ctx, pool)
	if err := s.finishSyncRun(ctx, pool, SyncRun{ID: other, Status: SyncRunning}); err == nil {
		t.Error("finishing with status running was accepted")
	}
}
