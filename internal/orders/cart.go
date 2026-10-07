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

// CartOwner says whose cart: a logged-in user's (UserID), or else the one of the browser's cart
// cookie (Token). An owner with neither has no cart.
type CartOwner struct {
	UserID uuid.UUID
	Token  string
}

// UserCart is the cart of a logged-in user.
func UserCart(id uuid.UUID) CartOwner { return CartOwner{UserID: id} }

// GuestCart is the cart of a cart cookie's token.
func GuestCart(token string) CartOwner { return CartOwner{Token: token} }

func (o CartOwner) isUser() bool { return o.UserID != uuid.Nil }

func (o CartOwner) none() bool { return !o.isUser() && o.Token == "" }

// key is the owner as store arguments: the user id, or else the token's hash; the other is nil.
func (o CartOwner) key() cartKey {
	if o.isUser() {
		return cartKey{userID: &o.UserID}
	}
	return cartKey{tokenHash: hashToken(o.Token)}
}

// AddLine is a product to add to a cart.
type AddLine struct {
	Slug     string
	Quantity int
}

// Cart returns the owner's cart at today's prices. An owner without a cart has an empty one.
func (s *Service) Cart(ctx context.Context, owner CartOwner) (Cart, error) {
	if owner.none() {
		return Cart{Lines: []CartLine{}}, nil
	}
	lines, err := s.store.cartLines(ctx, s.pool, owner.key())
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

// AddToCart adds products to the owner's cart, on top of what's there, all or none. An owner
// without a cart gets a new one. It returns the cart and, for a guest, its token: owner.Token,
// or the new cart's.
//
// Only products for sale and in stock can be added, and a product's quantity can't go over
// MaxQuantity. Field errors name the slug or quantity of lines[i].
func (s *Service) AddToCart(ctx context.Context, owner CartOwner, lines []AddLine) (Cart, string, error) {
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
		cartID, found := uuid.Nil, false
		if !owner.none() {
			if cartID, found, err = s.store.lockCart(ctx, tx, owner.key()); err != nil {
				return err
			}
		}
		if !found {
			key := owner.key()
			if !owner.isUser() {
				owner.Token, key.tokenHash = newToken()
			}
			if cartID, err = s.store.insertCart(ctx, tx, key); err != nil {
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

	c, err := s.Cart(ctx, owner)
	return c, owner.Token, err
}

// SetCartQuantity changes how many of a product the owner's cart holds. A product that isn't in
// the cart is ErrNotFound.
func (s *Service) SetCartQuantity(ctx context.Context, owner CartOwner, productID uuid.UUID, quantity int) (Cart, error) {
	var v validate.Validator
	v.Check(quantity >= 1 && quantity <= MaxQuantity, "quantity", "out_of_range")
	if err := v.Err(); err != nil {
		return Cart{}, err
	}
	if owner.none() {
		return Cart{}, ErrNotFound
	}
	ok, err := s.store.setQuantity(ctx, s.pool, owner.key(), productID, quantity)
	if err != nil {
		return Cart{}, err
	}
	if !ok {
		return Cart{}, ErrNotFound
	}
	return s.Cart(ctx, owner)
}

// RemoveFromCart takes a product out of the owner's cart. Removing one that isn't there is not
// an error: the cart is as asked either way.
func (s *Service) RemoveFromCart(ctx context.Context, owner CartOwner, productID uuid.UUID) (Cart, error) {
	if !owner.none() {
		if err := s.store.removeLine(ctx, s.pool, owner.key(), productID); err != nil {
			return Cart{}, err
		}
	}
	return s.Cart(ctx, owner)
}

// MergeCart moves the guest cart of token into the user's cart when they log in, so what they
// added before logging in isn't lost. Quantities of a product in both add up to MaxQuantity, and
// past MaxCartLines the oldest lines go. The guest cart is gone afterwards; with no cart of its
// own, the user simply takes it over.
func (s *Service) MergeCart(ctx context.Context, token string, userID uuid.UUID) error {
	if token == "" {
		return nil
	}
	return db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		guestID, userCartID, err := s.store.lockCartsForMerge(ctx, tx, hashToken(token), userID)
		if err != nil || guestID == uuid.Nil {
			return err
		}
		if userCartID == uuid.Nil {
			return s.store.giveCart(ctx, tx, guestID, userID)
		}
		return s.store.mergeCart(ctx, tx, guestID, userCartID, MaxQuantity, MaxCartLines)
	})
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
