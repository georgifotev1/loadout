package catalog

import (
	"testing"

	"github.com/gfotev/loadout/internal/money"
)

func TestResolveRule(t *testing.T) {
	gpu := Product{ProviderSKU: "GA-VC-N5070WF3-12GD", Brand: "GIGABYTE", Category: "gpu"}

	def := MarkupRule{ID: 1, Scope: ScopeDefault}
	cat := MarkupRule{ID: 2, Scope: ScopeCategory, Category: "gpu"}
	otherCat := MarkupRule{ID: 3, Scope: ScopeCategory, Category: "cpu"}
	brand := MarkupRule{ID: 4, Scope: ScopeBrandCategory, Category: "gpu", Brand: "Gigabyte"}
	brandOtherCat := MarkupRule{ID: 5, Scope: ScopeBrandCategory, Category: "motherboard", Brand: "GIGABYTE"}
	sku := MarkupRule{ID: 6, Scope: ScopeSKU, SKU: "GA-VC-N5070WF3-12GD"}
	otherSKU := MarkupRule{ID: 7, Scope: ScopeSKU, SKU: "ZOTAC-VC-N5060-8GB-TW"}

	tests := []struct {
		name   string
		rules  []MarkupRule
		wantID int64
		wantOK bool
	}{
		{"no rules", nil, 0, false},
		{"nothing matches", []MarkupRule{otherCat, otherSKU, brandOtherCat}, 0, false},
		{"default only", []MarkupRule{def}, 1, true},
		{"category beats default", []MarkupRule{def, cat}, 2, true},
		{"other category is ignored", []MarkupRule{def, otherCat}, 1, true},
		{"brand+category beats category, brand case-insensitive", []MarkupRule{def, cat, brand}, 4, true},
		{"brand in another category is ignored", []MarkupRule{cat, brandOtherCat}, 2, true},
		{"sku beats everything", []MarkupRule{sku, brand, cat, def}, 6, true},
		{"other sku is ignored", []MarkupRule{def, otherSKU}, 1, true},
		{"higher priority wins within a scope", []MarkupRule{
			{ID: 10, Scope: ScopeBrandCategory, Category: "gpu", Brand: "GIGABYTE", Priority: 0},
			{ID: 11, Scope: ScopeBrandCategory, Category: "gpu", Brand: "gigabyte", Priority: 5},
		}, 11, true},
		{"priority never beats specificity", []MarkupRule{
			{ID: 20, Scope: ScopeDefault, Priority: 100},
			{ID: 21, Scope: ScopeCategory, Category: "gpu"},
		}, 21, true},
		{"full tie: lower id wins, whatever the order", []MarkupRule{
			{ID: 31, Scope: ScopeBrandCategory, Category: "gpu", Brand: "gigabyte"},
			{ID: 30, Scope: ScopeBrandCategory, Category: "gpu", Brand: "GIGABYTE"},
		}, 30, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ResolveRule(tt.rules, gpu)
			if ok != tt.wantOK || got.ID != tt.wantID {
				t.Errorf("got rule %d ok=%v, want %d ok=%v", got.ID, ok, tt.wantID, tt.wantOK)
			}
		})
	}
}

func TestRetailPrice(t *testing.T) {
	tests := []struct {
		name   string
		dealer money.Cents
		rule   MarkupRule
		want   money.Cents
	}{
		{"no markup", 5235, MarkupRule{Rounding: RoundNone}, 5235},
		{"20% exact", 10000, MarkupRule{MarkupBP: 2000, Rounding: RoundNone}, 12000},
		{"rounds half up", 5235, MarkupRule{MarkupBP: 2000, Rounding: RoundNone}, 6282},   // 6282.0
		{"rounds .5 up", 25, MarkupRule{MarkupBP: 1000, Rounding: RoundNone}, 28},         // 27.5
		{"rounds below .5 down", 13, MarkupRule{MarkupBP: 1000, Rounding: RoundNone}, 14}, // 14.3
		{"min margin takes over on a cheap item", 1000, MarkupRule{MarkupBP: 1500, MinMarginCents: 500, Rounding: RoundNone}, 1500},
		{"min margin ignored when markup is bigger", 100000, MarkupRule{MarkupBP: 1500, MinMarginCents: 500, Rounding: RoundNone}, 115000},
		{".99 rounds up within the euro", 6282, MarkupRule{MarkupBP: 0, Rounding: Round99}, 6299},
		{".99 already on the ending stays", 1299, MarkupRule{Rounding: Round99}, 1299},
		{".99 from a round euro", 1300, MarkupRule{Rounding: Round99}, 1399},
		{".90 rounds up within the euro", 1250, MarkupRule{Rounding: Round90}, 1290},
		{".90 already on the ending stays", 1290, MarkupRule{Rounding: Round90}, 1290},
		{".90 past the ending goes to the next euro", 1295, MarkupRule{Rounding: Round90}, 1390},
		{"rounding after the margin floor keeps the margin", 1000, MarkupRule{MarkupBP: 0, MinMarginCents: 495, Rounding: Round90}, 1590},
		{"zero dealer price", 0, MarkupRule{MarkupBP: 2000, Rounding: Round99}, 99},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RetailPrice(tt.dealer, tt.rule)
			if got != tt.want {
				t.Errorf("RetailPrice(%d, %+v) = %d, want %d", tt.dealer, tt.rule, got, tt.want)
			}
			if got < tt.dealer+tt.rule.MinMarginCents {
				t.Errorf("price %d is below dealer + min margin", got)
			}
		})
	}
}

func TestMargin(t *testing.T) {
	tests := []struct {
		dealer, retail money.Cents
		wantCents      money.Cents
		wantBP         int
	}{
		{5000, 6250, 1250, 2000},
		{10000, 10000, 0, 0},
		{2999, 3990, 991, 2484}, // 24.837% rounds to 2484 bp
		{0, 0, 0, 0},            // a free product doesn't divide by zero
	}
	for _, tt := range tests {
		p := Price{DealerCents: tt.dealer, RetailCents: tt.retail}
		if got := p.MarginCents(); got != tt.wantCents {
			t.Errorf("%d → %d: margin %d, want %d", tt.dealer, tt.retail, got, tt.wantCents)
		}
		if got := p.MarginBP(); got != tt.wantBP {
			t.Errorf("%d → %d: margin %d bp, want %d", tt.dealer, tt.retail, got, tt.wantBP)
		}
	}
}
