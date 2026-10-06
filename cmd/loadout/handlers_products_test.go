package main

import (
	"fmt"
	"net/http"
	"testing"
)

func TestAdminProductsAPI(t *testing.T) {
	app := newCatalogApp(t)
	admin := adminCookie(t, app)

	t.Run("filters and joins prices, rules and specs", func(t *testing.T) {
		body := decode[AdminProductList](t, get(t, app, "/api/admin/products?q=ryzen+3+4100&status=active", admin), http.StatusOK)
		if body.Total != 1 || len(body.Items) != 1 {
			t.Fatalf("body = %+v, want one product", body)
		}
		p := body.Items[0]
		if p.Price == nil || p.Price.DealerCents == 0 || p.Price.MarginCents != p.Price.RetailCents-p.Price.DealerCents || p.Price.MarginBp == 0 {
			t.Errorf("price = %+v", p.Price)
		}
		if p.Rule == nil || p.Rule.Scope != RuleScopeDefault {
			t.Errorf("rule = %+v, want the default rule", p.Rule)
		}
		if p.SpecsStatus != AdminProductSpecsStatusIncomplete || !p.Active || p.Category != CategoryCpu {
			t.Errorf("product = %+v", p)
		}
	})

	t.Run("peripherals have no specs", func(t *testing.T) {
		body := decode[AdminProductList](t, get(t, app, "/api/admin/products?category=peripheral.mouse&page_size=1", admin), http.StatusOK)
		if len(body.Items) != 1 || body.Items[0].SpecsStatus != AdminProductSpecsStatusNone {
			t.Errorf("items = %+v", body.Items)
		}
	})

	t.Run("bad filter is 422", func(t *testing.T) {
		for _, q := range []string{"status=hidden", "category=laptop", "page=0"} {
			if rec := get(t, app, "/api/admin/products?"+q, admin); rec.Code != http.StatusUnprocessableEntity {
				t.Errorf("%s: status %d, want 422", q, rec.Code)
			}
		}
	})
}

func TestMarkupRulesAPI(t *testing.T) {
	app := newCatalogApp(t)
	admin := adminCookie(t, app)
	const rules = "/api/admin/markup-rules"
	ruleURL := func(id int64) string { return fmt.Sprintf("%s/%d", rules, id) }

	list := decode[MarkupRuleList](t, get(t, app, rules, admin), http.StatusOK)
	if len(list.Items) != 1 || list.Items[0].Scope != RuleScopeDefault || list.Items[0].Products == 0 {
		t.Fatalf("list = %+v, want the default rule pricing every product", list)
	}
	def := list.Items[0]

	created := decode[MarkupRule](t, send(t, app, http.MethodPost, rules,
		`{"scope":"brand_category","category":"gpu","brand":"Gigabyte","markup_bp":1800,"min_margin_cents":1000,"rounding":"99"}`, admin),
		http.StatusCreated)
	if created.Id == 0 || created.Version != 1 || created.Brand != "Gigabyte" || created.Category != CategoryGpu {
		t.Errorf("created = %+v", created)
	}

	t.Run("a duplicate target is 409 duplicate_rule", func(t *testing.T) {
		rec := send(t, app, http.MethodPost, rules,
			`{"scope":"brand_category","category":"gpu","brand":"GIGABYTE","markup_bp":0,"min_margin_cents":0,"rounding":"none"}`, admin)
		if e := decode[Error](t, rec, http.StatusConflict); e.Error.Code != ErrorCodeDuplicateRule {
			t.Errorf("code = %s", e.Error.Code)
		}
	})

	t.Run("invalid input is 422 naming the field", func(t *testing.T) {
		rec := send(t, app, http.MethodPost, rules,
			`{"scope":"sku","markup_bp":-5,"min_margin_cents":0,"rounding":"none"}`, admin)
		e := decode[Error](t, rec, http.StatusUnprocessableEntity)
		want := map[string]string{"sku": "required", "markup_bp": "out_of_range"}
		for _, f := range e.Error.Fields {
			if want[f.Field] == f.Code {
				delete(want, f.Field)
			}
		}
		if len(want) != 0 {
			t.Errorf("fields = %+v, missing %v", e.Error.Fields, want)
		}
	})

	t.Run("update checks the version", func(t *testing.T) {
		body := fmt.Sprintf(`{"scope":"brand_category","category":"gpu","brand":"Gigabyte","markup_bp":2000,"min_margin_cents":0,"rounding":"none","version":%d}`, created.Version)
		updated := decode[MarkupRule](t, send(t, app, http.MethodPut, ruleURL(created.Id), body, admin), http.StatusOK)
		if updated.Version != 2 || updated.MarkupBp != 2000 {
			t.Errorf("updated = %+v", updated)
		}
		// The same body again starts from version 1, which is stale now.
		e := decode[Error](t, send(t, app, http.MethodPut, ruleURL(created.Id), body, admin), http.StatusConflict)
		if e.Error.Code != ErrorCodeEditConflict {
			t.Errorf("code = %s, want edit_conflict", e.Error.Code)
		}
	})

	t.Run("the default rule stays", func(t *testing.T) {
		body := fmt.Sprintf(`{"scope":"category","category":"gpu","markup_bp":0,"min_margin_cents":0,"rounding":"none","version":%d}`, def.Version)
		if e := decode[Error](t, send(t, app, http.MethodPut, ruleURL(def.Id), body, admin), http.StatusConflict); e.Error.Code != ErrorCodeDefaultRule {
			t.Errorf("rescope: code = %s", e.Error.Code)
		}
		if e := decode[Error](t, send(t, app, http.MethodDelete, ruleURL(def.Id), "", admin), http.StatusConflict); e.Error.Code != ErrorCodeDefaultRule {
			t.Errorf("delete: code = %s", e.Error.Code)
		}
	})

	t.Run("delete", func(t *testing.T) {
		if rec := send(t, app, http.MethodDelete, ruleURL(created.Id), "", admin); rec.Code != http.StatusNoContent {
			t.Fatalf("status %d: %s", rec.Code, rec.Body)
		}
		if rec := send(t, app, http.MethodDelete, ruleURL(created.Id), "", admin); rec.Code != http.StatusNotFound {
			t.Errorf("again: status %d, want 404", rec.Code)
		}
		if list := decode[MarkupRuleList](t, get(t, app, rules, admin), http.StatusOK); len(list.Items) != 1 {
			t.Errorf("list = %+v, want only the default", list.Items)
		}
	})
}
