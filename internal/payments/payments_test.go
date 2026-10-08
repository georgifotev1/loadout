package payments

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stripe/stripe-go/v86"

	"github.com/gfotev/loadout/internal/orders"
	"github.com/gfotev/loadout/internal/platform/db/dbtest"
	"github.com/gfotev/loadout/internal/platform/jobs"
)

const webhookSecret = "whsec_test"

// fakeStripe is the part of Stripe's API that payments calls. It keeps one checkout session,
// created like Stripe does: the same idempotency key gets the same session back.
type fakeStripe struct {
	mu       sync.Mutex
	calls    []string     // "METHOD /path"
	forms    []url.Values // the bodies of the POSTs
	keys     []string     // their Idempotency-Key headers
	session  map[string]any
	refunded stripe.RefundStatus
}

func (f *fakeStripe) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, r.Method+" "+r.URL.Path)
	if r.Method == http.MethodPost {
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		f.forms = append(f.forms, r.PostForm)
		f.keys = append(f.keys, r.Header.Get("Idempotency-Key"))
	}
	var out any
	switch {
	case r.Method == http.MethodPost && r.URL.Path == "/v1/checkout/sessions":
		if f.session == nil {
			expires := r.PostForm.Get("expires_at")
			f.session = map[string]any{
				"id":             "cs_test_1",
				"object":         "checkout.session",
				"url":            "https://checkout.stripe.com/c/pay/cs_test_1",
				"status":         "open",
				"payment_status": "unpaid",
				"expires_at":     json.Number(expires),
			}
		}
		out = f.session
	case r.Method == http.MethodGet && r.URL.Path == "/v1/checkout/sessions/cs_test_1":
		out = f.session
	case r.Method == http.MethodPost && r.URL.Path == "/v1/checkout/sessions/cs_test_1/expire":
		f.session["status"] = "expired"
		out = f.session
	case r.Method == http.MethodPost && r.URL.Path == "/v1/refunds":
		out = map[string]any{"id": "re_1", "object": "refund", "status": f.refunded}
	default:
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

// set changes the session as Stripe would when the customer pays (or doesn't).
func (f *fakeStripe) set(status, paymentStatus string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.session["status"] = status
	f.session["payment_status"] = paymentStatus
	f.session["payment_intent"] = "pi_1"
}

type fixture struct {
	svc    *Service
	orders *orders.Service
	stripe *fakeStripe
	now    time.Time
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	pool := dbtest.New(t)
	inserter, err := jobs.New(pool, jobs.Config{Logger: slog.New(slog.DiscardHandler)})
	if err != nil {
		t.Fatal(err)
	}
	f := &fixture{orders: orders.NewService(pool, nil, inserter), stripe: &fakeStripe{refunded: stripe.RefundStatusSucceeded}}
	srv := httptest.NewServer(f.stripe)
	t.Cleanup(srv.Close)
	f.svc = NewService(pool, NewStripeClient("sk_test_x", srv.URL), f.orders, Config{
		WebhookSecret: webhookSecret,
		SiteURL:       "https://loadout.bg",
		Logger:        slog.New(slog.DiscardHandler),
	})
	if err := pool.QueryRow(context.Background(), `SELECT now()`).Scan(&f.now); err != nil {
		t.Fatal(err)
	}
	f.svc.now = func() time.Time { return f.now }
	return f
}

// newOrder stores an order with two lines, placed at createdAt, and returns it as its customer
// sees it.
func (f *fixture) newOrder(t *testing.T, p orders.Payment, createdAt time.Time) orders.OrderDetail {
	t.Helper()
	ctx := context.Background()
	var id uuid.UUID
	err := f.svc.pool.QueryRow(ctx, `
		INSERT INTO orders.orders (
			status, payment, token, email, name, phone, city, postcode, address,
			subtotal_cents, delivery_cents, total_cents, created_at
		)
		VALUES ('awaiting_payment', $1, $2, 'ivan@example.bg', 'Иван', '0888123456', 'София', '1000',
		        'ул. 1', 48000, 599, 48599, $3)
		RETURNING id`, p, "tok-"+uuid.NewString(), createdAt,
	).Scan(&id)
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.svc.pool.Exec(ctx, `
		INSERT INTO orders.order_lines (
			order_id, position, product_id, provider, provider_sku, slug, name, image, quantity,
			dealer_cents, retail_cents, vat_bp
		)
		VALUES ($1, 0, gen_random_uuid(), 'vali', 'CPU', 'cpu', 'Процесор', '', 2, 16000, 20000, 2000),
		       ($1, 1, gen_random_uuid(), 'vali', 'RAM', 'ram', 'Памет', '', 1, 6000, 8000, 2000)`, id)
	if err != nil {
		t.Fatal(err)
	}
	return f.order(t, id)
}

func (f *fixture) order(t *testing.T, id uuid.UUID) orders.OrderDetail {
	t.Helper()
	var token string
	if err := f.svc.pool.QueryRow(context.Background(), `SELECT token FROM orders.orders WHERE id = $1`, id).Scan(&token); err != nil {
		t.Fatal(err)
	}
	o, err := f.orders.Order(context.Background(), id, orders.Viewer{Token: token})
	if err != nil {
		t.Fatal(err)
	}
	return o
}

func TestPay(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	o := f.newOrder(t, orders.PaymentCard, f.now.Add(-5*time.Minute))

	got, err := f.svc.Pay(ctx, o)
	if err != nil {
		t.Fatal(err)
	}
	if got != "https://checkout.stripe.com/c/pay/cs_test_1" {
		t.Errorf("url %q", got)
	}
	form, key := f.stripe.forms[0], f.stripe.keys[0]
	page := "https://loadout.bg/order/" + o.ID.String() + "?token=" + o.Token
	for k, want := range map[string]string{
		"mode":                   "payment",
		"client_reference_id":    o.ID.String(),
		"customer_email":         "ivan@example.bg",
		"locale":                 "bg",
		"expires_at":             fmt.Sprint(o.CreatedAt.Add(orders.PaymentWindow).Unix()),
		"success_url":            page + "&paid=1",
		"cancel_url":             page,
		"integration_identifier": integrationID,
		"metadata[order_id]":     o.ID.String(),
		"payment_intent_data[metadata][order_id]":       o.ID.String(),
		"line_items[0][price_data][currency]":           "eur",
		"line_items[0][price_data][unit_amount]":        "20000",
		"line_items[0][quantity]":                       "2",
		"line_items[1][price_data][product_data][name]": "Памет",
		"line_items[2][price_data][product_data][name]": "Доставка",
		"line_items[2][price_data][unit_amount]":        "599",
	} {
		if form.Get(k) != want {
			t.Errorf("%s = %q, want %q", k, form.Get(k), want)
		}
	}
	for k := range form {
		if strings.HasPrefix(k, "payment_method_types") {
			t.Errorf("sent %s: payment methods are Stripe's (dynamic)", k)
		}
	}
	if key != "checkout-"+o.ID.String() {
		t.Errorf("idempotency key %q", key)
	}

	t.Run("the same page again while it's open", func(t *testing.T) {
		again, err := f.svc.Pay(ctx, o)
		want := []string{"POST /v1/checkout/sessions", "GET /v1/checkout/sessions/cs_test_1"}
		if err != nil || again != got || !slices.Equal(f.stripe.calls, want) {
			t.Errorf("url %q, err %v, calls %v", again, err, f.stripe.calls)
		}
	})

	t.Run("not after the page expired", func(t *testing.T) {
		later := *f
		later.svc.now = func() time.Time { return o.CreatedAt.Add(orders.PaymentWindow) }
		t.Cleanup(func() { f.svc.now = func() time.Time { return f.now } })
		if _, err := later.svc.Pay(ctx, o); !errors.Is(err, ErrNotPayable) {
			t.Errorf("err %v", err)
		}
	})

	for _, tt := range []struct {
		name  string
		order func(t *testing.T) orders.OrderDetail
	}{
		{"cash on delivery", func(t *testing.T) orders.OrderDetail {
			return f.newOrder(t, orders.PaymentCOD, f.now)
		}},
		{"too late for a new page", func(t *testing.T) orders.OrderDetail {
			// Stripe wants a page to live 30 minutes at least.
			return f.newOrder(t, orders.PaymentCard, f.now.Add(-(orders.PaymentWindow - minSessionLife)))
		}},
		{"paid already", func(t *testing.T) orders.OrderDetail {
			o := f.newOrder(t, orders.PaymentCard, f.now)
			if err := f.orders.Apply(ctx, o.ID, orders.EventPaymentSucceeded); err != nil {
				t.Fatal(err)
			}
			return f.order(t, o.ID)
		}},
	} {
		t.Run("not "+tt.name, func(t *testing.T) {
			calls := len(f.stripe.calls)
			if _, err := f.svc.Pay(ctx, tt.order(t)); !errors.Is(err, ErrNotPayable) {
				t.Errorf("err %v", err)
			}
			if len(f.stripe.calls) != calls {
				t.Errorf("called Stripe: %v", f.stripe.calls[calls:])
			}
		})
	}
}

// event signs a Stripe event the way Stripe does.
func event(t *testing.T, id string, typ stripe.EventType, object map[string]any) ([]byte, string) {
	t.Helper()
	payload, err := json.Marshal(map[string]any{
		"id":          id,
		"object":      "event",
		"api_version": stripe.APIVersion,
		"type":        typ,
		"data":        map[string]any{"object": object},
	})
	if err != nil {
		t.Fatal(err)
	}
	signed := stripe.GenerateTestSignedPayload(&stripe.UnsignedPayload{Payload: payload, Secret: webhookSecret})
	return payload, signed.Header
}

func session(o orders.OrderDetail, paymentStatus string) map[string]any {
	return map[string]any{
		"id":                  "cs_test_1",
		"object":              "checkout.session",
		"client_reference_id": o.ID.String(),
		"payment_status":      paymentStatus,
	}
}

func (f *fixture) transitions(t *testing.T, id uuid.UUID) int {
	t.Helper()
	var n int
	if err := f.svc.pool.QueryRow(context.Background(), `SELECT count(*) FROM orders.order_transitions WHERE order_id = $1`, id).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestWebhook(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	handle := func(payload []byte, sig string) error { return f.svc.HandleWebhook(ctx, payload, sig) }

	t.Run("paid", func(t *testing.T) {
		o := f.newOrder(t, orders.PaymentCard, f.now)
		payload, sig := event(t, "evt_paid", stripe.EventTypeCheckoutSessionCompleted, session(o, "paid"))
		if err := handle(payload, sig); err != nil {
			t.Fatal(err)
		}
		if got := f.order(t, o.ID); got.Status != orders.StatusSupplierPending || got.PaidAt == nil {
			t.Errorf("order %+v", got)
		}

		t.Run("sent twice is one effect", func(t *testing.T) {
			if err := handle(payload, sig); err != nil {
				t.Fatal(err)
			}
			if n := f.transitions(t, o.ID); n != 1 {
				t.Errorf("%d transitions", n)
			}
		})
	})

	t.Run("a bank debit pays later", func(t *testing.T) {
		o := f.newOrder(t, orders.PaymentCard, f.now)
		if err := handle(event(t, "evt_debit_completed", stripe.EventTypeCheckoutSessionCompleted, session(o, "unpaid"))); err != nil {
			t.Fatal(err)
		}
		if got := f.order(t, o.ID); got.Status != orders.StatusAwaitingPayment {
			t.Errorf("status %s before the money came", got.Status)
		}
		if err := handle(event(t, "evt_debit_paid", stripe.EventTypeCheckoutSessionAsyncPaymentSucceeded, session(o, "paid"))); err != nil {
			t.Fatal(err)
		}
		if got := f.order(t, o.ID); got.Status != orders.StatusSupplierPending {
			t.Errorf("status %s", got.Status)
		}
	})

	for _, tt := range []struct {
		typ stripe.EventType
		pay string
	}{
		{stripe.EventTypeCheckoutSessionExpired, "unpaid"},
		{stripe.EventTypeCheckoutSessionAsyncPaymentFailed, "unpaid"},
	} {
		t.Run(string(tt.typ), func(t *testing.T) {
			o := f.newOrder(t, orders.PaymentCard, f.now)
			if err := handle(event(t, "evt_"+string(tt.typ), tt.typ, session(o, tt.pay))); err != nil {
				t.Fatal(err)
			}
			if got := f.order(t, o.ID); got.Status != orders.StatusCancelled || got.PaidAt != nil {
				t.Errorf("order %+v", got)
			}
		})
	}

	t.Run("a refund", func(t *testing.T) {
		o := f.newOrder(t, orders.PaymentCard, f.now)
		for _, ev := range []orders.Event{orders.EventPaymentSucceeded, orders.EventCancel} {
			if err := f.orders.Apply(ctx, o.ID, ev); err != nil {
				t.Fatal(err)
			}
		}
		refund := func(status string) map[string]any {
			return map[string]any{"id": "re_1", "object": "refund", "status": status, "metadata": map[string]string{"order_id": o.ID.String()}}
		}
		if err := handle(event(t, "evt_refund_pending", stripe.EventTypeRefundCreated, refund("pending"))); err != nil {
			t.Fatal(err)
		}
		if got := f.order(t, o.ID); got.Status != orders.StatusRefunding {
			t.Errorf("status %s while pending", got.Status)
		}
		if err := handle(event(t, "evt_refund_done", stripe.EventTypeRefundUpdated, refund("succeeded"))); err != nil {
			t.Fatal(err)
		}
		if got := f.order(t, o.ID); got.Status != orders.StatusCancelled {
			t.Errorf("status %s", got.Status)
		}
	})

	t.Run("an event for no order of ours", func(t *testing.T) {
		gone := orders.OrderDetail{ID: uuid.New()}
		if err := handle(event(t, "evt_elsewhere", stripe.EventTypeCheckoutSessionCompleted, session(gone, "paid"))); err != nil {
			t.Error(err)
		}
		other := map[string]any{"id": "cs_link", "object": "checkout.session", "payment_status": "paid"}
		if err := handle(event(t, "evt_payment_link", stripe.EventTypeCheckoutSessionCompleted, other)); err != nil {
			t.Error(err)
		}
	})

	t.Run("an event that doesn't fit the order is logged, not retried", func(t *testing.T) {
		o := f.newOrder(t, orders.PaymentCOD, f.now)
		if err := handle(event(t, "evt_cod_expired", stripe.EventTypeCheckoutSessionExpired, session(o, "unpaid"))); err != nil {
			t.Error(err)
		}
	})

	t.Run("a bad signature", func(t *testing.T) {
		o := f.newOrder(t, orders.PaymentCard, f.now)
		payload, _ := event(t, "evt_forged", stripe.EventTypeCheckoutSessionCompleted, session(o, "paid"))
		_, sig := event(t, "evt_other", stripe.EventTypeCheckoutSessionCompleted, session(o, "paid"))
		if err := handle(payload, sig); !errors.Is(err, ErrBadSignature) {
			t.Errorf("err %v", err)
		}
		if got := f.order(t, o.ID); got.Status != orders.StatusAwaitingPayment {
			t.Errorf("status %s", got.Status)
		}
	})
}

type failingOrders struct{ err error }

func (f failingOrders) ApplyTx(context.Context, pgx.Tx, uuid.UUID, orders.Event) error { return f.err }

// An event that fails to apply isn't recorded, so Stripe's next delivery tries again.
func TestWebhookRollsBack(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	o := f.newOrder(t, orders.PaymentCard, f.now)
	errDB := errors.New("db is down")
	f.svc.orders = failingOrders{errDB}

	payload, sig := event(t, "evt_retry", stripe.EventTypeCheckoutSessionCompleted, session(o, "paid"))
	if err := f.svc.HandleWebhook(ctx, payload, sig); !errors.Is(err, errDB) {
		t.Fatalf("err %v", err)
	}
	f.svc.orders = f.orders
	if err := f.svc.HandleWebhook(ctx, payload, sig); err != nil {
		t.Fatal(err)
	}
	if got := f.order(t, o.ID); got.Status != orders.StatusSupplierPending {
		t.Errorf("status %s", got.Status)
	}
}

func TestOutcome(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	t.Run("no payment page", func(t *testing.T) {
		o := f.newOrder(t, orders.PaymentCard, f.now)
		if ev, err := f.svc.Outcome(ctx, o.ID); err != nil || ev != orders.EventPaymentTimedOut {
			t.Errorf("%q, %v", ev, err)
		}
	})

	o := f.newOrder(t, orders.PaymentCard, f.now)
	if _, err := f.svc.Pay(ctx, o); err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		status, paymentStatus string
		want                  orders.Event
	}{
		{"complete", "paid", orders.EventPaymentSucceeded},
		{"complete", "unpaid", ""},
		{"expired", "unpaid", orders.EventPaymentTimedOut},
		{"open", "unpaid", orders.EventPaymentTimedOut}, // expired on the way
	} {
		t.Run(tt.status+" "+tt.paymentStatus, func(t *testing.T) {
			f.stripe.set(tt.status, tt.paymentStatus)
			if ev, err := f.svc.Outcome(ctx, o.ID); err != nil || ev != tt.want {
				t.Errorf("%q, %v; want %q", ev, err, tt.want)
			}
		})
	}
	if last := f.stripe.calls[len(f.stripe.calls)-1]; last != "POST /v1/checkout/sessions/cs_test_1/expire" {
		t.Errorf("an open page wasn't expired: %v", f.stripe.calls)
	}
}

func TestRefund(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	o := f.newOrder(t, orders.PaymentCard, f.now)
	if _, err := f.svc.Pay(ctx, o); err != nil {
		t.Fatal(err)
	}
	f.stripe.set("complete", "paid")

	done, err := f.svc.Refund(ctx, o.ID)
	if err != nil || !done {
		t.Fatalf("done %v, err %v", done, err)
	}
	form, key := f.stripe.forms[len(f.stripe.forms)-1], f.stripe.keys[len(f.stripe.keys)-1]
	if form.Get("payment_intent") != "pi_1" || form.Get("metadata[order_id]") != o.ID.String() || form.Has("amount") {
		t.Errorf("form %v", form)
	}
	if key != "refund-"+o.ID.String() {
		t.Errorf("idempotency key %q", key)
	}

	t.Run("pending", func(t *testing.T) {
		f.stripe.refunded = stripe.RefundStatusPending
		if done, err := f.svc.Refund(ctx, o.ID); err != nil || done {
			t.Errorf("done %v, err %v", done, err)
		}
	})

	t.Run("no payment page", func(t *testing.T) {
		if _, err := f.svc.Refund(ctx, f.newOrder(t, orders.PaymentCard, f.now).ID); err == nil {
			t.Error("no error")
		}
	})
}

// The webhook is late: the order page's next call finds out from Stripe.
func TestPaidBeforeTheWebhook(t *testing.T) {
	ctx := context.Background()

	t.Run("sync", func(t *testing.T) {
		f := newFixture(t)
		o := f.newOrder(t, orders.PaymentCard, f.now)
		if _, err := f.svc.Pay(ctx, o); err != nil {
			t.Fatal(err)
		}
		if err := f.svc.Sync(ctx, o); err != nil {
			t.Fatal(err)
		}
		if got := f.order(t, o.ID); got.Status != orders.StatusAwaitingPayment {
			t.Errorf("status %s before paying", got.Status)
		}

		f.stripe.set("complete", "paid")
		if err := f.svc.Sync(ctx, o); err != nil {
			t.Fatal(err)
		}
		if got := f.order(t, o.ID); got.Status != orders.StatusSupplierPending || got.PaidAt == nil {
			t.Errorf("order %+v", got)
		}

		t.Run("and the webhook after it changes nothing", func(t *testing.T) {
			payload, sig := event(t, "evt_late", stripe.EventTypeCheckoutSessionCompleted, session(o, "paid"))
			if err := f.svc.HandleWebhook(ctx, payload, sig); err != nil {
				t.Fatal(err)
			}
			if n := f.transitions(t, o.ID); n != 1 {
				t.Errorf("%d transitions", n)
			}
		})
	})

	t.Run("pay", func(t *testing.T) {
		f := newFixture(t)
		o := f.newOrder(t, orders.PaymentCard, f.now)
		if _, err := f.svc.Pay(ctx, o); err != nil {
			t.Fatal(err)
		}
		f.stripe.set("complete", "paid")
		if _, err := f.svc.Pay(ctx, o); !errors.Is(err, ErrNotPayable) {
			t.Errorf("err %v", err)
		}
		if got := f.order(t, o.ID); got.Status != orders.StatusSupplierPending {
			t.Errorf("status %s", got.Status)
		}
	})

	t.Run("nothing to ask", func(t *testing.T) {
		f := newFixture(t)
		for _, o := range []orders.OrderDetail{
			f.newOrder(t, orders.PaymentCOD, f.now),
			f.newOrder(t, orders.PaymentCard, f.now), // no payment page yet
		} {
			if err := f.svc.Sync(ctx, o); err != nil {
				t.Error(err)
			}
		}
		if len(f.stripe.calls) != 0 {
			t.Errorf("called Stripe: %v", f.stripe.calls)
		}
	})
}
