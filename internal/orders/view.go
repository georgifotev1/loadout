package orders

import (
	"context"
	"crypto/subtle"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/gfotev/loadout/internal/money"
)

// OrderDetail is an order as its customer sees it. It leaves out what we pay the supplier.
type OrderDetail struct {
	ID        uuid.UUID
	Number    int64
	Token     string
	Status    Status
	Payment   Payment
	PaidAt    *time.Time
	CreatedAt time.Time
	UserID    *uuid.UUID
	Email     string
	Name      string
	Phone     string
	Delivery  Delivery
	// OfficeID and OfficeName are the courier's office or locker; "" for an address.
	OfficeID   string
	OfficeName string
	// City, Postcode and Address are where the parcel goes: the office's for an office.
	City          string
	Postcode      string
	Address       string
	Lines         []OrderLine
	SubtotalCents money.Cents
	DeliveryCents money.Cents
	// Assembly is whether the supplier assembles the PC of the lines InBuild, for AssemblyCents.
	Assembly      bool
	AssemblyCents money.Cents
	// BuildCents is the assembled PC's price: the lines InBuild and the fee. 0 without assembly.
	BuildCents money.Cents
	TotalCents money.Cents
	Shipment   *Shipment // nil until the waybill is made
}

// OrderLine is one product of an order, as it was sold.
type OrderLine struct {
	ProductID  uuid.UUID
	Slug       string
	Name       string
	Image      string // "" if the product had none
	Quantity   int
	UnitCents  money.Cents
	TotalCents money.Cents // UnitCents × Quantity
	InBuild    bool        // goes into the assembled PC
}

// OrderSummary is an order in a customer's order history.
type OrderSummary struct {
	ID         uuid.UUID
	Number     int64
	Status     Status
	CreatedAt  time.Time
	Count      int // units over all lines
	TotalCents money.Cents
}

// Viewer is who asks for an order: the token of the order's link, or a logged-in customer, or
// both.
type Viewer struct {
	Token  string
	UserID uuid.UUID
	// VerifiedEmail is the customer's email once they've verified it, "" before: it also opens
	// the guest orders placed with it.
	VerifiedEmail string
}

func (v Viewer) mayView(o OrderDetail) bool {
	switch {
	case v.Token != "" && subtle.ConstantTimeCompare([]byte(v.Token), []byte(o.Token)) == 1:
		return true
	case v.UserID != uuid.Nil && o.UserID != nil && *o.UserID == v.UserID:
		return true
	case v.VerifiedEmail != "" && strings.EqualFold(v.VerifiedEmail, o.Email):
		return true
	}
	return false
}

// Order returns an order for its customer. An order the viewer may not see is ErrNotFound, the
// same as one that doesn't exist.
func (s *Service) Order(ctx context.Context, id uuid.UUID, v Viewer) (OrderDetail, error) {
	o, err := s.store.order(ctx, s.pool, id)
	if err != nil {
		return OrderDetail{}, err
	}
	if !v.mayView(o) {
		return OrderDetail{}, ErrNotFound
	}
	return o, nil
}

// CustomerOrders returns a customer's order history, newest first: the orders placed while
// logged in and, with a verified email, the guest orders placed with it.
func (s *Service) CustomerOrders(ctx context.Context, userID uuid.UUID, verifiedEmail string) ([]OrderSummary, error) {
	return s.store.customerOrders(ctx, s.pool, userID, verifiedEmail)
}
