package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gfotev/loadout/internal/orders"
)

func cartRequest(t *testing.T, app *application, method, target, body string, cookie *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if cookie != nil {
		req.AddCookie(cookie)
	}
	app.routes().ServeHTTP(rec, req)
	return rec
}

func decodeCart(t *testing.T, rec *httptest.ResponseRecorder) Cart {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	var c Cart
	if err := json.Unmarshal(rec.Body.Bytes(), &c); err != nil {
		t.Fatal(err)
	}
	return c
}

func TestCartAPI(t *testing.T) {
	app := newCatalogApp(t)
	app.orders = orders.NewService(app.db, app.catalog)
	slug := slugs(t, app)
	cpu := slug["AMD-AM5-R5-7600-BOX"]

	t.Run("no cookie is an empty cart and sets none", func(t *testing.T) {
		rec := get(t, app, "/api/cart")
		c := decodeCart(t, rec)
		if len(c.Lines) != 0 || c.Count != 0 || c.Subtotal.AmountCents != 0 {
			t.Errorf("cart %+v", c)
		}
		if len(rec.Result().Cookies()) != 0 {
			t.Errorf("cookies %v", rec.Result().Cookies())
		}
	})

	t.Run("a failed add sets no cookie", func(t *testing.T) {
		rec := cartRequest(t, app, http.MethodPost, "/api/cart/lines", `{"lines":[{"slug":"nope","quantity":1}]}`, nil)
		if rec.Code != http.StatusUnprocessableEntity || len(rec.Result().Cookies()) != 0 {
			t.Errorf("status %d, cookies %v: %s", rec.Code, rec.Result().Cookies(), rec.Body)
		}
	})

	rec := cartRequest(t, app, http.MethodPost, "/api/cart/lines", `{"lines":[{"slug":"`+cpu+`","quantity":2}]}`, nil)
	c := decodeCart(t, rec)
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != "cart" || !cookies[0].HttpOnly || cookies[0].MaxAge != int(cartCookieTTL.Seconds()) {
		t.Fatalf("cookies %+v", cookies)
	}
	cookie := cookies[0]
	if len(c.Lines) != 1 || c.Count != 2 {
		t.Fatalf("cart %+v", c)
	}
	line := c.Lines[0]
	if line.Product == nil || line.LineTotal == nil || line.Slug != cpu ||
		line.LineTotal.AmountCents != 2*line.Product.Price.AmountCents || c.Subtotal != *line.LineTotal {
		t.Errorf("line %+v", line)
	}

	t.Run("the cookie finds the cart", func(t *testing.T) {
		c := decodeCart(t, get(t, app, "/api/cart", cookie))
		if c.Count != 2 {
			t.Errorf("cart %+v", c)
		}
	})

	t.Run("set quantity", func(t *testing.T) {
		target := "/api/cart/lines/" + line.ProductId.String()
		c := decodeCart(t, cartRequest(t, app, http.MethodPut, target, `{"quantity":3}`, cookie))
		if c.Count != 3 {
			t.Errorf("cart %+v", c)
		}
		if rec := cartRequest(t, app, http.MethodPut, target, `{"quantity":3}`, nil); rec.Code != http.StatusNotFound {
			t.Errorf("without the cookie: status %d", rec.Code)
		}
		if rec := cartRequest(t, app, http.MethodPut, target, `{"quantity":11}`, cookie); rec.Code != http.StatusUnprocessableEntity {
			t.Errorf("11: status %d", rec.Code)
		}
	})

	t.Run("remove", func(t *testing.T) {
		rec := cartRequest(t, app, http.MethodDelete, "/api/cart/lines/"+line.ProductId.String(), "", cookie)
		if c := decodeCart(t, rec); len(c.Lines) != 0 || c.Count != 0 {
			t.Errorf("cart %+v", c)
		}
	})
}
