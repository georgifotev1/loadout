package main

import (
	"net/http"

	"github.com/google/uuid"

	"github.com/gfotev/loadout/internal/auth"
	"github.com/gfotev/loadout/internal/money"
	"github.com/gfotev/loadout/internal/orders"
)

// GetCheckout handles GET /api/checkout.
func (app *application) GetCheckout(w http.ResponseWriter, r *http.Request) {
	q, err := app.orders.Quote(r.Context(), app.cartOwner(r))
	if err != nil {
		app.handleError(w, r, err)
		return
	}
	app.writeJSON(w, r, http.StatusOK, Checkout{
		Cart:      cart(q.Cart),
		Delivery:  eur(q.DeliveryCents),
		Total:     eur(q.TotalCents),
		Orderable: q.Orderable,
	})
}

// PlaceOrder handles POST /api/checkout.
func (app *application) PlaceOrder(w http.ResponseWriter, r *http.Request) {
	var in CheckoutRequest
	if err := readJSON(w, r, &in); err != nil {
		app.handleError(w, r, err)
		return
	}
	owner := app.cartOwner(r)
	placed, err := app.orders.Checkout(r.Context(), owner, orders.CheckoutInput{
		Name:               in.Name,
		Email:              in.Email,
		Phone:              in.Phone,
		City:               in.City,
		Postcode:           in.Postcode,
		Address:            in.Address,
		Payment:            orders.Payment(in.Payment),
		ExpectedTotalCents: money.Cents(in.ExpectedTotalCents),
		UserID:             owner.UserID,
	})
	if err != nil {
		app.handleError(w, r, err)
		return
	}
	if owner.Token != "" { // the guest cart is gone
		app.setCartCookie(w, "")
	}
	app.writeJSON(w, r, http.StatusCreated, OrderPlaced{Id: placed.ID, Number: placed.Number, Token: placed.Token})
}

// ListMyOrders handles GET /api/orders.
func (app *application) ListMyOrders(w http.ResponseWriter, r *http.Request) {
	sess, ok := sessionFrom(r.Context())
	if !ok {
		app.handleError(w, r, auth.ErrUnauthenticated)
		return
	}
	list, err := app.orders.CustomerOrders(r.Context(), sess.User.ID, verifiedEmail(sess.User))
	if err != nil {
		app.handleError(w, r, err)
		return
	}
	out := make([]OrderSummary, len(list))
	for i, o := range list {
		out[i] = OrderSummary{
			Id:        o.ID,
			Number:    o.Number,
			Status:    OrderStatus(o.Status),
			CreatedAt: o.CreatedAt,
			Count:     o.Count,
			Total:     eur(o.TotalCents),
		}
	}
	app.writeJSON(w, r, http.StatusOK, OrderList{Orders: out})
}

// GetOrder handles GET /api/orders/{order_id}.
func (app *application) GetOrder(w http.ResponseWriter, r *http.Request, id uuid.UUID, params GetOrderParams) {
	v := orders.Viewer{Token: params.Token}
	if sess, ok := sessionFrom(r.Context()); ok {
		v.UserID, v.VerifiedEmail = sess.User.ID, verifiedEmail(sess.User)
	}
	o, err := app.orders.Order(r.Context(), id, v)
	if err != nil {
		app.handleError(w, r, err)
		return
	}
	lines := make([]OrderLine, len(o.Lines))
	for i, l := range o.Lines {
		lines[i] = OrderLine{
			ProductId: l.ProductID,
			Slug:      l.Slug,
			Name:      l.Name,
			Image:     l.Image,
			Quantity:  l.Quantity,
			UnitPrice: eur(l.UnitCents),
			LineTotal: eur(l.TotalCents),
		}
	}
	app.writeJSON(w, r, http.StatusOK, Order{
		Id:        o.ID,
		Number:    o.Number,
		Status:    OrderStatus(o.Status),
		Payment:   PaymentMethod(o.Payment),
		CreatedAt: o.CreatedAt,
		Name:      o.Name,
		Email:     o.Email,
		Phone:     o.Phone,
		City:      o.City,
		Postcode:  o.Postcode,
		Address:   o.Address,
		Lines:     lines,
		Subtotal:  eur(o.SubtotalCents),
		Delivery:  eur(o.DeliveryCents),
		Total:     eur(o.TotalCents),
	})
}

// verifiedEmail is the user's email once they've verified it, else "": an unverified address
// could be anyone's, so it doesn't open the guest orders placed with it.
func verifiedEmail(u auth.User) string {
	if u.EmailVerified {
		return u.Email
	}
	return ""
}
