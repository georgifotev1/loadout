package orders

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/google/uuid"

	"github.com/gfotev/loadout/internal/catalog"
	"github.com/gfotev/loadout/internal/configurator"
	"github.com/gfotev/loadout/internal/money"
)

func TestSelection(t *testing.T) {
	pc := []buildPart{
		{"cpu", "cpu", 1},
		{"motherboard", "mb", 1},
		{"ram", "ram", 2},
		{"storage", "ssd", 1},
		{"psu", "psu", 1},
		{"case", "case", 1},
		{"peripheral.mouse", "mouse", 3},
	}
	sel, ok := selection(pc)
	if !ok {
		t.Fatal("a PC with a mouse isn't one build")
	}
	if sel.CPU != "cpu" || sel.Motherboard != "mb" || !slices.Equal(sel.RAM, []string{"ram", "ram"}) ||
		!slices.Equal(sel.Storage, []string{"ssd"}) || sel.PSU != "psu" || sel.Case != "case" || sel.GPU != "" {
		t.Errorf("selection %+v", sel)
	}

	for _, tc := range []struct {
		name  string
		lines []buildPart
	}{
		{"no PC parts", []buildPart{{"peripheral.mouse", "mouse", 1}}},
		{"nothing", nil},
		{"two of one CPU", []buildPart{{"cpu", "cpu", 2}}},
		{"two CPUs", []buildPart{{"cpu", "a", 1}, {"cpu", "b", 1}}},
		{"too many RAM kits", []buildPart{{"ram", "a", 3}, {"ram", "b", 2}}},
		{"too many drives", []buildPart{{"storage", "ssd", configurator.MaxDrives + 1}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, ok := selection(tc.lines); ok {
				t.Error("taken as one build")
			}
		})
	}
}

// fakeBuilds passes a build when every part's slug is in ok, as if the rest were missing or
// didn't fit.
type fakeBuilds struct {
	ok []string
}

func (f *fakeBuilds) Validate(_ context.Context, sel configurator.Selection) (configurator.Validation, error) {
	var v configurator.Validation
	for _, slug := range slices.Concat([]string{sel.CPU, sel.Motherboard, sel.GPU, sel.PSU, sel.Case, sel.Cooler}, sel.RAM, sel.Storage) {
		if slug != "" && !slices.Contains(f.ok, slug) {
			v.Errors = append(v.Errors, configurator.Finding{Rule: configurator.RulePartRequired})
		}
	}
	v.Assembly.Offered = v.OK()
	return v, nil
}

// categoryPart is a part in stock with a category.
func categoryPart(slug, category string, cents money.Cents) catalog.Part {
	p := part(slug, cents, 5)
	p.Category = category
	return p
}

func TestCheckoutWithAssembly(t *testing.T) {
	s, cat := newCheckoutService(t)
	cat.parts = append(cat.parts,
		categoryPart("pc-cpu", "cpu", 200_00),
		categoryPart("pc-ram", "ram", 80_00),
		categoryPart("mouse", "peripheral.mouse", 40_00),
	)
	s.builds = &fakeBuilds{ok: []string{"pc-cpu", "pc-ram"}}
	ctx := context.Background()
	owner := guestCart(t, s,
		AddLine{Slug: "pc-ram", Quantity: 2},
		AddLine{Slug: "mouse", Quantity: 1},
	)
	fee := configurator.AssemblyCents

	without, err := s.Quote(ctx, owner)
	if err != nil {
		t.Fatal(err)
	}
	if without.Assembly.Chosen || without.Cart.Assembly.Requested || without.TotalCents != 200_00+DeliveryCents {
		t.Errorf("not asked for: %+v", without.Assembly)
	}

	// The builder's add to cart asks for assembly with the parts.
	c, _, err := s.AddToCart(ctx, owner, []AddLine{{Slug: "pc-cpu", Quantity: 1}}, true)
	if err != nil {
		t.Fatal(err)
	}
	if !c.Assembly.Chosen() || c.TotalCents != 400_00+fee || c.Assembly.BuildCents != 360_00+fee {
		t.Errorf("cart %+v, assembly %+v", c, c.Assembly)
	}
	for _, l := range c.Lines {
		if l.InBuild != (l.Slug != "mouse") {
			t.Errorf("%s: in build %v", l.Slug, l.InBuild)
		}
	}
	with, err := s.Quote(ctx, owner)
	if err != nil {
		t.Fatal(err)
	}
	if !with.Assembly.Chosen || with.TotalCents != 400_00+DeliveryCents+fee {
		t.Errorf("with assembly %+v, total %d", with.Assembly, with.TotalCents)
	}

	t.Run("the total the customer saw must have the fee", func(t *testing.T) {
		_, err := s.Checkout(ctx, owner, checkoutInput(with.TotalCents-fee))
		if got := fieldCodes(t, err); !slices.Equal(got, []string{"expected_total_cents: changed"}) {
			t.Errorf("errors %v", got)
		}
	})

	placed, err := s.Checkout(ctx, owner, checkoutInput(with.TotalCents))
	if err != nil {
		t.Fatal(err)
	}
	o, err := s.Order(ctx, placed.ID, Viewer{Token: placed.Token})
	if err != nil {
		t.Fatal(err)
	}
	if !o.Assembly || o.AssemblyCents != fee || o.BuildCents != 360_00+fee || o.SubtotalCents != 400_00 || o.TotalCents != with.TotalCents {
		t.Errorf("order %+v", o)
	}
	// The PC's parts first, in the builder's order; the mouse after them.
	if got := []string{o.Lines[0].Slug, o.Lines[1].Slug, o.Lines[2].Slug}; !slices.Equal(got, []string{"pc-cpu", "pc-ram", "mouse"}) {
		t.Errorf("lines in order %v", got)
	}
	var built []string
	for _, l := range o.Lines {
		if l.InBuild {
			built = append(built, l.Slug)
		}
	}
	slices.Sort(built)
	if !slices.Equal(built, []string{"pc-cpu", "pc-ram"}) {
		t.Errorf("in the build: %v; the mouse ships as it is", built)
	}
}

func TestCartAssembly(t *testing.T) {
	s, cat := newCheckoutService(t)
	cat.parts = append(cat.parts,
		categoryPart("pc-cpu", "cpu", 200_00),
		categoryPart("mouse", "peripheral.mouse", 40_00),
	)
	s.builds = &fakeBuilds{ok: []string{"pc-cpu"}}
	ctx := context.Background()

	if _, err := s.SetCartAssembly(ctx, GuestCart(""), true); !errors.Is(err, ErrNotFound) {
		t.Errorf("without a cart: %v", err)
	}

	owner := guestCart(t, s, AddLine{Slug: "pc-cpu", Quantity: 2})
	c, err := s.SetCartAssembly(ctx, owner, true)
	if err != nil {
		t.Fatal(err)
	}
	if !c.Assembly.Requested || c.Assembly.Offered || c.TotalCents != c.SubtotalCents {
		t.Errorf("two CPUs: assembly %+v, total %d", c.Assembly, c.TotalCents)
	}

	t.Run("checkout doesn't assemble what isn't offered", func(t *testing.T) {
		q, err := s.Quote(ctx, owner)
		if err != nil {
			t.Fatal(err)
		}
		if q.Assembly.Chosen || q.TotalCents != 400_00+DeliveryCents {
			t.Fatalf("quote %+v", q.Assembly)
		}
		placed, err := s.Checkout(ctx, owner, checkoutInput(q.TotalCents))
		if err != nil {
			t.Fatal(err)
		}
		o, err := s.Order(ctx, placed.ID, Viewer{Token: placed.Token})
		if err != nil {
			t.Fatal(err)
		}
		if o.Assembly || o.Lines[0].InBuild {
			t.Errorf("order %+v", o)
		}
	})

	t.Run("asking stops", func(t *testing.T) {
		owner := guestCart(t, s, AddLine{Slug: "pc-cpu", Quantity: 1})
		if c, err := s.SetCartAssembly(ctx, owner, true); err != nil || !c.Assembly.Chosen() {
			t.Fatalf("cart %+v, %v", c, err)
		}
		c, err := s.SetCartAssembly(ctx, owner, false)
		if err != nil || c.Assembly.Requested || !c.Assembly.Offered || c.TotalCents != c.SubtotalCents {
			t.Errorf("cart %+v, %v", c, err)
		}
	})

	t.Run("logging in keeps the request", func(t *testing.T) {
		guest := guestCart(t, s, AddLine{Slug: "pc-cpu", Quantity: 1})
		if _, err := s.SetCartAssembly(ctx, guest, true); err != nil {
			t.Fatal(err)
		}
		user := UserCart(uuid.New())
		if _, _, err := s.AddToCart(ctx, user, []AddLine{{Slug: "mouse", Quantity: 1}}, false); err != nil {
			t.Fatal(err)
		}
		if err := s.MergeCart(ctx, guest.Token, user.UserID); err != nil {
			t.Fatal(err)
		}
		c, err := s.Cart(ctx, user)
		if err != nil || !c.Assembly.Chosen() {
			t.Errorf("cart %+v, %v", c, err)
		}
	})
}
