package main

import (
	"net/http"

	"github.com/google/uuid"

	"github.com/gfotev/loadout/internal/auth"
	"github.com/gfotev/loadout/internal/orders"
)

// ListAdminOrders handles GET /api/admin/orders.
func (app *application) ListAdminOrders(w http.ResponseWriter, r *http.Request, params ListAdminOrdersParams) {
	page := valueOr(params.Page, 1)
	pageSize := valueOr(params.PageSize, defaultPageSize)

	res, err := app.orders.AdminOrders(r.Context(), orders.AdminOrderFilter{
		Text:        params.Q,
		Status:      orders.Status(params.Status),
		NeedsAction: params.NeedsAction,
		Page:        page,
		PageSize:    pageSize,
	})
	if err != nil {
		app.handleError(w, r, err)
		return
	}

	resp := AdminOrderList{
		Items:    make([]AdminOrderSummary, len(res.Orders)),
		Page:     page,
		PageSize: pageSize,
		Total:    res.Total,
	}
	for i, o := range res.Orders {
		resp.Items[i] = AdminOrderSummary{
			Id:          o.ID,
			Number:      o.Number,
			Status:      OrderStatus(o.Status),
			Payment:     PaymentMethod(o.Payment),
			PaidAt:      o.PaidAt,
			CreatedAt:   o.CreatedAt,
			StatusSince: o.StatusSince,
			Stale:       o.Stale,
			Name:        o.Name,
			Email:       o.Email,
			Count:       o.Count,
			Total:       eur(o.TotalCents),
		}
	}
	app.writeJSON(w, r, http.StatusOK, resp)
}

// GetAdminOrder handles GET /api/admin/orders/{order_id}.
func (app *application) GetAdminOrder(w http.ResponseWriter, r *http.Request, id uuid.UUID) {
	o, err := app.orders.AdminOrder(r.Context(), id)
	if err != nil {
		app.handleError(w, r, err)
		return
	}
	app.writeJSON(w, r, http.StatusOK, adminOrderJSON(o))
}

// ApplyAdminOrderEvent handles POST /api/admin/orders/{order_id}/events.
func (app *application) ApplyAdminOrderEvent(w http.ResponseWriter, r *http.Request, id uuid.UUID) {
	var in AdminOrderEventInput
	if err := readJSON(w, r, &in); err != nil {
		app.handleError(w, r, err)
		return
	}
	sess, ok := sessionFrom(r.Context())
	if !ok { // adminOnly lets no one else through
		app.handleError(w, r, auth.ErrUnauthenticated)
		return
	}
	by := orders.Actor{Email: sess.User.Email, Note: in.Note}
	if err := app.orders.AdminApply(r.Context(), id, orders.Event(in.Event), by); err != nil {
		app.handleError(w, r, err)
		return
	}
	o, err := app.orders.AdminOrder(r.Context(), id)
	if err != nil {
		app.handleError(w, r, err)
		return
	}
	app.writeJSON(w, r, http.StatusOK, adminOrderJSON(o))
}

func adminOrderJSON(o orders.AdminOrderDetail) AdminOrder {
	lines := make([]AdminOrderLine, len(o.Lines))
	for i, l := range o.Lines {
		lines[i] = AdminOrderLine{
			ProductId:  l.ProductID,
			Slug:       l.Slug,
			Name:       l.Name,
			Image:      l.Image,
			Quantity:   l.Quantity,
			UnitPrice:  eur(l.UnitCents),
			LineTotal:  eur(l.TotalCents),
			InBuild:    l.InBuild,
			Provider:   l.Provider,
			Sku:        l.ProviderSKU,
			DealerUnit: eur(l.DealerCents),
			VatBp:      l.VATbp,
		}
	}
	transitions := make([]OrderTransition, len(o.Transitions))
	for i, t := range o.Transitions {
		transitions[i] = OrderTransition{
			Seq:   t.Seq,
			Event: string(t.Event),
			From:  OrderStatus(t.From),
			To:    OrderStatus(t.To),
			At:    t.At,
			Actor: t.Actor.Email,
			Note:  t.Actor.Note,
		}
	}
	actions := make([]AdminOrderEvent, len(o.Actions))
	for i, ev := range o.Actions {
		actions[i] = AdminOrderEvent(ev)
	}
	return AdminOrder{
		Id:          o.ID,
		Number:      o.Number,
		Token:       o.Token,
		Status:      OrderStatus(o.Status),
		Payment:     PaymentMethod(o.Payment),
		PaidAt:      o.PaidAt,
		CreatedAt:   o.CreatedAt,
		StatusSince: o.StatusSince,
		Stale:       o.Stale,
		UserId:      o.UserID,
		Name:        o.Name,
		Email:       o.Email,
		Phone:       o.Phone,
		City:        o.City,
		Postcode:    o.Postcode,
		Address:     o.Address,
		Lines:       lines,
		Subtotal:    eur(o.SubtotalCents),
		Delivery:    eur(o.DeliveryCents),
		Assembly:    optionalEUR(o.Assembly, o.AssemblyCents),
		Total:       eur(o.TotalCents),
		DealerTotal: eur(o.DealerCents),
		Margin:      eur(o.MarginCents),
		Transitions: transitions,
		Actions:     actions,
	}
}
