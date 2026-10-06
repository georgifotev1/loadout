package catalog

import (
	"context"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"

	"github.com/gfotev/loadout/internal/money"
	"github.com/gfotev/loadout/internal/platform/db"
	"github.com/gfotev/loadout/internal/platform/validate"
)

const (
	// MaxMarkupBP caps a rule's markup: +1000%.
	MaxMarkupBP = 100_000
	// MaxMinMarginCents caps a rule's minimum margin: €10,000.
	MaxMinMarginCents money.Cents = 1_000_000
	// maxRuleTargetLen caps a rule's brand and SKU.
	maxRuleTargetLen = 100
)

// RuleInput is what an admin sets on a markup rule. Only the target fields its Scope matches on
// may be set. Priority isn't editable: one rule per target leaves nothing for it to break.
type RuleInput struct {
	Scope          RuleScope
	Category       string
	Brand          string
	SKU            string
	MarkupBP       int
	MinMarginCents money.Cents
	Rounding       Rounding
}

// MarkupRules returns every markup rule with how many products it prices, from the least to the
// most specific scope.
func (s *Service) MarkupRules(ctx context.Context) ([]ListedRule, error) {
	return s.store.listedRules(ctx, s.pool)
}

// CreateMarkupRule adds a rule. It applies from the next sync. A rule with the same target is
// ErrDuplicateRule; invalid input is a *validate.Error.
func (s *Service) CreateMarkupRule(ctx context.Context, in RuleInput) (MarkupRule, error) {
	in = normalizeRule(in)
	if err := checkRule(in); err != nil {
		return MarkupRule{}, err
	}
	return s.store.insertMarkupRule(ctx, s.pool, ruleFrom(0, in))
}

// UpdateMarkupRule replaces rule id's target and pricing, if it's still at version: a newer
// edit is ErrEditConflict, so a stale form can't overwrite it. The default rule must stay
// default (ErrDefaultRule). It applies from the next sync.
func (s *Service) UpdateMarkupRule(ctx context.Context, id int64, version int, in RuleInput) (MarkupRule, error) {
	in = normalizeRule(in)
	if err := checkRule(in); err != nil {
		return MarkupRule{}, err
	}

	var updated MarkupRule
	err := db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		cur, err := s.store.markupRuleForUpdate(ctx, tx, id)
		if err != nil {
			return err
		}
		if cur.Version != version {
			return ErrEditConflict
		}
		if (cur.Scope == ScopeDefault) != (in.Scope == ScopeDefault) {
			return ErrDefaultRule
		}
		updated, err = s.store.updateMarkupRule(ctx, tx, ruleFrom(id, in))
		return err
	})
	return updated, err
}

// DeleteMarkupRule deletes a rule; the default rule can't be (ErrDefaultRule). Prices it made
// stay until the next sync reprices them with the rule that matches then.
func (s *Service) DeleteMarkupRule(ctx context.Context, id int64) error {
	return db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		cur, err := s.store.markupRuleForUpdate(ctx, tx, id)
		if err != nil {
			return err
		}
		if cur.Scope == ScopeDefault {
			return ErrDefaultRule
		}
		return s.store.deleteMarkupRule(ctx, tx, id)
	})
}

func normalizeRule(in RuleInput) RuleInput {
	in.Category = strings.TrimSpace(in.Category)
	in.Brand = strings.TrimSpace(in.Brand)
	in.SKU = strings.TrimSpace(in.SKU)
	return in
}

// checkRule validates an admin's rule. Each scope needs exactly its own target fields, like the
// markup_rules_scope_fields CHECK, but with an error naming the field.
func checkRule(in RuleInput) error {
	var v validate.Validator
	scopes := []RuleScope{ScopeDefault, ScopeCategory, ScopeBrandCategory, ScopeSKU}
	v.Check(slices.Contains(scopes, in.Scope), "scope", "unknown")

	needsCategory := in.Scope == ScopeCategory || in.Scope == ScopeBrandCategory
	switch {
	case needsCategory && in.Category == "":
		v.Add("category", "required")
	case needsCategory && !IsCategory(in.Category):
		v.Add("category", "unknown")
	case !needsCategory && in.Category != "":
		v.Add("category", "not_allowed")
	}
	checkTarget(&v, "brand", in.Brand, in.Scope == ScopeBrandCategory)
	checkTarget(&v, "sku", in.SKU, in.Scope == ScopeSKU)

	v.Check(in.MarkupBP >= 0 && in.MarkupBP <= MaxMarkupBP, "markup_bp", "out_of_range")
	v.Check(in.MinMarginCents >= 0 && in.MinMarginCents <= MaxMinMarginCents, "min_margin_cents", "out_of_range")
	v.Check(slices.Contains([]Rounding{RoundNone, Round99, Round90}, in.Rounding), "rounding", "unknown")
	return v.Err()
}

func checkTarget(v *validate.Validator, field, value string, needed bool) {
	switch {
	case needed && value == "":
		v.Add(field, "required")
	case !needed && value != "":
		v.Add(field, "not_allowed")
	case utf8.RuneCountInString(value) > maxRuleTargetLen:
		v.Add(field, "too_long")
	}
}

func ruleFrom(id int64, in RuleInput) MarkupRule {
	return MarkupRule{
		ID:             id,
		Scope:          in.Scope,
		Category:       in.Category,
		Brand:          in.Brand,
		SKU:            in.SKU,
		MarkupBP:       in.MarkupBP,
		MinMarginCents: in.MinMarginCents,
		Rounding:       in.Rounding,
	}
}
