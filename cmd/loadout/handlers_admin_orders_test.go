package main

import (
	"context"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/gfotev/loadout/internal/carrier"
	"github.com/gfotev/loadout/internal/orders"
)

func TestAdminOrdersAPI(t *testing.T) {
	app := newOrdersApp(t)
	admin := adminCookie(t, app)
	cpu := slugs(t, app)["AMD-AM5-R5-7600-BOX"]
	guest := cookieNamed(cartRequest(t, app, http.MethodPost, "/api/cart/lines", `{"lines":[{"slug":"`+cpu+`","quantity":2}]}`), "cart")
	co := decodeJSON[Checkout](t, get(t, app, "/api/checkout", guest), http.StatusOK)
	placed := decodeJSON[OrderPlaced](t, cartRequest(t, app, http.MethodPost, "/api/checkout", checkoutBody(addressOption(co).Total.AmountCents), guest), http.StatusCreated)
	target := "/api/admin/orders/" + placed.Id.String()

	t.Run("list", func(t *testing.T) {
		list := decode[AdminOrderList](t, get(t, app, "/api/admin/orders?needs_action=true&q="+strconv.FormatInt(placed.Number, 10), admin), http.StatusOK)
		if list.Total != 1 || list.Items[0].Id != placed.Id || list.Items[0].Status != OrderStatusSupplierPending ||
			list.Items[0].Count != 2 || list.Items[0].Total != addressOption(co).Total || list.Items[0].Stale {
			t.Errorf("list %+v", list)
		}
	})

	t.Run("detail", func(t *testing.T) {
		o := decode[AdminOrder](t, get(t, app, target, admin), http.StatusOK)
		if o.Number != placed.Number || o.Token != placed.Token || len(o.Lines) != 1 || o.Lines[0].Sku != "AMD-AM5-R5-7600-BOX" ||
			o.Lines[0].DealerUnit.AmountCents <= 0 || o.DealerTotal.AmountCents+o.Margin.AmountCents != o.Subtotal.AmountCents {
			t.Errorf("order %+v", o)
		}
		want := []AdminOrderEvent{AdminOrderEventSupplierConfirmed, AdminOrderEventSupplierRejected, AdminOrderEventCancel}
		if !slices.Equal(o.Actions, want) || len(o.Transitions) != 1 || o.Transitions[0].Actor != "" {
			t.Errorf("actions %v, transitions %+v", o.Actions, o.Transitions)
		}
	})

	t.Run("confirm", func(t *testing.T) {
		rec := send(t, app, http.MethodPost, target+"/events", `{"event": "supplier_confirmed", "note": "Vali: 12345"}`, admin)
		o := decode[AdminOrder](t, rec, http.StatusOK)
		last := o.Transitions[len(o.Transitions)-1]
		if o.Status != OrderStatusReadyToShip || last.Event != "supplier_confirmed" || last.Note != "Vali: 12345" ||
			!strings.HasPrefix(last.Actor, "admin-") || !slices.Equal(o.Actions, []AdminOrderEvent{AdminOrderEventCancel}) {
			t.Errorf("order %+v", o)
		}
	})

	t.Run("label", func(t *testing.T) {
		if rec := get(t, app, target+"/label", admin); rec.Code != http.StatusNotFound {
			t.Fatalf("before the waybill: status %d", rec.Code)
		}
		app.carrier = carrier.NewMock()
		job := &river.Job[orders.StepArgs]{JobRow: &rivertype.JobRow{}, Args: orders.StepArgs{OrderID: placed.Id, Command: orders.CmdCreateShipment}}
		if err := orders.NewStepWorker(app.orders, nil, nil, app.carrier).Work(context.Background(), job); err != nil {
			t.Fatal(err)
		}
		o := decode[AdminOrder](t, get(t, app, target, admin), http.StatusOK)
		if o.Shipment == nil || o.Shipment.Carrier != "mock" || !strings.HasSuffix(o.Shipment.Number, "-"+strconv.FormatInt(placed.Number, 10)) {
			t.Fatalf("shipment %+v", o.Shipment)
		}
		rec := get(t, app, target+"/label", admin)
		if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "application/pdf" || !strings.HasPrefix(rec.Body.String(), "%PDF-") {
			t.Errorf("status %d, type %q", rec.Code, rec.Header().Get("Content-Type"))
		}
		if rec := get(t, app, target+"/label", guest); rec.Code != http.StatusUnauthorized {
			t.Errorf("guest: status %d", rec.Code)
		}
	})

	t.Run("an event the order can't take", func(t *testing.T) {
		rec := send(t, app, http.MethodPost, target+"/events", `{"event": "supplier_rejected"}`, admin)
		if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), `"invalid_transition"`) {
			t.Errorf("status %d: %s", rec.Code, rec.Body)
		}
	})

	t.Run("refused", func(t *testing.T) {
		for _, body := range []string{`{"event": "picked_up"}`, `{"event": "refund_succeeded"}`} {
			if rec := send(t, app, http.MethodPost, target+"/events", body, admin); rec.Code != http.StatusUnprocessableEntity {
				t.Errorf("%s: status %d: %s", body, rec.Code, rec.Body)
			}
		}
	})

	t.Run("customers get no access", func(t *testing.T) {
		if rec := get(t, app, target, guest); rec.Code != http.StatusUnauthorized {
			t.Errorf("status %d", rec.Code)
		}
	})
}
