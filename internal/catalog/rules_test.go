package catalog

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/gfotev/loadout/internal/platform/validate"
)

func TestMarkupRuleService(t *testing.T) {
	env := newSyncEnv(t)
	ctx := context.Background()
	svc := env.svc

	rules, err := svc.MarkupRules(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) != 1 || rules[0].Scope != ScopeDefault {
		t.Fatalf("rules = %+v, want the default one", rules)
	}
	def := rules[0].MarkupRule

	cpu, err := svc.CreateMarkupRule(ctx, RuleInput{Scope: ScopeCategory, Category: " cpu ", MarkupBP: 3000, Rounding: RoundNone})
	if err != nil {
		t.Fatal(err)
	}
	if cpu.Category != "cpu" || cpu.Version != 1 {
		t.Errorf("created %+v, want the category trimmed at version 1", cpu)
	}

	t.Run("applies from the next sync", func(t *testing.T) {
		env.mustSync(t)
		// 52.35 € + 30% = 68.055 €, half up to 68.06 €
		if got := env.price(t, ryzen); got.RetailCents != 6806 || *got.RuleID != cpu.ID {
			t.Errorf("price = %+v, want 6806 by rule %d", got, cpu.ID)
		}
		rules, err := svc.MarkupRules(ctx)
		if err != nil {
			t.Fatal(err)
		}
		i := slices.IndexFunc(rules, func(r ListedRule) bool { return r.ID == cpu.ID })
		if i < 0 || rules[i].Products == 0 {
			t.Errorf("rules = %+v, want the cpu rule pricing products", rules)
		}
	})

	t.Run("a duplicate target is ErrDuplicateRule", func(t *testing.T) {
		_, err := svc.CreateMarkupRule(ctx, RuleInput{Scope: ScopeCategory, Category: "cpu", Rounding: RoundNone})
		if !errors.Is(err, ErrDuplicateRule) {
			t.Errorf("err = %v, want ErrDuplicateRule", err)
		}
	})

	t.Run("invalid input names the field", func(t *testing.T) {
		tests := []struct {
			name  string
			in    RuleInput
			field string
			code  string
		}{
			{"unknown scope", RuleInput{Scope: "brand"}, "scope", "unknown"},
			{"category missing", RuleInput{Scope: ScopeCategory}, "category", "required"},
			{"unknown category", RuleInput{Scope: ScopeCategory, Category: "laptop"}, "category", "unknown"},
			{"category on a sku rule", RuleInput{Scope: ScopeSKU, SKU: "X", Category: "cpu"}, "category", "not_allowed"},
			{"brand missing", RuleInput{Scope: ScopeBrandCategory, Category: "gpu", Brand: "  "}, "brand", "required"},
			{"brand on a category rule", RuleInput{Scope: ScopeCategory, Category: "gpu", Brand: "MSI"}, "brand", "not_allowed"},
			{"sku missing", RuleInput{Scope: ScopeSKU}, "sku", "required"},
			{"sku too long", RuleInput{Scope: ScopeSKU, SKU: strings.Repeat("x", 101)}, "sku", "too_long"},
			{"negative markup", RuleInput{Scope: ScopeSKU, SKU: "X", MarkupBP: -1}, "markup_bp", "out_of_range"},
			{"huge markup", RuleInput{Scope: ScopeSKU, SKU: "X", MarkupBP: MaxMarkupBP + 1}, "markup_bp", "out_of_range"},
			{"negative margin", RuleInput{Scope: ScopeSKU, SKU: "X", MinMarginCents: -1}, "min_margin_cents", "out_of_range"},
			{"unknown rounding", RuleInput{Scope: ScopeSKU, SKU: "X", Rounding: "95"}, "rounding", "unknown"},
		}
		for _, tt := range tests {
			if tt.in.Rounding == "" {
				tt.in.Rounding = RoundNone
			}
			_, err := svc.CreateMarkupRule(ctx, tt.in)
			var verr *validate.Error
			if !errors.As(err, &verr) || !slices.Contains(verr.Fields, validate.FieldError{Field: tt.field, Code: tt.code}) {
				t.Errorf("%s: err = %v, want %s %s", tt.name, err, tt.field, tt.code)
			}
		}
	})

	t.Run("an edit from a stale version is ErrEditConflict", func(t *testing.T) {
		in := RuleInput{Scope: ScopeCategory, Category: "cpu", MarkupBP: 2500, Rounding: Round90}
		updated, err := svc.UpdateMarkupRule(ctx, cpu.ID, cpu.Version, in)
		if err != nil {
			t.Fatal(err)
		}
		if updated.Version != 2 || updated.MarkupBP != 2500 || updated.Rounding != Round90 {
			t.Errorf("updated %+v", updated)
		}
		in.MarkupBP = 9900
		if _, err := svc.UpdateMarkupRule(ctx, cpu.ID, cpu.Version, in); !errors.Is(err, ErrEditConflict) {
			t.Errorf("err = %v, want ErrEditConflict", err)
		}
		cpu = updated
	})

	t.Run("a rule can change its target", func(t *testing.T) {
		in := RuleInput{Scope: ScopeBrandCategory, Category: "cpu", Brand: "AMD", MarkupBP: 2500, Rounding: RoundNone}
		updated, err := svc.UpdateMarkupRule(ctx, cpu.ID, cpu.Version, in)
		if err != nil {
			t.Fatal(err)
		}
		if updated.Scope != ScopeBrandCategory || updated.Brand != "AMD" {
			t.Errorf("updated %+v", updated)
		}
		cpu = updated
	})

	t.Run("the default rule stays", func(t *testing.T) {
		in := RuleInput{Scope: ScopeCategory, Category: "gpu", Rounding: RoundNone}
		if _, err := svc.UpdateMarkupRule(ctx, def.ID, def.Version, in); !errors.Is(err, ErrDefaultRule) {
			t.Errorf("rescoping the default: err = %v, want ErrDefaultRule", err)
		}
		if _, err := svc.UpdateMarkupRule(ctx, cpu.ID, cpu.Version, RuleInput{Scope: ScopeDefault, Rounding: RoundNone}); !errors.Is(err, ErrDefaultRule) {
			t.Errorf("making another rule default: err = %v, want ErrDefaultRule", err)
		}
		if err := svc.DeleteMarkupRule(ctx, def.ID); !errors.Is(err, ErrDefaultRule) {
			t.Errorf("deleting the default: err = %v, want ErrDefaultRule", err)
		}
		updated, err := svc.UpdateMarkupRule(ctx, def.ID, def.Version, RuleInput{Scope: ScopeDefault, MarkupBP: 1000, Rounding: Round99})
		if err != nil || updated.MarkupBP != 1000 {
			t.Errorf("editing the default's pricing: %+v, %v", updated, err)
		}
	})

	t.Run("delete keeps prices until the next sync", func(t *testing.T) {
		before := env.price(t, ryzen)
		if err := svc.DeleteMarkupRule(ctx, cpu.ID); err != nil {
			t.Fatal(err)
		}
		if got := env.price(t, ryzen); got.RetailCents != before.RetailCents || got.RuleID != nil {
			t.Errorf("price = %+v, want the same amount without a rule", got)
		}
		if err := svc.DeleteMarkupRule(ctx, cpu.ID); !errors.Is(err, ErrNotFound) {
			t.Errorf("deleting twice: err = %v, want ErrNotFound", err)
		}
		if _, err := svc.UpdateMarkupRule(ctx, cpu.ID, cpu.Version, RuleInput{Scope: ScopeSKU, SKU: "X", Rounding: RoundNone}); !errors.Is(err, ErrNotFound) {
			t.Errorf("updating a deleted rule: err = %v, want ErrNotFound", err)
		}
	})
}

func TestAdminProducts(t *testing.T) {
	env := newSyncEnv(t)
	ctx := context.Background()
	env.mustSync(t)
	n := len(env.src.Items)

	// One product the supplier dropped and one that was never priced.
	gone := env.src.Items[1].SKU
	unpriced := env.src.Items[2].SKU
	if _, err := env.pool.Exec(ctx, `UPDATE catalog.products SET active = false WHERE provider_sku = $1`, gone); err != nil {
		t.Fatal(err)
	}
	if _, err := env.pool.Exec(ctx, `DELETE FROM catalog.prices WHERE product_id = $1`, env.product(t, unpriced).ID); err != nil {
		t.Fatal(err)
	}

	list := func(t *testing.T, f AdminProductFilter) AdminProductPage {
		t.Helper()
		if f.Page == 0 {
			f.Page, f.PageSize = 1, MaxPageSize
		}
		page, err := env.svc.AdminProducts(ctx, f)
		if err != nil {
			t.Fatal(err)
		}
		return page
	}
	find := func(page AdminProductPage, sku string) *AdminProduct {
		for i := range page.Products {
			if page.Products[i].SKU == sku {
				return &page.Products[i]
			}
		}
		return nil
	}

	t.Run("lists every product, hidden ones too", func(t *testing.T) {
		all := list(t, AdminProductFilter{})
		if all.Total != n || len(all.Products) != min(n, MaxPageSize) {
			t.Fatalf("total %d, %d rows; want %d", all.Total, len(all.Products), n)
		}
		p := find(all, ryzen)
		if p == nil || p.Price == nil || p.Price.DealerCents != 5235 || p.Price.RetailCents != 6282 ||
			p.RuleScope != ScopeDefault || p.StockQty != 19 || !p.Active || p.Image == "" {
			t.Errorf("ryzen = %+v", p)
		}
		if p := find(all, unpriced); p == nil || p.Price != nil || p.RuleScope != "" {
			t.Errorf("unpriced = %+v, want no price", p)
		}
	})

	t.Run("text matches name, brand or sku, ignoring case", func(t *testing.T) {
		for _, q := range []string{"ryzen 3 4100", "amd-am4-r3", "  RYZ-4100  "} {
			if find(list(t, AdminProductFilter{Text: q}), ryzen) == nil {
				t.Errorf("q=%q doesn't find %s", q, ryzen)
			}
		}
		// % is a character, not a wildcard.
		if got := list(t, AdminProductFilter{Text: "%"}); got.Total != 0 {
			t.Errorf("q=%%: %d matches, want 0", got.Total)
		}
	})

	t.Run("filters by category and status", func(t *testing.T) {
		for _, p := range list(t, AdminProductFilter{Category: "cpu"}).Products {
			if p.Category != "cpu" {
				t.Errorf("category filter returned %s (%s)", p.SKU, p.Category)
			}
		}
		inactive := list(t, AdminProductFilter{Status: StatusInactive})
		if inactive.Total != 1 || inactive.Products[0].SKU != gone {
			t.Errorf("inactive = %+v, want only %s", inactive.Products, gone)
		}
		if active := list(t, AdminProductFilter{Status: StatusActive}); active.Total != n-1 || find(active, gone) != nil {
			t.Errorf("active: total %d, want %d without %s", active.Total, n-1, gone)
		}
	})

	t.Run("pages", func(t *testing.T) {
		p2 := list(t, AdminProductFilter{Page: 2, PageSize: 5})
		if len(p2.Products) != 5 || p2.Total != n {
			t.Errorf("page 2: %d rows, total %d", len(p2.Products), p2.Total)
		}
	})

	t.Run("rejects bad filters", func(t *testing.T) {
		for _, f := range []AdminProductFilter{
			{Category: "laptop", Page: 1, PageSize: 10},
			{Status: "hidden", Page: 1, PageSize: 10},
			{Text: strings.Repeat("я", MaxSearchTextLen+1), Page: 1, PageSize: 10},
			{Page: 0, PageSize: 10},
			{Page: 1, PageSize: MaxPageSize + 1},
		} {
			var verr *validate.Error
			if _, err := env.svc.AdminProducts(ctx, f); !errors.As(err, &verr) {
				t.Errorf("%+v: err = %v, want a validation error", f, err)
			}
		}
	})
}
