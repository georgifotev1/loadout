package catalog

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gfotev/loadout/internal/platform/db/dbtest"
)

func testProduct() Product {
	return Product{
		Provider:       "vali",
		ProviderSKU:    "AMD-AM4-R3-RYZ-4100-BOX",
		Brand:          "AMD",
		Name:           "Процесор AMD Ryzen 3 4100",
		Slug:           "amd-ryzen-3-4100",
		Category:       "cpu",
		Attributes:     map[string]string{"Сокет": "AM4"},
		Images:         []string{"https://www.vali.bg/a.jpg"},
		ContentHash:    "content-1",
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
		if got.ID != id || got.Attributes["Сокет"] != "AM4" || len(got.Images) != 1 || !got.Active {
			t.Errorf("read back %+v", got)
		}
		if byID, err := s.productByID(ctx, pool, id); err != nil || byID.Slug != got.Slug {
			t.Errorf("by id: %+v, %v", byID, err)
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
			"content":    func(p *Product) { p.ContentHash = "content-2"; p.Name = "Ryzen 3 4100 (new name)" },
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
				if got.Name != p.Name || got.ContentHash != p.ContentHash || got.CommercialHash != p.CommercialHash {
					t.Errorf("not updated: %+v", got)
				}
				if !got.UpdatedAt.After(got.CreatedAt) {
					t.Errorf("updated_at %v not after created_at %v", got.UpdatedAt, got.CreatedAt)
				}
			})
		}
	})

	t.Run("update keeps the slug", func(t *testing.T) {
		reset(t)
		mustUpsert(t, s, pool, testProduct())

		p := testProduct()
		p.Slug = "something-else"
		p.ContentHash = "content-2"
		mustUpsert(t, s, pool, p)

		if _, err := s.productBySlug(ctx, pool, "amd-ryzen-3-4100"); err != nil {
			t.Fatalf("original slug: %v", err)
		}
	})

	t.Run("unknown slug or ID is ErrNotFound", func(t *testing.T) {
		if _, err := s.productBySlug(ctx, pool, "nope"); !errors.Is(err, ErrNotFound) {
			t.Errorf("err = %v, want ErrNotFound", err)
		}
		if _, err := s.productByID(ctx, pool, uuid.New()); !errors.Is(err, ErrNotFound) {
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
	rule, err := s.insertMarkupRule(ctx, pool, MarkupRule{Scope: ScopeCategory, Category: "cpu", MarkupBP: 2000, Rounding: RoundNone})
	if err != nil {
		t.Fatal(err)
	}
	ruleID := rule.ID
	price := Price{ProductID: productID, DealerCents: 5235, RetailCents: 6282, VATbp: 2000, RuleID: &ruleID}

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

func TestSetStock(t *testing.T) {
	pool := dbtest.New(t)
	ctx := context.Background()
	var s store
	productID := mustUpsert(t, s, pool, testProduct())

	steps := []struct {
		name        string
		qty         int
		wantChanged bool
	}{
		{"initial", 19, true},
		{"unchanged writes nothing", 19, false},
		{"qty changes", 18, true},
		{"none left", 0, true},
	}
	for _, st := range steps {
		changed, err := s.setStock(ctx, pool, productID, st.qty)
		if err != nil {
			t.Fatalf("%s: %v", st.name, err)
		}
		if changed != st.wantChanged {
			t.Errorf("%s: changed = %v, want %v", st.name, changed, st.wantChanged)
		}
		if got, err := s.stockByProduct(ctx, pool, productID); err != nil || got != st.qty {
			t.Errorf("%s: stock = %d, %v; want %d", st.name, got, err, st.qty)
		}
	}

	t.Run("rejects negative stock", func(t *testing.T) {
		if _, err := s.setStock(ctx, pool, productID, -1); err == nil {
			t.Error("want a CHECK violation")
		}
	})
}

func TestMarkupRules(t *testing.T) {
	pool := dbtest.New(t)
	ctx := context.Background()
	var s store
	// Start without the default rule that migration 00007 seeds.
	if _, err := pool.Exec(ctx, `DELETE FROM catalog.markup_rules`); err != nil {
		t.Fatal(err)
	}

	gigabyte := MarkupRule{Scope: ScopeBrandCategory, Category: "gpu", Brand: "Gigabyte", MarkupBP: 1500, Rounding: Round99}
	created, err := s.insertMarkupRule(ctx, pool, gigabyte)
	if err != nil {
		t.Fatal(err)
	}
	if created.ID == 0 || created.Version != 1 || created.Brand != "Gigabyte" || created.SKU != "" {
		t.Errorf("created %+v", created)
	}

	t.Run("one rule per target, brands ignoring case", func(t *testing.T) {
		same := gigabyte
		same.Brand = "GIGABYTE"
		if _, err := s.insertMarkupRule(ctx, pool, same); !errors.Is(err, ErrDuplicateRule) {
			t.Errorf("same brand in capitals: err = %v, want ErrDuplicateRule", err)
		}
		// Two default rules collide too, thanks to NULLS NOT DISTINCT.
		def := MarkupRule{Scope: ScopeDefault, MarkupBP: 2000, Rounding: RoundNone}
		if _, err := s.insertMarkupRule(ctx, pool, def); err != nil {
			t.Fatal(err)
		}
		if _, err := s.insertMarkupRule(ctx, pool, def); !errors.Is(err, ErrDuplicateRule) {
			t.Errorf("second default: err = %v, want ErrDuplicateRule", err)
		}
	})

	t.Run("update bumps the version", func(t *testing.T) {
		r := created
		r.MarkupBP, r.MinMarginCents = 1800, 2000
		got, err := s.updateMarkupRule(ctx, pool, r)
		if err != nil {
			t.Fatal(err)
		}
		if got.Version != 2 || got.MarkupBP != 1800 || got.MinMarginCents != 2000 || !got.UpdatedAt.After(created.UpdatedAt) {
			t.Errorf("updated %+v", got)
		}
		locked, err := s.markupRuleForUpdate(ctx, pool, created.ID)
		if err != nil || locked != got {
			t.Errorf("read back %+v, %v", locked, err)
		}
	})

	t.Run("update onto another rule's target is ErrDuplicateRule", func(t *testing.T) {
		other, err := s.insertMarkupRule(ctx, pool, MarkupRule{Scope: ScopeBrandCategory, Category: "gpu", Brand: "ASUS", Rounding: RoundNone})
		if err != nil {
			t.Fatal(err)
		}
		other.Brand = "gigabyte"
		if _, err := s.updateMarkupRule(ctx, pool, other); !errors.Is(err, ErrDuplicateRule) {
			t.Errorf("err = %v, want ErrDuplicateRule", err)
		}
	})

	t.Run("listed by scope with product counts", func(t *testing.T) {
		productID := mustUpsert(t, s, pool, testProduct())
		price := Price{ProductID: productID, DealerCents: 100, RetailCents: 120, VATbp: 2000, RuleID: &created.ID}
		if _, err := s.upsertPrice(ctx, pool, price); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if _, err := pool.Exec(ctx, `TRUNCATE catalog.products CASCADE`); err != nil {
				t.Errorf("truncate: %v", err)
			}
		})

		rules, err := s.listedRules(ctx, pool)
		if err != nil {
			t.Fatal(err)
		}
		var got []string
		for _, r := range rules {
			got = append(got, fmt.Sprintf("%s/%s:%d", r.Scope, r.Brand, r.Products))
		}
		want := []string{"default/:0", "brand_category/ASUS:0", "brand_category/Gigabyte:1"}
		if !slices.Equal(got, want) {
			t.Errorf("got %v, want %v", got, want)
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
			if _, err := s.insertMarkupRule(ctx, pool, r); err == nil {
				t.Errorf("%s: accepted, want a CHECK violation", name)
			}
		}
	})

	t.Run("unknown id is ErrNotFound", func(t *testing.T) {
		if _, err := s.markupRuleForUpdate(ctx, pool, 999999); !errors.Is(err, ErrNotFound) {
			t.Errorf("lock: err = %v", err)
		}
		if _, err := s.updateMarkupRule(ctx, pool, MarkupRule{ID: 999999, Scope: ScopeSKU, SKU: "X", Rounding: RoundNone}); !errors.Is(err, ErrNotFound) {
			t.Errorf("update: err = %v", err)
		}
		if err := s.deleteMarkupRule(ctx, pool, 999999); !errors.Is(err, ErrNotFound) {
			t.Errorf("delete: err = %v", err)
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

	// Newest first: other, failed, id. A page past the end is empty.
	runs, total, err := s.syncRuns(ctx, pool, 2, 0)
	if err != nil {
		t.Fatal(err)
	}
	if total != 3 || len(runs) != 2 || runs[0].ID != other || runs[1].ID != failed {
		t.Errorf("page 1: total %d, runs %+v", total, runs)
	}
	runs, _, _ = s.syncRuns(ctx, pool, 2, 2)
	if len(runs) != 1 || runs[0].ID != id {
		t.Errorf("page 2: runs %+v", runs)
	}
	if runs, _, _ = s.syncRuns(ctx, pool, 2, 4); len(runs) != 0 {
		t.Errorf("past the end: runs %+v", runs)
	}
}
