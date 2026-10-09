package orders

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/gfotev/loadout/internal/catalog"
	"github.com/gfotev/loadout/internal/money"
	"github.com/gfotev/loadout/internal/platform/db/dbtest"
	"github.com/gfotev/loadout/internal/platform/validate"
)

// fakeCatalog sells the parts in it. Tests change them to hide a product or change a price.
type fakeCatalog struct {
	mu    sync.Mutex
	parts []catalog.Part
}

func (f *fakeCatalog) PartsByIDs(_ context.Context, ids []uuid.UUID) ([]catalog.Part, error) {
	return f.find(func(p catalog.Part) bool { return slices.Contains(ids, p.ID) }), nil
}

func (f *fakeCatalog) PartsBySlugs(_ context.Context, slugs []string) ([]catalog.Part, error) {
	return f.find(func(p catalog.Part) bool { return slices.Contains(slugs, p.Slug) }), nil
}

// SaleParts sells every part at a dealer price of 80% of retail, with the slug as the SKU.
func (f *fakeCatalog) SaleParts(_ context.Context, ids []uuid.UUID) ([]catalog.SalePart, error) {
	var out []catalog.SalePart
	for _, p := range f.find(func(p catalog.Part) bool { return slices.Contains(ids, p.ID) }) {
		out = append(out, catalog.SalePart{
			Part:        p,
			Provider:    "vali",
			ProviderSKU: strings.ToUpper(p.Slug),
			DealerCents: p.RetailCents * 8 / 10,
			VATbp:       money.VATStandardBP,
		})
	}
	return out, nil
}

func (f *fakeCatalog) find(match func(catalog.Part) bool) []catalog.Part {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []catalog.Part
	for _, p := range f.parts {
		if match(p) {
			out = append(out, p)
		}
	}
	return out
}

func (f *fakeCatalog) update(slug string, fn func(*catalog.Part)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := range f.parts {
		if f.parts[i].Slug == slug {
			fn(&f.parts[i])
		}
	}
}

func (f *fakeCatalog) remove(slug string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.parts = slices.DeleteFunc(f.parts, func(p catalog.Part) bool { return p.Slug == slug })
}

func part(slug string, cents money.Cents, stock int) catalog.Part {
	return catalog.Part{ID: uuid.New(), ProductSummary: catalog.ProductSummary{
		Slug:        slug,
		Name:        "Product " + slug,
		RetailCents: cents,
		StockQty:    stock,
	}}
}

func newCartService(t *testing.T) (*Service, *fakeCatalog) {
	t.Helper()
	cat := &fakeCatalog{parts: []catalog.Part{
		part("cpu", 200_00, 5),
		part("ram", 80_00, 5),
		part("gpu", 500_00, 0),
	}}
	return NewService(dbtest.New(t), cat, nil, nil), cat // the cart queues no jobs
}

// fieldCodes lists a validation error's fields as "field: code".
func fieldCodes(t *testing.T, err error) []string {
	t.Helper()
	var verr *validate.Error
	if !errors.As(err, &verr) {
		t.Fatalf("err %v, want a validation error", err)
	}
	var out []string
	for _, f := range verr.Fields {
		out = append(out, f.Field+": "+f.Code)
	}
	return out
}

func TestCart(t *testing.T) {
	s, cat := newCartService(t)
	ctx := context.Background()

	t.Run("no token is an empty cart", func(t *testing.T) {
		for _, token := range []string{"", "unknown"} {
			c, err := s.Cart(ctx, GuestCart(token))
			if err != nil {
				t.Fatal(err)
			}
			if len(c.Lines) != 0 || c.Count != 0 || c.SubtotalCents != 0 {
				t.Errorf("token %q: cart %+v", token, c)
			}
		}
	})

	// The first add starts a cart; later ones add to it, on top of what's there.
	c, token, err := s.AddToCart(ctx, GuestCart(""), []AddLine{{Slug: "cpu", Quantity: 1}}, false)
	if err != nil {
		t.Fatal(err)
	}
	if token == "" || c.Count != 1 || c.SubtotalCents != 200_00 {
		t.Fatalf("token %q, cart %+v", token, c)
	}
	c, again, err := s.AddToCart(ctx, GuestCart(token), []AddLine{{Slug: "ram", Quantity: 2}, {Slug: "cpu", Quantity: 1}}, false)
	if err != nil {
		t.Fatal(err)
	}
	if again != token {
		t.Errorf("token changed to %q", again)
	}
	if c.Count != 4 || c.SubtotalCents != 2*200_00+2*80_00 {
		t.Errorf("cart %+v", c)
	}
	if got := slugsOf(c); !slices.Equal(got, []string{"ram", "cpu"}) && !slices.Equal(got, []string{"cpu", "ram"}) {
		t.Errorf("lines %v", got)
	}

	t.Run("an unknown token starts a new cart", func(t *testing.T) {
		_, other, err := s.AddToCart(ctx, GuestCart("unknown"), []AddLine{{Slug: "cpu", Quantity: 1}}, false)
		if err != nil {
			t.Fatal(err)
		}
		if other == "unknown" || other == token {
			t.Errorf("token %q", other)
		}
	})

	t.Run("invalid adds change nothing", func(t *testing.T) {
		tests := []struct {
			name  string
			lines []AddLine
			want  []string
		}{
			{"nothing", nil, []string{"lines: required"}},
			{"bad quantity", []AddLine{{Slug: "cpu", Quantity: 0}}, []string{"lines[0].quantity: out_of_range"}},
			{"not for sale", []AddLine{{Slug: "cpu", Quantity: 1}, {Slug: "nope", Quantity: 1}}, []string{"lines[1].slug: unavailable"}},
			{"out of stock", []AddLine{{Slug: "gpu", Quantity: 1}}, []string{"lines[0].slug: out_of_stock"}},
			// The cart has 2 CPUs: 2 + 5 + 4 is over 10, though each line alone isn't.
			{"over the limit with the cart", []AddLine{{Slug: "cpu", Quantity: 5}, {Slug: "cpu", Quantity: 4}},
				[]string{"lines[0].quantity: too_many", "lines[1].quantity: too_many"}},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				_, _, err := s.AddToCart(ctx, GuestCart(token), tt.lines, false)
				if got := fieldCodes(t, err); !slices.Equal(got, tt.want) {
					t.Errorf("fields %v, want %v", got, tt.want)
				}
				c, err := s.Cart(ctx, GuestCart(token))
				if err != nil {
					t.Fatal(err)
				}
				if c.Count != 4 {
					t.Errorf("count %d after a failed add", c.Count)
				}
			})
		}
	})

	cpu := lineOf(t, c, "cpu").ProductID

	t.Run("set quantity", func(t *testing.T) {
		c, err := s.SetCartQuantity(ctx, GuestCart(token), cpu, 3)
		if err != nil {
			t.Fatal(err)
		}
		if lineOf(t, c, "cpu").Quantity != 3 || c.Count != 5 {
			t.Errorf("cart %+v", c)
		}
		if _, err := s.SetCartQuantity(ctx, GuestCart(token), cpu, MaxQuantity+1); fieldCodes(t, err)[0] != "quantity: out_of_range" {
			t.Errorf("err %v", err)
		}
		if _, err := s.SetCartQuantity(ctx, GuestCart(token), uuid.New(), 1); !errors.Is(err, ErrNotFound) {
			t.Errorf("unknown product: err %v", err)
		}
		if _, err := s.SetCartQuantity(ctx, GuestCart(""), cpu, 1); !errors.Is(err, ErrNotFound) {
			t.Errorf("no cart: err %v", err)
		}
	})

	t.Run("a product no longer for sale stays, out of the subtotal", func(t *testing.T) {
		cat.update("ram", func(p *catalog.Part) { p.Slug, p.Name = "ram-renamed", "RAM renamed" })
		c, err := s.Cart(ctx, GuestCart(token))
		if err != nil {
			t.Fatal(err)
		}
		if l := lineOf(t, c, "ram-renamed"); l.Name != "RAM renamed" || l.Product == nil {
			t.Errorf("renamed line %+v", l)
		}

		cat.remove("ram-renamed")
		c, err = s.Cart(ctx, GuestCart(token))
		if err != nil {
			t.Fatal(err)
		}
		l := lineOf(t, c, "ram") // as it was added
		if l.Product != nil || l.LineCents != 0 || l.Name != "Product ram" {
			t.Errorf("gone line %+v", l)
		}
		if c.Count != 5 || c.SubtotalCents != 3*200_00 {
			t.Errorf("count %d, subtotal %d", c.Count, c.SubtotalCents)
		}
	})

	t.Run("remove", func(t *testing.T) {
		ram := lineOf(t, c, "ram").ProductID
		for range 2 { // removing twice is fine
			c, err := s.RemoveFromCart(ctx, GuestCart(token), ram)
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(slugsOf(c), []string{"cpu"}) {
				t.Errorf("lines %v", slugsOf(c))
			}
		}
	})

	t.Run("empty", func(t *testing.T) {
		if _, err := s.SetCartAssembly(ctx, GuestCart(token), true); err != nil {
			t.Fatal(err)
		}
		for range 2 { // emptying twice is fine
			c, err := s.EmptyCart(ctx, GuestCart(token))
			if err != nil {
				t.Fatal(err)
			}
			if len(c.Lines) != 0 || c.Count != 0 || c.TotalCents != 0 || c.Assembly.Requested {
				t.Errorf("cart %+v", c)
			}
		}
		if _, err := s.EmptyCart(ctx, GuestCart("")); err != nil {
			t.Errorf("no cart: err %v", err)
		}
	})
}

// Two adds to one cart at once can't both pass the quantity limit: the cart row lock makes the
// second wait and see the first's lines.
func TestAddToCartConcurrently(t *testing.T) {
	s, _ := newCartService(t)
	ctx := context.Background()
	_, token, err := s.AddToCart(ctx, GuestCart(""), []AddLine{{Slug: "cpu", Quantity: 1}}, false)
	if err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := range errs {
		wg.Go(func() {
			_, _, errs[i] = s.AddToCart(ctx, GuestCart(token), []AddLine{{Slug: "cpu", Quantity: 5}}, false)
		})
	}
	wg.Wait()

	failed := 0
	for _, err := range errs {
		if err != nil {
			if got := fieldCodes(t, err); !slices.Equal(got, []string{"lines[0].quantity: too_many"}) {
				t.Errorf("fields %v", got)
			}
			failed++
		}
	}
	c, err := s.Cart(ctx, GuestCart(token))
	if err != nil {
		t.Fatal(err)
	}
	if failed != 1 || c.Count != 6 {
		t.Errorf("%d failed, count %d; want 1 and 6", failed, c.Count)
	}
}

func TestUserCart(t *testing.T) {
	s, _ := newCartService(t)
	ctx := context.Background()
	user := UserCart(uuid.New())

	// A user's cart has no token: it's found by the user.
	c, token, err := s.AddToCart(ctx, user, []AddLine{{Slug: "cpu", Quantity: 1}}, false)
	if err != nil {
		t.Fatal(err)
	}
	if token != "" || c.Count != 1 {
		t.Fatalf("token %q, cart %+v", token, c)
	}
	if c, err = s.SetCartQuantity(ctx, user, lineOf(t, c, "cpu").ProductID, 2); err != nil || c.Count != 2 {
		t.Fatalf("cart %+v, err %v", c, err)
	}
	if c, err = s.Cart(ctx, UserCart(uuid.New())); err != nil || c.Count != 0 {
		t.Errorf("another user's cart %+v, err %v", c, err)
	}
}

func TestMergeCart(t *testing.T) {
	s, cat := newCartService(t)
	ctx := context.Background()
	guest := func(lines ...AddLine) string {
		t.Helper()
		_, token, err := s.AddToCart(ctx, GuestCart(""), lines, false)
		if err != nil {
			t.Fatal(err)
		}
		return token
	}
	merge := func(token string, user CartOwner) Cart {
		t.Helper()
		if err := s.MergeCart(ctx, token, user.UserID); err != nil {
			t.Fatal(err)
		}
		// The guest cart is gone either way.
		if c, err := s.Cart(ctx, GuestCart(token)); err != nil || c.Count != 0 {
			t.Errorf("guest cart after merge %+v, err %v", c, err)
		}
		c, err := s.Cart(ctx, user)
		if err != nil {
			t.Fatal(err)
		}
		return c
	}

	t.Run("a user without a cart takes the guest's over", func(t *testing.T) {
		user := UserCart(uuid.New())
		c := merge(guest(AddLine{Slug: "cpu", Quantity: 2}), user)
		if c.Count != 2 || lineOf(t, c, "cpu").Quantity != 2 {
			t.Errorf("cart %+v", c)
		}
	})

	t.Run("lines add up, to the limit", func(t *testing.T) {
		user := UserCart(uuid.New())
		if _, _, err := s.AddToCart(ctx, user, []AddLine{{Slug: "cpu", Quantity: 8}, {Slug: "ram", Quantity: 1}}, false); err != nil {
			t.Fatal(err)
		}
		c := merge(guest(AddLine{Slug: "cpu", Quantity: 5}, AddLine{Slug: "ram", Quantity: 2}), user)
		if lineOf(t, c, "cpu").Quantity != MaxQuantity || lineOf(t, c, "ram").Quantity != 3 {
			t.Errorf("cart %+v", c)
		}
	})

	t.Run("past the line limit the oldest lines go", func(t *testing.T) {
		var adds []AddLine
		for i := range MaxCartLines {
			p := part(fmt.Sprintf("p%02d", i), 1_00, 5)
			cat.parts = append(cat.parts, p)
			adds = append(adds, AddLine{Slug: p.Slug, Quantity: 1})
		}
		user := UserCart(uuid.New())
		if _, _, err := s.AddToCart(ctx, user, adds, false); err != nil {
			t.Fatal(err)
		}
		c := merge(guest(AddLine{Slug: "cpu", Quantity: 1}), user)
		if len(c.Lines) != MaxCartLines || c.Lines[0].Slug != "cpu" {
			t.Errorf("%d lines, first %q", len(c.Lines), c.Lines[0].Slug)
		}
	})

	t.Run("no guest cart is nothing to do", func(t *testing.T) {
		user := UserCart(uuid.New())
		for _, token := range []string{"", "unknown"} {
			if c := merge(token, user); c.Count != 0 {
				t.Errorf("cart %+v", c)
			}
		}
	})
}

func slugsOf(c Cart) []string {
	var out []string
	for _, l := range c.Lines {
		out = append(out, l.Slug)
	}
	return out
}

func lineOf(t *testing.T, c Cart, slug string) CartLine {
	t.Helper()
	for _, l := range c.Lines {
		if l.Slug == slug {
			return l
		}
	}
	t.Fatalf("no %q line in %v", slug, slugsOf(c))
	return CartLine{}
}
