package orders

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/gfotev/loadout/internal/carrier"
	"github.com/gfotev/loadout/internal/money"
	"github.com/gfotev/loadout/internal/platform/db"
	"github.com/gfotev/loadout/internal/platform/jobs"
)

// Shipment is an order's waybill with the courier.
type Shipment struct {
	Carrier     string
	Number      string
	TrackingURL string // "" if the courier has no tracking page
	CreatedAt   time.Time
}

// Shipment returns an order's shipment, ErrNotFound if it has none yet.
func (s *Service) Shipment(ctx context.Context, orderID uuid.UUID) (Shipment, error) {
	sh, err := s.store.shipment(ctx, s.pool, orderID)
	switch {
	case err != nil:
		return Shipment{}, err
	case sh == nil:
		return Shipment{}, ErrNotFound
	}
	return *sh, nil
}

// createShipment makes the waybill of an order ready to ship, once. An order cancelled before
// the job runs gets none; one cancelled while the courier is making it keeps it, unused, on the
// admin's page.
func (w *StepWorker) createShipment(ctx context.Context, id uuid.UUID) error {
	o, err := w.svc.store.order(ctx, w.svc.pool, id)
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if o.Shipment != nil || o.Status != StatusReadyToShip {
		return nil
	}
	s, err := w.carrier.CreateShipment(ctx, shipmentRequest(o))
	if err != nil {
		return fmt.Errorf("orders: create shipment of order %s: %w", id, err)
	}
	return w.svc.store.insertShipment(ctx, w.svc.pool, id, w.carrier.Name(), s)
}

func shipmentRequest(o OrderDetail) carrier.Request {
	var cod money.Cents
	if o.Payment == PaymentCOD && o.PaidAt == nil {
		cod = o.TotalCents
	}
	contents := "Компютърни компоненти"
	if o.Assembly {
		contents = "Компютър"
	}
	return carrier.Request{
		Reference:     strconv.FormatInt(o.Number, 10),
		Name:          o.Name,
		Phone:         o.Phone,
		Email:         o.Email,
		OfficeID:      o.OfficeID,
		City:          o.City,
		Postcode:      o.Postcode,
		Address:       o.Address,
		CODCents:      cod,
		DeclaredCents: o.TotalCents - o.DeliveryCents,
		Contents:      contents,
	}
	// TODO(speedy-api): parcels and their weight. An assembled PC is one parcel; how the
	// supplier packs the rest, and whether the courier needs weights, comes with the contract.
}

// TrackArgs is the periodic job that asks the courier what became of the parcels on their way.
type TrackArgs struct{}

func (TrackArgs) Kind() string { return "poll_tracking" }

func (TrackArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{
		Queue: jobs.QueueOrders,
		// A failed poll isn't retried: the next one asks again.
		MaxAttempts: 1,
		// At most one poll waiting or running.
		UniqueOpts: river.UniqueOpts{ByState: []rivertype.JobState{
			rivertype.JobStateAvailable,
			rivertype.JobStatePending,
			rivertype.JobStateRunning,
			rivertype.JobStateScheduled,
		}},
	}
}

// PeriodicTracking polls the courier every interval, and once when the process starts.
func PeriodicTracking(interval time.Duration) *river.PeriodicJob {
	return river.NewPeriodicJob(
		river.PeriodicInterval(interval),
		func() (river.JobArgs, *river.InsertOpts) { return TrackArgs{}, nil },
		&river.PeriodicJobOpts{ID: "poll_tracking", RunOnStart: true},
	)
}

// TrackWorker carries out TrackArgs jobs: it applies picked_up and delivered to the orders whose
// parcels the courier took or handed over, and payment_succeeded to a cash on delivery order
// once the courier collected it.
// TODO(speedy-api): a parcel the customer refused or didn't collect goes back to the sender;
// the order needs a way to take that once we know how the courier reports it.
type TrackWorker struct {
	river.WorkerDefaults[TrackArgs]
	svc     *Service
	carrier carrier.Carrier
}

func NewTrackWorker(svc *Service, c carrier.Carrier) *TrackWorker {
	return &TrackWorker{svc: svc, carrier: c}
}

func (w *TrackWorker) Work(ctx context.Context, job *river.Job[TrackArgs]) error {
	list, err := w.svc.store.shipmentsOnTheWay(ctx, w.svc.pool, w.carrier.Name())
	if err != nil {
		return err
	}
	var errs []error
	for _, s := range list {
		t, err := w.carrier.Track(ctx, s.number)
		if err != nil {
			errs = append(errs, fmt.Errorf("orders: track %s of order %s: %w", s.number, s.orderID, err))
			continue
		}
		var evs []Event
		if t.PickedUp || t.Delivered {
			evs = append(evs, EventPickedUp)
		}
		if t.Delivered {
			evs = append(evs, EventDelivered)
			if s.payment == PaymentCOD {
				evs = append(evs, EventPaymentSucceeded)
			}
		}
		if len(evs) == 0 {
			continue
		}
		err = db.InTx(ctx, w.svc.pool, func(tx pgx.Tx) error {
			for _, ev := range evs {
				if err := w.svc.ApplyTx(ctx, tx, s.orderID, ev); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// onTheWay is a shipment the courier hasn't delivered yet.
type onTheWay struct {
	orderID uuid.UUID
	number  string
	payment Payment
}

// shipmentsOnTheWay returns the carrier's shipments of orders waiting for the courier or on
// their way.
func (store) shipmentsOnTheWay(ctx context.Context, q db.DBTX, carrierName string) ([]onTheWay, error) {
	query := `
		SELECT s.order_id, s.number, o.payment
		FROM orders.shipments s
		JOIN orders.orders o ON o.id = s.order_id
		WHERE s.carrier = $1 AND o.status IN ('ready_to_ship', 'shipped')
		ORDER BY s.created_at`
	rows, err := q.Query(ctx, query, carrierName)
	if err != nil {
		return nil, fmt.Errorf("orders: shipments on the way: %w", err)
	}
	out, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (onTheWay, error) {
		var s onTheWay
		err := row.Scan(
			&s.orderID,
			&s.number,
			&s.payment,
		)
		return s, err
	})
	if err != nil {
		return nil, fmt.Errorf("orders: shipments on the way: %w", err)
	}
	return out, nil
}

func (store) insertShipment(ctx context.Context, q db.DBTX, orderID uuid.UUID, carrierName string, s carrier.Shipment) error {
	insert := `
		INSERT INTO orders.shipments (order_id, carrier, number, tracking_url)
		VALUES ($1, $2, $3, $4)`
	if _, err := q.Exec(ctx, insert,
		orderID,
		carrierName,
		s.Number,
		s.TrackingURL,
	); err != nil {
		return fmt.Errorf("orders: insert shipment %s of order %s: %w", s.Number, orderID, err)
	}
	return nil
}

// shipment returns an order's shipment, or nil if it has none yet.
func (store) shipment(ctx context.Context, q db.DBTX, orderID uuid.UUID) (*Shipment, error) {
	query := `
		SELECT carrier, number, tracking_url, created_at
		FROM orders.shipments
		WHERE order_id = $1`
	var s Shipment
	err := q.QueryRow(ctx, query, orderID).Scan(
		&s.Carrier,
		&s.Number,
		&s.TrackingURL,
		&s.CreatedAt,
	)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return nil, nil
	case err != nil:
		return nil, fmt.Errorf("orders: shipment of order %s: %w", orderID, err)
	}
	return &s, nil
}
