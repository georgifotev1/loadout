package orders

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/gfotev/loadout/internal/platform/db/dbtest"
	"github.com/gfotev/loadout/internal/platform/jobs"
)

func newOrderService(t *testing.T) *Service {
	t.Helper()
	pool := dbtest.New(t)
	jobsClient, err := jobs.New(pool, jobs.Config{Logger: slog.New(slog.DiscardHandler)})
	if err != nil {
		t.Fatal(err)
	}
	s := NewService(pool, &fakeCatalog{}, jobsClient)
	now := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return now }
	return s
}

// newOrder stores an order waiting for its payment. Checkout will make them; until then, tests do.
func newOrder(t *testing.T, s *Service, p Payment) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	err := s.pool.QueryRow(context.Background(),
		`INSERT INTO orders.orders (status, payment) VALUES ('awaiting_payment', $1) RETURNING id`, p,
	).Scan(&id)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func loadOrder(t *testing.T, s *Service, id uuid.UUID) Order {
	t.Helper()
	o, err := s.store.lockOrder(context.Background(), s.pool, id)
	if err != nil {
		t.Fatal(err)
	}
	return o
}

type transitionRow struct {
	Seq      int
	Event    Event
	From, To Status
}

func transitionRows(t *testing.T, s *Service, id uuid.UUID) []transitionRow {
	t.Helper()
	rows, err := s.pool.Query(context.Background(), `
		SELECT seq, event, from_status, to_status FROM orders.order_transitions
		WHERE order_id = $1 ORDER BY seq`, id)
	if err != nil {
		t.Fatal(err)
	}
	out, err := pgx.CollectRows(rows, pgx.RowToStructByPos[transitionRow])
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func queuedSteps(t *testing.T, s *Service, id uuid.UUID) []StepArgs {
	t.Helper()
	rows, err := s.pool.Query(context.Background(), `
		SELECT args FROM river.river_job WHERE kind = 'order_step' AND args->>'order_id' = $1 ORDER BY id`,
		id.String())
	if err != nil {
		t.Fatal(err)
	}
	out, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (StepArgs, error) {
		var raw []byte
		var a StepArgs
		if err := row.Scan(&raw); err != nil {
			return a, err
		}
		return a, json.Unmarshal(raw, &a)
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestApply(t *testing.T) {
	s := newOrderService(t)
	ctx := context.Background()
	id := newOrder(t, s, PaymentCOD)

	for _, ev := range []Event{EventCODSelected, EventSupplierConfirmed} {
		if err := s.Apply(ctx, id, ev); err != nil {
			t.Fatalf("%s: %v", ev, err)
		}
	}

	o := loadOrder(t, s, id)
	if o.Status != StatusReadyToShip || o.Seq != 2 || o.PaidAt != nil {
		t.Errorf("order %+v", o)
	}
	wantRows := []transitionRow{
		{1, EventCODSelected, StatusAwaitingPayment, StatusSupplierPending},
		{2, EventSupplierConfirmed, StatusSupplierPending, StatusReadyToShip},
	}
	if got := transitionRows(t, s, id); !slices.Equal(got, wantRows) {
		t.Errorf("transitions %+v, want %+v", got, wantRows)
	}
	wantSteps := []StepArgs{
		{id, CmdPlaceSupplierOrder, 1},
		{id, CmdEmailReceived, 1},
		{id, CmdCreateShipment, 2},
		{id, CmdEmailConfirmed, 2},
	}
	if got := queuedSteps(t, s, id); !slices.Equal(got, wantSteps) {
		t.Errorf("steps %+v, want %+v", got, wantSteps)
	}
}

func TestApplyPaymentIsDated(t *testing.T) {
	s := newOrderService(t)
	ctx := context.Background()
	id := newOrder(t, s, PaymentCard)

	if err := s.Apply(ctx, id, EventPaymentSucceeded); err != nil {
		t.Fatal(err)
	}
	if o := loadOrder(t, s, id); o.PaidAt == nil || !o.PaidAt.Equal(s.now()) {
		t.Errorf("paid at %v, want %v", o.PaidAt, s.now())
	}
}

// A webhook or a job delivered twice changes the order once and queues its steps once.
func TestApplyTwiceIsOneEffect(t *testing.T) {
	s := newOrderService(t)
	ctx := context.Background()
	id := newOrder(t, s, PaymentCard)

	for range 2 {
		if err := s.Apply(ctx, id, EventPaymentSucceeded); err != nil {
			t.Fatal(err)
		}
	}
	if o := loadOrder(t, s, id); o.Seq != 1 {
		t.Errorf("seq %d, want 1", o.Seq)
	}
	if n := len(transitionRows(t, s, id)); n != 1 {
		t.Errorf("%d transitions, want 1", n)
	}
	if n := len(queuedSteps(t, s, id)); n != 2 {
		t.Errorf("%d steps, want 2", n)
	}
}

func TestApplyRefusesInvalidEvents(t *testing.T) {
	s := newOrderService(t)
	ctx := context.Background()
	id := newOrder(t, s, PaymentCOD)

	if err := s.Apply(ctx, id, EventShipmentCreated); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("err %v, want ErrInvalidTransition", err)
	}
	if o := loadOrder(t, s, id); o.Status != StatusAwaitingPayment || o.Seq != 0 {
		t.Errorf("order %+v", o)
	}
	if n := len(transitionRows(t, s, id)); n != 0 {
		t.Errorf("%d transitions, want none", n)
	}
}

func TestApplyUnknownOrder(t *testing.T) {
	s := newOrderService(t)
	if err := s.Apply(context.Background(), uuid.New(), EventCancel); !errors.Is(err, ErrNotFound) {
		t.Errorf("err %v, want ErrNotFound", err)
	}
}

// failingInserter queues the first ok jobs, then fails, like a jobs table that breaks midway.
type failingInserter struct {
	jobs Inserter
	ok   int
}

var errInsert = errors.New("insert failed")

func (f *failingInserter) InsertTx(ctx context.Context, tx pgx.Tx, args river.JobArgs, opts *river.InsertOpts) (*rivertype.JobInsertResult, error) {
	if f.ok == 0 {
		return nil, errInsert
	}
	f.ok--
	return f.jobs.InsertTx(ctx, tx, args, opts)
}

// The new state, its transition row and its steps commit together: if the second step can't be
// queued, the order hasn't moved and the first step is gone too.
func TestApplyRollsBackWithoutItsSteps(t *testing.T) {
	s := newOrderService(t)
	ctx := context.Background()
	id := newOrder(t, s, PaymentCOD)
	s.jobs = &failingInserter{jobs: s.jobs, ok: 1}

	if err := s.Apply(ctx, id, EventCODSelected); !errors.Is(err, errInsert) {
		t.Fatalf("err %v, want the insert error", err)
	}
	if o := loadOrder(t, s, id); o.Status != StatusAwaitingPayment || o.Seq != 0 {
		t.Errorf("order %+v", o)
	}
	if n := len(transitionRows(t, s, id)); n != 0 {
		t.Errorf("%d transitions, want none", n)
	}
	if n := len(queuedSteps(t, s, id)); n != 0 {
		t.Errorf("%d steps, want none", n)
	}
}

// Events on one order arriving at once take turns on the row lock: each sees the state the one
// before it left, so the order moves once and the others are no-ops.
func TestApplyConcurrently(t *testing.T) {
	s := newOrderService(t)
	ctx := context.Background()
	id := newOrder(t, s, PaymentCard)

	const n = 8
	errs := make([]error, n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Go(func() { errs[i] = s.Apply(ctx, id, EventPaymentSucceeded) })
	}
	wg.Wait()
	if err := errors.Join(errs...); err != nil {
		t.Fatal(err)
	}

	if o := loadOrder(t, s, id); o.Status != StatusSupplierPending || o.Seq != 1 {
		t.Errorf("order %+v", o)
	}
	if n := len(transitionRows(t, s, id)); n != 1 {
		t.Errorf("%d transitions, want 1", n)
	}
	if n := len(queuedSteps(t, s, id)); n != 2 {
		t.Errorf("%d steps, want 2", n)
	}
}
