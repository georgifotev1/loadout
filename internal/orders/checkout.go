package orders

import (
	"context"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/gfotev/loadout/internal/catalog"
	"github.com/gfotev/loadout/internal/money"
	"github.com/gfotev/loadout/internal/platform/db"
	"github.com/gfotev/loadout/internal/platform/validate"
)

// DeliveryCents is what delivery to an address costs, VAT included.
// TODO(phase-7): Speedy's own quote replaces the flat fee, with office and APT delivery (step 7).
const DeliveryCents money.Cents = 5_99

// Quote is a cart as checkout shows it: the lines, delivery and what the order would cost.
type Quote struct {
	Cart          Cart
	DeliveryCents money.Cents
	TotalCents    money.Cents
	// Orderable is whether checkout would take the cart: it has lines, and every product is for
	// sale and in stock.
	Orderable bool
}

// CheckoutInput is what the customer fills in at checkout.
type CheckoutInput struct {
	Name     string
	Email    string
	Phone    string
	City     string
	Postcode string
	Address  string
	Payment  Payment
	// ExpectedTotalCents is the total the customer saw. If prices moved since, checkout refuses
	// and they look again, instead of being charged an amount they didn't agree to.
	ExpectedTotalCents money.Cents
	// UserID is the logged-in customer, uuid.Nil for a guest.
	UserID uuid.UUID
}

// Placed is a new order and the token for its page.
type Placed struct {
	ID     uuid.UUID
	Number int64
	Token  string
}

// Quote returns the owner's cart with delivery and the total.
func (s *Service) Quote(ctx context.Context, owner CartOwner) (Quote, error) {
	c, err := s.Cart(ctx, owner)
	if err != nil {
		return Quote{}, err
	}
	q := Quote{Cart: c, DeliveryCents: DeliveryCents, TotalCents: c.SubtotalCents + DeliveryCents}
	q.Orderable = len(c.Lines) > 0
	for _, l := range c.Lines {
		if l.Product == nil || l.Product.StockQty <= 0 {
			q.Orderable = false
		}
	}
	return q, nil
}

// Checkout turns the owner's cart into an order, in one transaction: it locks the cart, prices
// it again from the catalog, stores the order and its lines, deletes the cart and applies the
// payment's first event, which queues the order's first steps.
//
// It refuses (field errors) an empty cart ("cart: empty"), a product no longer for sale or out
// of stock ("cart: unavailable"), and a total other than the one the customer saw
// ("expected_total: changed"). Only cash on delivery for now.
func (s *Service) Checkout(ctx context.Context, owner CartOwner, in CheckoutInput) (Placed, error) {
	in.trim()
	if err := in.validate(); err != nil {
		return Placed{}, err
	}

	var placed Placed
	err := db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		var v validate.Validator
		cartID, found := uuid.Nil, false
		if !owner.none() {
			var err error
			if cartID, found, err = s.store.lockCart(ctx, tx, owner.key()); err != nil {
				return err
			}
		}
		var lines []CartLine
		if found {
			var err error
			if lines, err = s.store.cartLines(ctx, tx, owner.key()); err != nil {
				return err
			}
		}
		if len(lines) == 0 {
			v.Add("cart", "empty")
			return v.Err()
		}

		ids := make([]uuid.UUID, len(lines))
		for i, l := range lines {
			ids[i] = l.ProductID
		}
		parts, err := s.catalog.SaleParts(ctx, ids)
		if err != nil {
			return err
		}
		byID := make(map[uuid.UUID]catalog.SalePart, len(parts))
		for _, p := range parts {
			byID[p.ID] = p
		}

		o := draft{CheckoutInput: in, DeliveryCents: DeliveryCents}
		for _, l := range lines {
			p, ok := byID[l.ProductID]
			if !ok || p.StockQty <= 0 {
				v.Add("cart", "unavailable")
				continue
			}
			o.Lines = append(o.Lines, orderLine{SalePart: p, Quantity: l.Quantity})
			o.SubtotalCents += p.RetailCents * money.Cents(l.Quantity)
		}
		if err := v.Err(); err != nil {
			return err
		}
		v.Check(o.SubtotalCents+o.DeliveryCents == in.ExpectedTotalCents, "expected_total", "changed")
		if err := v.Err(); err != nil {
			return err
		}

		o.Token = randomToken()
		id, number, err := s.store.insertOrder(ctx, tx, o)
		if err != nil {
			return err
		}
		if err := s.store.deleteCart(ctx, tx, cartID); err != nil {
			return err
		}
		if err := s.ApplyTx(ctx, tx, id, EventCODSelected); err != nil {
			return err
		}
		placed = Placed{ID: id, Number: number, Token: o.Token}
		return nil
	})
	return placed, err
}

// draft is an order as checkout stores it.
type draft struct {
	CheckoutInput
	Token         string
	Lines         []orderLine
	SubtotalCents money.Cents
	DeliveryCents money.Cents
}

type orderLine struct {
	catalog.SalePart
	Quantity int
}

func (in *CheckoutInput) trim() {
	for _, f := range []*string{&in.Name, &in.Email, &in.Phone, &in.City, &in.Postcode, &in.Address} {
		*f = strings.TrimSpace(*f)
	}
}

func (in CheckoutInput) validate() error {
	var v validate.Validator
	text := func(field, s string, max int) {
		v.Check(s != "", field, "required")
		v.Check(utf8.RuneCountInString(s) <= max, field, "too_long")
	}
	text("name", in.Name, 100)
	v.Check(in.Email != "", "email", "required")
	v.Check(validate.Email(in.Email), "email", "invalid_email")
	v.Check(in.Phone != "", "phone", "required")
	v.Check(phone(in.Phone), "phone", "invalid_phone")
	text("city", in.City, 100)
	v.Check(in.Postcode != "", "postcode", "required")
	v.Check(postcode(in.Postcode), "postcode", "invalid_postcode")
	text("address", in.Address, 200)
	// TODO(phase-7): card payments come with Stripe (step 5).
	v.Check(in.Payment == PaymentCOD, "payment", "unsupported")
	return v.Err()
}

// phone accepts a phone number as people write it: 9 to 15 digits, with spaces, dashes, dots,
// brackets and a leading +. Speedy's API decides later whether it takes it as written.
func phone(s string) bool {
	digits := 0
	for i, r := range s {
		switch {
		case unicode.IsDigit(r) && r < utf8.RuneSelf:
			digits++
		case r == '+' && i == 0, r == ' ', r == '-', r == '.', r == '(', r == ')':
		default:
			return false
		}
	}
	return digits >= 9 && digits <= 15
}

// postcode is a Bulgarian postcode: four digits.
func postcode(s string) bool {
	if len(s) != 4 {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
