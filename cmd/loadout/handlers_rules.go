package main

import (
	"net/http"

	"github.com/gfotev/loadout/internal/catalog"
	"github.com/gfotev/loadout/internal/money"
)

// ListMarkupRules handles GET /api/admin/markup-rules.
func (app *application) ListMarkupRules(w http.ResponseWriter, r *http.Request) {
	rules, err := app.catalog.MarkupRules(r.Context())
	if err != nil {
		app.handleError(w, r, err)
		return
	}
	resp := MarkupRuleList{Items: make([]ListedMarkupRule, 0, len(rules))}
	for _, rule := range rules {
		j := markupRuleJSON(rule.MarkupRule)
		resp.Items = append(resp.Items, ListedMarkupRule{
			Id:             j.Id,
			Scope:          j.Scope,
			Category:       j.Category,
			Brand:          j.Brand,
			Sku:            j.Sku,
			MarkupBp:       j.MarkupBp,
			MinMarginCents: j.MinMarginCents,
			Rounding:       j.Rounding,
			Version:        j.Version,
			UpdatedAt:      j.UpdatedAt,
			Products:       rule.Products,
		})
	}
	app.writeJSON(w, r, http.StatusOK, resp)
}

// CreateMarkupRule handles POST /api/admin/markup-rules.
func (app *application) CreateMarkupRule(w http.ResponseWriter, r *http.Request) {
	var in MarkupRuleInput
	if err := readJSON(w, r, &in); err != nil {
		app.handleError(w, r, err)
		return
	}
	rule, err := app.catalog.CreateMarkupRule(r.Context(), catalog.RuleInput{
		Scope:          catalog.RuleScope(in.Scope),
		Category:       string(in.Category),
		Brand:          in.Brand,
		SKU:            in.Sku,
		MarkupBP:       in.MarkupBp,
		MinMarginCents: money.Cents(in.MinMarginCents),
		Rounding:       catalog.Rounding(in.Rounding),
	})
	if err != nil {
		app.handleError(w, r, err)
		return
	}
	app.writeJSON(w, r, http.StatusCreated, markupRuleJSON(rule))
}

// UpdateMarkupRule handles PUT /api/admin/markup-rules/{rule_id}.
func (app *application) UpdateMarkupRule(w http.ResponseWriter, r *http.Request, ruleID int64) {
	var in MarkupRuleUpdate
	if err := readJSON(w, r, &in); err != nil {
		app.handleError(w, r, err)
		return
	}
	rule, err := app.catalog.UpdateMarkupRule(r.Context(), ruleID, in.Version, catalog.RuleInput{
		Scope:          catalog.RuleScope(in.Scope),
		Category:       string(in.Category),
		Brand:          in.Brand,
		SKU:            in.Sku,
		MarkupBP:       in.MarkupBp,
		MinMarginCents: money.Cents(in.MinMarginCents),
		Rounding:       catalog.Rounding(in.Rounding),
	})
	if err != nil {
		app.handleError(w, r, err)
		return
	}
	app.writeJSON(w, r, http.StatusOK, markupRuleJSON(rule))
}

// DeleteMarkupRule handles DELETE /api/admin/markup-rules/{rule_id}.
func (app *application) DeleteMarkupRule(w http.ResponseWriter, r *http.Request, ruleID int64) {
	if err := app.catalog.DeleteMarkupRule(r.Context(), ruleID); err != nil {
		app.handleError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func markupRuleJSON(rule catalog.MarkupRule) MarkupRule {
	return MarkupRule{
		Id:             rule.ID,
		Scope:          RuleScope(rule.Scope),
		Category:       Category(rule.Category),
		Brand:          rule.Brand,
		Sku:            rule.SKU,
		MarkupBp:       rule.MarkupBP,
		MinMarginCents: int64(rule.MinMarginCents),
		Rounding:       Rounding(rule.Rounding),
		Version:        rule.Version,
		UpdatedAt:      rule.UpdatedAt,
	}
}
