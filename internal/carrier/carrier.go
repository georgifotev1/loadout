// Package carrier is the port between orders and the courier that delivers them. Orders only
// ever sees a Request, a Shipment and a Tracking; each courier's adapter turns them into its own
// API. Mock stands in until there's a contract with a courier (Speedy).
//
// It holds only what orders needs now: a waybill for an order, what became of it, and its label.
// Offices, parcel lockers and price quotes come with checkout's delivery options.
package carrier

import (
	"context"
	"errors"

	"github.com/gfotev/loadout/internal/money"
)

// ErrUnknownShipment is returned for a waybill number the carrier doesn't know.
var ErrUnknownShipment = errors.New("carrier: unknown shipment")

// Carrier is one courier's API.
type Carrier interface {
	// Name identifies the courier ("speedy"). It's stored with every shipment, so it must not
	// change.
	Name() string
	// CreateShipment makes a waybill for a parcel that waits for the courier. It isn't
	// idempotent: a call repeated after a lost answer makes a second waybill.
	CreateShipment(ctx context.Context, r Request) (Shipment, error)
	// Track says what became of a shipment.
	Track(ctx context.Context, number string) (Tracking, error)
	// Label is the waybill to stick on the parcel, as a PDF.
	Label(ctx context.Context, number string) ([]byte, error)
}

// Request is an order's parcel as the courier needs it.
type Request struct {
	// Reference is our order number, printed on the waybill.
	Reference string
	Name      string
	Phone     string
	Email     string
	City      string
	Postcode  string
	Address   string
	// CODCents is what the courier collects on delivery, VAT included; 0 for a paid order.
	CODCents money.Cents
	// DeclaredCents is the goods' value, which the courier insures.
	DeclaredCents money.Cents
	// Contents is what's in the parcel, in a few words for the waybill.
	Contents string
}

// Shipment is a waybill the courier made.
type Shipment struct {
	Number string
	// TrackingURL is the courier's page that follows the shipment; "" if it has none.
	TrackingURL string
}

// Tracking is what became of a shipment.
type Tracking struct {
	PickedUp  bool // the courier has the parcel
	Delivered bool // and handed it over, collecting the COD amount if there was one
}
