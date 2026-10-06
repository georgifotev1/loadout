package orders

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/gfotev/loadout/internal/catalog"
	"github.com/gfotev/loadout/internal/money"
	"github.com/gfotev/loadout/internal/platform/db"
	"github.com/gfotev/loadout/internal/platform/validate"
)

// Limits on a cart: more than a shop like ours sells to one person at once. MaxQuantity is also
// a CHECK on orders.cart_lines.
const (
	MaxQuantity  = 10 // of one product
	MaxCartLines = 50
)

// CartLine is one product in a cart.
type CartLine struct {
	ProductID uuid.UUID
	Slug      string // the product's slug now, or as it was added if it's no longer for sale
	Name      string // likewise
	Quantity  int
	// Product is the product as it's sold now, nil when it's no longer for sale (hidden,
	// unpriced or gone). Such a line stays until the shopper removes it, so they see what
	// happened, but it isn't counted in the subtotal and can't be ordered.
	Product   *catalog.ProductSummary
	LineCents money.Cents // the price now × quantity; 0 when not for sale
}

// Cart is a shopper's cart at today's prices.
type Cart struct {
	Lines         []CartLine // the last added first
	Count         int        // units over all lines
	SubtotalCents money.Cents
}

// AddLine is a product to add to a cart.
type AddLine struct {
	Slug     string
	Quantity int
}

// Cart returns the cart of token at today's prices. Without a token, or with one that has no
// cart, it's an empty cart.
func (s *Service) Cart(ctx context.Context, token string) (Cart, error) {
	if token == "" {
		return Cart{Lines: []CartLine{}}, nil
	}
	lines, err := s.store.cartLines(ctx, s.pool, hashToken(token))
	if err != nil {
		return Cart{}, err
	}
	ids := make([]uuid.UUID, 0, len(lines))
	for _, l := range lines {
		ids = append(ids, l.ProductID)
	}
	parts, err := s.catalog.PartsByIDs(ctx, ids)
	if err != nil {
		return Cart{}, err
	}
	forSale := make(map[uuid.UUID]catalog.ProductSummary, len(parts))
	for _, p := range parts {
		forSale[p.ID] = p.ProductSummary
	}

	c := Cart{Lines: lines}
	for i := range c.Lines {
		l := &c.Lines[i]
		c.Count += l.Quantity
		if p, ok := forSale[l.ProductID]; ok {
			l.Product = &p
			l.Slug, l.Name = p.Slug, p.Name
			l.LineCents = p.RetailCents * money.Cents(l.Quantity)
			c.SubtotalCents += l.LineCents
		}
	}
	return c, nil
}

// AddToCart adds products to the cart of token, on top of what's there, all or none. Without a
// token, or with one that has no cart, it starts a new cart. It returns the cart and its token:
// token itself, or the new cart's.
//
// Only products for sale and in stock can be added, and a product's quantity can't go over
// MaxQuantity. Field errors name the slug or quantity of lines[i].
func (s *Service) AddToCart(ctx context.Context, token string, lines []AddLine) (Cart, string, error) {
	var v validate.Validator
	v.Check(len(lines) > 0, "lines", "required")
	v.Check(len(lines) <= MaxCartLines, "lines", "too_many")
	add := map[string]int{} // quantity by slug; a slug can be listed twice
	var slugs []string
	for i, l := range lines {
		v.Check(l.Slug != "", fmt.Sprintf("lines[%d].slug", i), "required")
		v.Check(l.Quantity >= 1 && l.Quantity <= MaxQuantity, fmt.Sprintf("lines[%d].quantity", i), "out_of_range")
		if _, ok := add[l.Slug]; !ok {
			slugs = append(slugs, l.Slug)
		}
		add[l.Slug] += l.Quantity
	}
	if err := v.Err(); err != nil {
		return Cart{}, "", err
	}

	parts, err := s.catalog.PartsBySlugs(ctx, slugs)
	if err != nil {
		return Cart{}, "", err
	}
	bySlug := make(map[string]catalog.Part, len(parts))
	for _, p := range parts {
		bySlug[p.Slug] = p
	}
	for i, l := range lines {
		p, ok := bySlug[l.Slug]
		switch {
		case !ok:
			v.Add(fmt.Sprintf("lines[%d].slug", i), "unavailable")
		case p.StockQty <= 0:
			v.Add(fmt.Sprintf("lines[%d].slug", i), "out_of_stock")
		}
	}
	if err := v.Err(); err != nil {
		return Cart{}, "", err
	}

	err = db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		cartID, found, err := s.store.lockCart(ctx, tx, hashToken(token))
		if err != nil {
			return err
		}
		if !found {
			var hash []byte
			token, hash = newToken()
			if cartID, err = s.store.insertCart(ctx, tx, hash); err != nil {
				return err
			}
		}
		have, err := s.store.quantities(ctx, tx, cartID)
		if err != nil {
			return err
		}

		for i, l := range lines {
			v.Check(have[bySlug[l.Slug].ID]+add[l.Slug] <= MaxQuantity, fmt.Sprintf("lines[%d].quantity", i), "too_many")
		}
		lineCount := len(have)
		for _, slug := range slugs {
			if _, ok := have[bySlug[slug].ID]; !ok {
				lineCount++
			}
		}
		v.Check(lineCount <= MaxCartLines, "lines", "too_many")
		if err := v.Err(); err != nil {
			return err
		}

		for _, slug := range slugs {
			p := bySlug[slug]
			if err := s.store.addLine(ctx, tx, cartID, p.ID, p.Slug, p.Name, add[slug]); err != nil {
				return err
			}
		}
		return s.store.touchCart(ctx, tx, cartID)
	})
	if err != nil {
		return Cart{}, "", err
	}

	c, err := s.Cart(ctx, token)
	return c, token, err
}

// SetCartQuantity changes how many of a product the cart of token holds. A product that isn't
// in the cart is ErrNotFound.
func (s *Service) SetCartQuantity(ctx context.Context, token string, productID uuid.UUID, quantity int) (Cart, error) {
	var v validate.Validator
	v.Check(quantity >= 1 && quantity <= MaxQuantity, "quantity", "out_of_range")
	if err := v.Err(); err != nil {
		return Cart{}, err
	}
	if token == "" {
		return Cart{}, ErrNotFound
	}
	ok, err := s.store.setQuantity(ctx, s.pool, hashToken(token), productID, quantity)
	if err != nil {
		return Cart{}, err
	}
	if !ok {
		return Cart{}, ErrNotFound
	}
	return s.Cart(ctx, token)
}

// RemoveFromCart takes a product out of the cart of token. Removing one that isn't there is
// not an error: the cart is as asked either way.
func (s *Service) RemoveFromCart(ctx context.Context, token string, productID uuid.UUID) (Cart, error) {
	if token != "" {
		if err := s.store.removeLine(ctx, s.pool, hashToken(token), productID); err != nil {
			return Cart{}, err
		}
	}
	return s.Cart(ctx, token)
}

// newToken returns a random cart token for the cookie and its hash for the database.
func newToken() (string, []byte) {
	b := make([]byte, 32)
	_, _ = rand.Read(b) // crypto/rand.Read never returns an error
	token := base64.RawURLEncoding.EncodeToString(b)
	return token, hashToken(token)
}

// hashToken is a plain SHA-256, as for auth sessions: the token is 256 random bits, so it can't
// be guessed and needs no slow hash.
func hashToken(token string) []byte {
	h := sha256.Sum256([]byte(token))
	return h[:]
}
