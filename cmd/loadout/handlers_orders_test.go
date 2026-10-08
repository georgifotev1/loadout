package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"
	"github.com/stripe/stripe-go/v86"

	"github.com/gfotev/loadout/internal/orders"
	"github.com/gfotev/loadout/internal/payments"
	"github.com/gfotev/loadout/internal/platform/jobs"
)

func newOrdersApp(t *testing.T) *application {
	t.Helper()
	app := newCatalogApp(t)
	inserter, err := jobs.New(app.db, jobs.Config{Logger: slog.New(slog.DiscardHandler)})
	if err != nil {
		t.Fatal(err)
	}
	app.orders = orders.NewService(app.db, app.catalog, inserter)
	return app
}

func decodeJSON[T any](t *testing.T, rec *httptest.ResponseRecorder, status int) T {
	t.Helper()
	if rec.Code != status {
		t.Fatalf("status %d, want %d: %s", rec.Code, status, rec.Body)
	}
	var v T
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatal(err)
	}
	return v
}

func checkoutBody(total int64) string {
	return checkoutBodyPaying(total, "cod")
}

func checkoutBodyPaying(total int64, payment string) string {
	return `{"name": "Иван Иванов", "email": "ivan@example.bg", "phone": "0888 123 456",
		"city": "София", "postcode": "1000", "address": "ул. Граф Игнатиев 1", "payment": "` + payment + `",
		"expected_total_cents": ` + strconv.FormatInt(total, 10) + `}`
}

func TestCheckoutAPI(t *testing.T) {
	app := newOrdersApp(t)
	cpu := slugs(t, app)["AMD-AM5-R5-7600-BOX"]
	guest := cookieNamed(cartRequest(t, app, http.MethodPost, "/api/cart/lines", `{"lines":[{"slug":"`+cpu+`","quantity":2}]}`), "cart")

	co := decodeJSON[Checkout](t, get(t, app, "/api/checkout", guest), http.StatusOK)
	if !slices.Equal(co.Payments, []PaymentMethod{PaymentMethodCod}) {
		t.Errorf("payments %v, want cash on delivery only without Stripe", co.Payments)
	}
	if !co.Orderable || co.Cart.Count != 2 || co.Delivery.AmountCents != int64(orders.DeliveryCents) ||
		co.Total.AmountCents != co.Cart.Subtotal.AmountCents+co.Delivery.AmountCents {
		t.Fatalf("checkout %+v", co)
	}

	t.Run("card isn't on offer without Stripe", func(t *testing.T) {
		rec := cartRequest(t, app, http.MethodPost, "/api/checkout", checkoutBodyPaying(co.Total.AmountCents, "card"), guest)
		if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), `"unsupported"`) {
			t.Errorf("status %d: %s", rec.Code, rec.Body)
		}
	})

	t.Run("a stale total is refused", func(t *testing.T) {
		rec := cartRequest(t, app, http.MethodPost, "/api/checkout", checkoutBody(co.Total.AmountCents-1), guest)
		if rec.Code != http.StatusUnprocessableEntity {
			t.Errorf("status %d: %s", rec.Code, rec.Body)
		}
	})

	rec := cartRequest(t, app, http.MethodPost, "/api/checkout", checkoutBody(co.Total.AmountCents), guest)
	placed := decodeJSON[OrderPlaced](t, rec, http.StatusCreated)
	if c := cookieNamed(rec, "cart"); c == nil || c.MaxAge >= 0 {
		t.Errorf("cart cookie not cleared: %+v", c)
	}
	path := "/api/orders/" + placed.Id.String()

	o := decodeJSON[Order](t, get(t, app, path+"?token="+placed.Token), http.StatusOK)
	if o.Number != placed.Number || o.Status != OrderStatusSupplierPending || o.Payment != PaymentMethodCod ||
		o.Total != co.Total || len(o.Lines) != 1 || o.Lines[0].Slug != cpu || o.Lines[0].Quantity != 2 {
		t.Errorf("order %+v", o)
	}

	t.Run("without the token it's not found", func(t *testing.T) {
		for _, target := range []string{path, path + "?token=nope"} {
			if rec := get(t, app, target); rec.Code != http.StatusNotFound {
				t.Errorf("%s: status %d", target, rec.Code)
			}
		}
	})

	t.Run("the cart is empty now", func(t *testing.T) {
		rec := cartRequest(t, app, http.MethodPost, "/api/checkout", checkoutBody(co.Total.AmountCents), guest)
		if rec.Code != http.StatusUnprocessableEntity {
			t.Errorf("status %d: %s", rec.Code, rec.Body)
		}
	})
}

func TestCustomerOrdersAPI(t *testing.T) {
	app := newOrdersApp(t)
	if rec := get(t, app, "/api/orders"); rec.Code != http.StatusUnauthorized {
		t.Errorf("without a session: status %d", rec.Code)
	}

	if err := app.auth.Register(context.Background(), "ivan@example.bg", testPassword); err != nil {
		t.Fatal(err)
	}
	session := sessionCookie(t, postJSON(t, app, "/api/auth/login", `{"email": "ivan@example.bg", "password": "`+testPassword+`"}`))
	cpu := slugs(t, app)["AMD-AM5-R5-7600-BOX"]
	cartRequest(t, app, http.MethodPost, "/api/cart/lines", `{"lines":[{"slug":"`+cpu+`","quantity":1}]}`, session)
	co := decodeJSON[Checkout](t, get(t, app, "/api/checkout", session), http.StatusOK)
	placed := decodeJSON[OrderPlaced](t, cartRequest(t, app, http.MethodPost, "/api/checkout", checkoutBody(co.Total.AmountCents), session), http.StatusCreated)

	list := decodeJSON[OrderList](t, get(t, app, "/api/orders", session), http.StatusOK)
	if len(list.Orders) != 1 || list.Orders[0].Id != placed.Id || list.Orders[0].Count != 1 || list.Orders[0].Total != co.Total {
		t.Errorf("orders %+v", list)
	}
	if rec := get(t, app, "/api/orders/"+placed.Id.String(), session); rec.Code != http.StatusOK {
		t.Errorf("own order without the token: status %d", rec.Code)
	}
}

// withStripe sets up card payments against a fake Stripe that creates the payment pages
// (https://checkout.stripe.com/c/pay/cs_test_<order id>), after failing the first `down` times.
// The returned pay marks an order's page paid, as if the customer paid it.
func withStripe(t *testing.T, app *application, down int32) (pay func(orderID uuid.UUID)) {
	t.Helper()
	var calls atomic.Int32
	var paid sync.Map
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if id, ok := strings.CutPrefix(r.URL.Path, "/v1/checkout/sessions/"); ok && r.Method == http.MethodGet {
			status, paymentStatus := "open", "unpaid"
			if _, ok := paid.Load(id); ok {
				status, paymentStatus = "complete", "paid"
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id": "` + id + `", "object": "checkout.session", "status": "` + status + `", "payment_status": "` + paymentStatus + `"}`))
			return
		}
		if r.Method != http.MethodPost || r.URL.Path != "/v1/checkout/sessions" {
			http.NotFound(w, r)
			return
		}
		n := calls.Add(1)
		if n <= down {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error": {"type": "api_error", "message": "down"}}`))
			return
		}
		_ = r.ParseForm()
		w.Header().Set("Content-Type", "application/json")
		id := "cs_test_" + r.PostForm.Get("client_reference_id")
		_, _ = w.Write([]byte(`{"id": "` + id + `", "object": "checkout.session", "status": "open",
			"url": "https://checkout.stripe.com/c/pay/` + id + `", "expires_at": ` + r.PostForm.Get("expires_at") + `}`))
	}))
	t.Cleanup(srv.Close)
	app.payments = payments.NewService(app.db, payments.NewStripeClient("sk_test_x", srv.URL), app.orders, payments.Config{
		WebhookSecret: "whsec_test",
		SiteURL:       "https://loadout.bg",
		Logger:        app.logger,
	})
	return func(orderID uuid.UUID) { paid.Store("cs_test_"+orderID.String(), true) }
}

func TestCardCheckoutAPI(t *testing.T) {
	app := newOrdersApp(t)
	pay := withStripe(t, app, 1)
	cpu := slugs(t, app)["AMD-AM5-R5-7600-BOX"]
	order := func(t *testing.T) OrderPlaced {
		t.Helper()
		guest := cookieNamed(cartRequest(t, app, http.MethodPost, "/api/cart/lines", `{"lines":[{"slug":"`+cpu+`","quantity":1}]}`), "cart")
		co := decodeJSON[Checkout](t, get(t, app, "/api/checkout", guest), http.StatusOK)
		if !slices.Equal(co.Payments, []PaymentMethod{PaymentMethodCod, PaymentMethodCard}) {
			t.Errorf("payments %v", co.Payments)
		}
		return decodeJSON[OrderPlaced](t, cartRequest(t, app, http.MethodPost, "/api/checkout", checkoutBodyPaying(co.Total.AmountCents, "card"), guest), http.StatusCreated)
	}
	page := func(id uuid.UUID) string { return "https://checkout.stripe.com/c/pay/cs_test_" + id.String() }

	// Stripe is down for the first one: the order is placed anyway, without its page.
	placed := order(t)
	if placed.PaymentUrl != "" {
		t.Errorf("payment_url %q while Stripe is down", placed.PaymentUrl)
	}
	path := "/api/orders/" + placed.Id.String()
	o := decodeJSON[Order](t, get(t, app, path+"?token="+placed.Token), http.StatusOK)
	if o.Status != OrderStatusAwaitingPayment || o.Payment != PaymentMethodCard {
		t.Errorf("order %+v", o)
	}

	t.Run("the order page asks again", func(t *testing.T) {
		p := decodeJSON[Payment](t, cartRequest(t, app, http.MethodPost, path+"/payment?token="+placed.Token, ""), http.StatusOK)
		if p.Url != page(placed.Id) {
			t.Errorf("url %q", p.Url)
		}
		if rec := cartRequest(t, app, http.MethodPost, path+"/payment", ""); rec.Code != http.StatusNotFound {
			t.Errorf("without the token: status %d", rec.Code)
		}
	})

	t.Run("with Stripe up, checkout returns the page", func(t *testing.T) {
		placed := order(t)
		if placed.PaymentUrl != page(placed.Id) {
			t.Errorf("payment_url %q", placed.PaymentUrl)
		}

		t.Run("back from paying, before the webhook", func(t *testing.T) {
			syncURL := "/api/orders/" + placed.Id.String() + "/payment/sync?token=" + placed.Token
			o := decodeJSON[Order](t, cartRequest(t, app, http.MethodPost, syncURL, ""), http.StatusOK)
			if o.Status != OrderStatusAwaitingPayment {
				t.Errorf("status %s before paying", o.Status)
			}
			pay(placed.Id)
			o = decodeJSON[Order](t, cartRequest(t, app, http.MethodPost, syncURL, ""), http.StatusOK)
			if o.Status != OrderStatusSupplierPending || o.PaidAt == nil {
				t.Errorf("order %+v", o)
			}
			if rec := cartRequest(t, app, http.MethodPost, "/api/orders/"+placed.Id.String()+"/payment/sync", ""); rec.Code != http.StatusNotFound {
				t.Errorf("without the token: status %d", rec.Code)
			}
		})
	})

	t.Run("cash on delivery isn't paid by card", func(t *testing.T) {
		guest := cookieNamed(cartRequest(t, app, http.MethodPost, "/api/cart/lines", `{"lines":[{"slug":"`+cpu+`","quantity":1}]}`), "cart")
		co := decodeJSON[Checkout](t, get(t, app, "/api/checkout", guest), http.StatusOK)
		cod := decodeJSON[OrderPlaced](t, cartRequest(t, app, http.MethodPost, "/api/checkout", checkoutBody(co.Total.AmountCents), guest), http.StatusCreated)
		rec := cartRequest(t, app, http.MethodPost, "/api/orders/"+cod.Id.String()+"/payment?token="+cod.Token, "")
		if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), `"not_payable"`) {
			t.Errorf("status %d: %s", rec.Code, rec.Body)
		}
	})

	t.Run("stripe's webhook", func(t *testing.T) {
		payload, err := json.Marshal(map[string]any{
			"id": "evt_1", "object": "event", "api_version": stripe.APIVersion, "type": "checkout.session.completed",
			"data": map[string]any{"object": map[string]any{
				"id": "cs_test_1", "object": "checkout.session", "client_reference_id": placed.Id.String(), "payment_status": "paid",
			}},
		})
		if err != nil {
			t.Fatal(err)
		}
		send := func(sig string) *httptest.ResponseRecorder {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/api/webhooks/stripe", strings.NewReader(string(payload)))
			req.Header.Set("Stripe-Signature", sig)
			app.routes().ServeHTTP(rec, req)
			return rec
		}
		if rec := send("t=1,v1=00"); rec.Code != http.StatusBadRequest {
			t.Errorf("bad signature: status %d", rec.Code)
		}
		signed := stripe.GenerateTestSignedPayload(&stripe.UnsignedPayload{Payload: payload, Secret: "whsec_test"})
		if rec := send(signed.Header); rec.Code != http.StatusNoContent {
			t.Fatalf("status %d: %s", rec.Code, rec.Body)
		}
		o := decodeJSON[Order](t, get(t, app, path+"?token="+placed.Token), http.StatusOK)
		if o.Status != OrderStatusSupplierPending {
			t.Errorf("status %s", o.Status)
		}
		rec := cartRequest(t, app, http.MethodPost, path+"/payment?token="+placed.Token, "")
		if rec.Code != http.StatusConflict {
			t.Errorf("paying a paid order: status %d", rec.Code)
		}
	})
}
