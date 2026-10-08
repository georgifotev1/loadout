package main

import (
	"errors"
	"io"
	"net/http"

	"github.com/google/uuid"

	"github.com/gfotev/loadout/internal/auth"
	"github.com/gfotev/loadout/internal/money"
	"github.com/gfotev/loadout/internal/orders"
	"github.com/gfotev/loadout/internal/payments"
	"github.com/gfotev/loadout/internal/platform/validate"
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
		Payments:  app.paymentMethods(),
	})
}

// paymentMethods are the payment methods on offer: card only when Stripe is set up.
func (app *application) paymentMethods() []PaymentMethod {
	if app.payments == nil {
		return []PaymentMethod{PaymentMethodCod}
	}
	return []PaymentMethod{PaymentMethodCod, PaymentMethodCard}
}

// PlaceOrder handles POST /api/checkout.
func (app *application) PlaceOrder(w http.ResponseWriter, r *http.Request) {
	var in CheckoutRequest
	if err := readJSON(w, r, &in); err != nil {
		app.handleError(w, r, err)
		return
	}
	if in.Payment == PaymentMethodCard && app.payments == nil {
		var v validate.Validator
		v.Add("payment", "unsupported")
		app.handleError(w, r, v.Err())
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
	out := OrderPlaced{Id: placed.ID, Number: placed.Number, Token: placed.Token}
	if in.Payment == PaymentMethodCard {
		// The order is placed whatever Stripe says: without a payment page, the order page asks
		// for one again.
		if url, err := app.payPage(r, placed.ID, orders.Viewer{Token: placed.Token}); err != nil {
			app.logger.Error("no payment page for a new order", "order_id", placed.ID, "err", err, "request_id", requestIDFrom(r.Context()))
		} else {
			out.PaymentUrl = url
		}
	}
	app.writeJSON(w, r, http.StatusCreated, out)
}

// PayOrder handles POST /api/orders/{order_id}/payment.
func (app *application) PayOrder(w http.ResponseWriter, r *http.Request, id uuid.UUID, params PayOrderParams) {
	url, err := app.payPage(r, id, app.orderViewer(r, params.Token))
	if err != nil {
		app.handleError(w, r, err)
		return
	}
	app.writeJSON(w, r, http.StatusOK, Payment{Url: url})
}

// payPage returns the address of Stripe's payment page for an order the viewer may see.
func (app *application) payPage(r *http.Request, id uuid.UUID, v orders.Viewer) (string, error) {
	o, err := app.orders.Order(r.Context(), id, v)
	if err != nil {
		return "", err
	}
	if app.payments == nil {
		return "", payments.ErrNotPayable
	}
	return app.payments.Pay(r.Context(), o)
}

// maxWebhookBytes bounds a Stripe event. Ours are a few kilobytes.
const maxWebhookBytes = 256 << 10

// stripeWebhook handles POST /api/webhooks/stripe. It isn't in api/openapi.yaml: Stripe calls
// it, not our clients. A 2xx tells Stripe the event is handled; anything else and it sends the
// event again, for up to three days.
func (app *application) stripeWebhook(w http.ResponseWriter, r *http.Request) {
	if app.payments == nil {
		app.notFound(w, r)
		return
	}
	payload, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxWebhookBytes))
	if err != nil {
		app.handleError(w, r, bodyErrorf("body too large or unreadable"))
		return
	}
	err = app.payments.HandleWebhook(r.Context(), payload, r.Header.Get("Stripe-Signature"))
	switch {
	case errors.Is(err, payments.ErrBadSignature):
		app.logger.Warn("stripe webhook refused", "err", err, "request_id", requestIDFrom(r.Context()))
		app.writeError(w, r, http.StatusBadRequest, ErrorCodeInvalidBody, "invalid signature")
	case err != nil:
		app.handleError(w, r, err)
	default:
		w.WriteHeader(http.StatusNoContent)
	}
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
	o, err := app.orders.Order(r.Context(), id, app.orderViewer(r, params.Token))
	if err != nil {
		app.handleError(w, r, err)
		return
	}
	app.writeJSON(w, r, http.StatusOK, orderJSON(o))
}

// SyncOrderPayment handles POST /api/orders/{order_id}/payment/sync.
func (app *application) SyncOrderPayment(w http.ResponseWriter, r *http.Request, id uuid.UUID, params SyncOrderPaymentParams) {
	v := app.orderViewer(r, params.Token)
	o, err := app.orders.Order(r.Context(), id, v)
	if err != nil {
		app.handleError(w, r, err)
		return
	}
	if app.payments != nil {
		if err := app.payments.Sync(r.Context(), o); err != nil {
			app.handleError(w, r, err)
			return
		}
		if o, err = app.orders.Order(r.Context(), id, v); err != nil {
			app.handleError(w, r, err)
			return
		}
	}
	app.writeJSON(w, r, http.StatusOK, orderJSON(o))
}

func orderJSON(o orders.OrderDetail) Order {
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
	return Order{
		Id:        o.ID,
		Number:    o.Number,
		Status:    OrderStatus(o.Status),
		Payment:   PaymentMethod(o.Payment),
		PaidAt:    o.PaidAt,
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
	}
}

// orderViewer is who asks for an order: the link's token and the logged-in customer, if any.
func (app *application) orderViewer(r *http.Request, token string) orders.Viewer {
	v := orders.Viewer{Token: token}
	if sess, ok := sessionFrom(r.Context()); ok {
		v.UserID, v.VerifiedEmail = sess.User.ID, verifiedEmail(sess.User)
	}
	return v
}

// verifiedEmail is the user's email once they've verified it, else "": an unverified address
// could be anyone's, so it doesn't open the guest orders placed with it.
func verifiedEmail(u auth.User) string {
	if u.EmailVerified {
		return u.Email
	}
	return ""
}
