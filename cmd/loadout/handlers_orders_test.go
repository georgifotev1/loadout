package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/gfotev/loadout/internal/orders"
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
	return `{"name": "Иван Иванов", "email": "ivan@example.bg", "phone": "0888 123 456",
		"city": "София", "postcode": "1000", "address": "ул. Граф Игнатиев 1", "payment": "cod",
		"expected_total_cents": ` + strconv.FormatInt(total, 10) + `}`
}

func TestCheckoutAPI(t *testing.T) {
	app := newOrdersApp(t)
	cpu := slugs(t, app)["AMD-AM5-R5-7600-BOX"]
	guest := cookieNamed(cartRequest(t, app, http.MethodPost, "/api/cart/lines", `{"lines":[{"slug":"`+cpu+`","quantity":2}]}`), "cart")

	co := decodeJSON[Checkout](t, get(t, app, "/api/checkout", guest), http.StatusOK)
	if !co.Orderable || co.Cart.Count != 2 || co.Delivery.AmountCents != int64(orders.DeliveryCents) ||
		co.Total.AmountCents != co.Cart.Subtotal.AmountCents+co.Delivery.AmountCents {
		t.Fatalf("checkout %+v", co)
	}

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
