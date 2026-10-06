package main

import (
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/gfotev/loadout/internal/orders"
	"github.com/gfotev/loadout/internal/platform/config"
)

// cartCookieTTL is how long a cart cookie lives after the cart last changed.
const cartCookieTTL = 30 * 24 * time.Hour

// GetCart handles GET /api/cart.
func (app *application) GetCart(w http.ResponseWriter, r *http.Request) {
	c, err := app.orders.Cart(r.Context(), app.cartToken(r))
	if err != nil {
		app.handleError(w, r, err)
		return
	}
	app.writeJSON(w, r, http.StatusOK, cart(c))
}

// AddToCart handles POST /api/cart/lines.
func (app *application) AddToCart(w http.ResponseWriter, r *http.Request) {
	var in CartAdd
	if err := readJSON(w, r, &in); err != nil {
		app.handleError(w, r, err)
		return
	}
	lines := make([]orders.AddLine, len(in.Lines))
	for i, l := range in.Lines {
		lines[i] = orders.AddLine{Slug: l.Slug, Quantity: l.Quantity}
	}
	c, token, err := app.orders.AddToCart(r.Context(), app.cartToken(r), lines)
	if err != nil {
		app.handleError(w, r, err)
		return
	}
	app.setCartCookie(w, token)
	app.writeJSON(w, r, http.StatusOK, cart(c))
}

// SetCartQuantity handles PUT /api/cart/lines/{product_id}.
func (app *application) SetCartQuantity(w http.ResponseWriter, r *http.Request, productID uuid.UUID) {
	var in CartQuantity
	if err := readJSON(w, r, &in); err != nil {
		app.handleError(w, r, err)
		return
	}
	token := app.cartToken(r)
	c, err := app.orders.SetCartQuantity(r.Context(), token, productID, in.Quantity)
	if err != nil {
		app.handleError(w, r, err)
		return
	}
	app.setCartCookie(w, token)
	app.writeJSON(w, r, http.StatusOK, cart(c))
}

// RemoveFromCart handles DELETE /api/cart/lines/{product_id}.
func (app *application) RemoveFromCart(w http.ResponseWriter, r *http.Request, productID uuid.UUID) {
	c, err := app.orders.RemoveFromCart(r.Context(), app.cartToken(r), productID)
	if err != nil {
		app.handleError(w, r, err)
		return
	}
	app.writeJSON(w, r, http.StatusOK, cart(c))
}

func cart(c orders.Cart) Cart {
	lines := make([]CartLine, len(c.Lines))
	for i, l := range c.Lines {
		lines[i] = CartLine{
			ProductId: l.ProductID,
			Slug:      l.Slug,
			Name:      l.Name,
			Quantity:  l.Quantity,
		}
		if l.Product != nil {
			p, total := summary(*l.Product), eur(l.LineCents)
			lines[i].Product, lines[i].LineTotal = &p, &total
		}
	}
	return Cart{Lines: lines, Count: c.Count, Subtotal: eur(c.SubtotalCents)}
}

// cartToken is the cart cookie's value, or "" without one.
func (app *application) cartToken(r *http.Request) string {
	if c, err := r.Cookie(app.cartCookieName()); err == nil {
		return c.Value
	}
	return ""
}

// cartCookieName has the __Host- prefix in production, like the session cookie.
func (app *application) cartCookieName() string {
	if app.cfg.Env == config.EnvProduction {
		return "__Host-cart"
	}
	return "cart"
}

// setCartCookie (re)sets the cart cookie for another cartCookieTTL. HttpOnly and SameSite=Lax
// for the same reasons as the session cookie: the web app never reads it, the API does.
func (app *application) setCartCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     app.cartCookieName(),
		Value:    token,
		Path:     "/",
		MaxAge:   int(cartCookieTTL.Seconds()),
		HttpOnly: true,
		Secure:   app.cfg.Env == config.EnvProduction,
		SameSite: http.SameSiteLaxMode,
	})
}
