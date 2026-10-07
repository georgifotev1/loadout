package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gfotev/loadout/internal/orders"
)

func cartRequest(t *testing.T, app *application, method, target, body string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	for _, c := range cookies {
		if c != nil {
			req.AddCookie(c)
		}
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
	app.orders = orders.NewService(app.db, app.catalog, nil) // the cart queues no jobs
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

func cookieNamed(rec *httptest.ResponseRecorder, name string) *http.Cookie {
	for _, c := range rec.Result().Cookies() {
		if c.Name == name {
			return c
		}
	}
	return nil
}

// A guest cart moves into the account on login; a logged-in user's cart is the account's, with
// no cart cookie, and stays with it after logout.
func TestCartFollowsTheAccount(t *testing.T) {
	app := newCatalogApp(t)
	app.orders = orders.NewService(app.db, app.catalog, nil) // the cart queues no jobs
	app.auth = newAuthService(t, app.db)
	if err := app.auth.Register(context.Background(), "ivan@example.bg", testPassword); err != nil {
		t.Fatal(err)
	}
	cpu := slugs(t, app)["AMD-AM5-R5-7600-BOX"]
	add := `{"lines":[{"slug":"` + cpu + `","quantity":1}]}`

	guest := cookieNamed(cartRequest(t, app, http.MethodPost, "/api/cart/lines", add), "cart")
	login := cartRequest(t, app, http.MethodPost, "/api/auth/login", `{"email": "ivan@example.bg", "password": "`+testPassword+`"}`, guest)
	if login.Code != http.StatusOK {
		t.Fatalf("login: status %d: %s", login.Code, login.Body)
	}
	if c := cookieNamed(login, "cart"); c == nil || c.MaxAge >= 0 {
		t.Errorf("cart cookie not cleared on login: %+v", c)
	}
	session := sessionCookie(t, login)

	rec := cartRequest(t, app, http.MethodPost, "/api/cart/lines", add, session)
	if c := decodeCart(t, rec); c.Count != 2 {
		t.Errorf("cart %+v, want the guest's line and this one", c)
	}
	if c := cookieNamed(rec, "cart"); c != nil {
		t.Errorf("a logged-in add set a cart cookie: %+v", c)
	}
	if c := decodeCart(t, get(t, app, "/api/cart", guest)); c.Count != 0 {
		t.Errorf("the guest cart is still there: %+v", c)
	}

	logout := post(t, app, "/api/auth/logout", session)
	if c := cookieNamed(logout, "cart"); c == nil || c.MaxAge >= 0 {
		t.Errorf("cart cookie not cleared on logout: %+v", c)
	}
	login = postJSON(t, app, "/api/auth/login", `{"email": "ivan@example.bg", "password": "`+testPassword+`"}`)
	if c := decodeCart(t, get(t, app, "/api/cart", sessionCookie(t, login))); c.Count != 2 {
		t.Errorf("cart after logging in again %+v", c)
	}
}
