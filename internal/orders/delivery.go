package orders

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/gfotev/loadout/internal/carrier"
	"github.com/gfotev/loadout/internal/money"
	"github.com/gfotev/loadout/internal/platform/db"
	"github.com/gfotev/loadout/internal/platform/jobs"
)

// Delivery is where the courier takes the parcel.
type Delivery string

const (
	DeliveryAddress Delivery = "address"
	DeliveryOffice  Delivery = "office" // a courier's office, where the customer collects it
	DeliveryAPT     Delivery = "apt"    // a parcel locker
)

// Deliveries are the delivery methods in the order checkout offers them.
var Deliveries = []Delivery{DeliveryOffice, DeliveryAPT, DeliveryAddress}

// What delivery costs the customer, VAT included: our prices, not the courier's quote, so they
// don't depend on weights we don't know yet.
const (
	AddressDeliveryCents money.Cents = 5_99
	OfficeDeliveryCents  money.Cents = 3_99
	APTDeliveryCents     money.Cents = 3_49
)

// Cents is what the delivery costs.
func (d Delivery) Cents() money.Cents {
	switch d {
	case DeliveryOffice:
		return OfficeDeliveryCents
	case DeliveryAPT:
		return APTDeliveryCents
	default:
		return AddressDeliveryCents
	}
}

// fits says whether the parcel fits the delivery. An assembled PC doesn't fit a parcel locker.
// TODO(speedy-api): the locker's sizes and weights, against the parcels', once the contract says
// what the courier needs to know about them.
func (d Delivery) fits(assembled bool) bool {
	return d != DeliveryAPT || !assembled
}

// officeKind is the kind of office a delivery goes to.
func (d Delivery) officeKind() carrier.OfficeKind {
	if d == DeliveryAPT {
		return carrier.KindAPT
	}
	return carrier.KindOffice
}

// Office is a courier's office or parcel locker the customer can collect from.
type Office = carrier.Office

// maxOffices is how many offices a search returns.
const maxOffices = 20

// SearchOffices returns the offices of a kind whose name, city, postcode or address contain
// every word of text, ignoring case: up to 20, by city and name. Text shorter than 2 letters
// matches nothing.
func (s *Service) SearchOffices(ctx context.Context, text string, kind carrier.OfficeKind) ([]Office, error) {
	words := strings.Fields(strings.ToLower(text))
	if len([]rune(strings.Join(words, ""))) < 2 {
		return nil, nil
	}
	return s.store.searchOffices(ctx, s.pool, words, kind)
}

// OfficeSyncArgs is the daily job that copies the courier's offices and lockers.
type OfficeSyncArgs struct{}

func (OfficeSyncArgs) Kind() string { return "sync_offices" }

func (OfficeSyncArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{
		Queue: jobs.QueueSync,
		// A failed sync isn't retried: checkout keeps yesterday's offices, and the next run tries.
		MaxAttempts: 1,
		UniqueOpts: river.UniqueOpts{ByState: []rivertype.JobState{
			rivertype.JobStateAvailable,
			rivertype.JobStatePending,
			rivertype.JobStateRunning,
			rivertype.JobStateScheduled,
		}},
	}
}

// PeriodicOfficeSync syncs the offices once a day, and once when the process starts.
func PeriodicOfficeSync() *river.PeriodicJob {
	return river.NewPeriodicJob(
		river.PeriodicInterval(24*time.Hour),
		func() (river.JobArgs, *river.InsertOpts) { return OfficeSyncArgs{}, nil },
		&river.PeriodicJobOpts{ID: "sync_offices", RunOnStart: true},
	)
}

// OfficeSyncWorker carries out OfficeSyncArgs jobs.
type OfficeSyncWorker struct {
	river.WorkerDefaults[OfficeSyncArgs]
	svc     *Service
	carrier carrier.Carrier
}

func NewOfficeSyncWorker(svc *Service, c carrier.Carrier) *OfficeSyncWorker {
	return &OfficeSyncWorker{svc: svc, carrier: c}
}

func (w *OfficeSyncWorker) Work(ctx context.Context, job *river.Job[OfficeSyncArgs]) error {
	list, err := w.carrier.Offices(ctx)
	if err != nil {
		return fmt.Errorf("orders: offices from the courier: %w", err)
	}
	return w.svc.replaceOffices(ctx, list)
}

// replaceOffices makes the courier's list the offices checkout offers, in one transaction. An
// empty list is refused: a courier with no offices at all is a broken answer, and it would leave
// checkout with none.
func (s *Service) replaceOffices(ctx context.Context, list []Office) error {
	if len(list) == 0 {
		return errors.New("orders: the courier listed no offices")
	}
	now := s.now()
	return db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		return s.store.replaceOffices(ctx, tx, list, now)
	})
}

func (store) replaceOffices(ctx context.Context, tx pgx.Tx, list []Office, now time.Time) error {
	upsert := `
		INSERT INTO orders.offices (id, kind, name, city, postcode, address, hours, search, synced_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (id) DO UPDATE SET
			kind = excluded.kind,
			name = excluded.name,
			city = excluded.city,
			postcode = excluded.postcode,
			address = excluded.address,
			hours = excluded.hours,
			search = excluded.search,
			synced_at = excluded.synced_at`
	var b pgx.Batch
	ids := make([]string, len(list))
	for i, o := range list {
		ids[i] = o.ID
		search := strings.ToLower(strings.Join([]string{o.Name, o.City, o.Postcode, o.Address}, " "))
		b.Queue(upsert,
			o.ID,
			o.Kind,
			o.Name,
			o.City,
			o.Postcode,
			o.Address,
			o.Hours,
			search,
			now,
		)
	}
	if err := tx.SendBatch(ctx, &b).Close(); err != nil {
		return fmt.Errorf("orders: save offices: %w", err)
	}
	// Whatever this sync didn't see is gone from the courier.
	if _, err := tx.Exec(ctx, `DELETE FROM orders.offices WHERE id <> ALL($1)`, ids); err != nil {
		return fmt.Errorf("orders: delete closed offices: %w", err)
	}
	return nil
}

// searchOffices: every word is in search; strpos rather than LIKE, so % and _ aren't wildcards.
func (store) searchOffices(ctx context.Context, q db.DBTX, words []string, kind carrier.OfficeKind) ([]Office, error) {
	query := `
		SELECT id, kind, name, city, postcode, address, hours
		FROM orders.offices
		WHERE kind = $1
		  AND NOT EXISTS (SELECT 1 FROM unnest($2::text[]) w WHERE strpos(search, w) = 0)
		ORDER BY city, name
		LIMIT $3`
	rows, err := q.Query(ctx, query, kind, words, maxOffices)
	if err != nil {
		return nil, fmt.Errorf("orders: search offices: %w", err)
	}
	out, err := pgx.CollectRows(rows, scanOffice)
	if err != nil {
		return nil, fmt.Errorf("orders: search offices: %w", err)
	}
	return out, nil
}

// office returns an office of a kind, or nil if the courier has no such office.
func (store) office(ctx context.Context, q db.DBTX, id string, kind carrier.OfficeKind) (*Office, error) {
	query := `
		SELECT id, kind, name, city, postcode, address, hours
		FROM orders.offices
		WHERE id = $1 AND kind = $2`
	rows, err := q.Query(ctx, query, id, kind)
	if err != nil {
		return nil, fmt.Errorf("orders: office %s: %w", id, err)
	}
	o, err := pgx.CollectOneRow(rows, scanOffice)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return nil, nil
	case err != nil:
		return nil, fmt.Errorf("orders: office %s: %w", id, err)
	}
	return &o, nil
}

func scanOffice(row pgx.CollectableRow) (Office, error) {
	var o Office
	err := row.Scan(
		&o.ID,
		&o.Kind,
		&o.Name,
		&o.City,
		&o.Postcode,
		&o.Address,
		&o.Hours,
	)
	return o, err
}
