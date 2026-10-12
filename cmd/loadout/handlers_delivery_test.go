package main

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"testing"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/gfotev/loadout/internal/carrier"
	"github.com/gfotev/loadout/internal/orders"
)

func TestDeliveryAPI(t *testing.T) {
	app := newOrdersApp(t)
	sync := orders.NewOfficeSyncWorker(app.orders, carrier.NewMock())
	if err := sync.Work(context.Background(), &river.Job[orders.OfficeSyncArgs]{JobRow: &rivertype.JobRow{}}); err != nil {
		t.Fatal(err)
	}

	search := func(q, method string) *http.Response {
		return get(t, app, "/api/delivery/offices?"+url.Values{"q": {q}, "method": {method}}.Encode()).Result()
	}
	rec := get(t, app, "/api/delivery/offices?"+url.Values{"q": {"лозенец"}, "method": {"office"}}.Encode())
	list := decodeJSON[OfficeList](t, rec, http.StatusOK)
	if len(list.Offices) != 1 || list.Offices[0].Id != "102" || list.Offices[0].Method != DeliveryMethodOffice ||
		list.Offices[0].Hours == "" {
		t.Errorf("offices %+v", list.Offices)
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "public, max-age=300" {
		t.Errorf("Cache-Control %q", cc)
	}
	if res := search("софия", "address"); res.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("method address: status %d", res.StatusCode)
	}

	cpu := slugs(t, app)["AMD-AM5-R5-7600-BOX"]
	guest := cookieNamed(cartRequest(t, app, http.MethodPost, "/api/cart/lines", `{"lines":[{"slug":"`+cpu+`","quantity":1}]}`), "cart")
	co := decodeJSON[Checkout](t, get(t, app, "/api/checkout", guest), http.StatusOK)
	if len(co.Deliveries) != 3 || co.Deliveries[0].Method != DeliveryMethodOffice || !co.Deliveries[0].Available {
		t.Fatalf("deliveries %+v", co.Deliveries)
	}
	body := `{"name": "Иван Иванов", "email": "ivan@example.bg", "phone": "0888 123 456",
		"delivery_method": "office", "office_id": "102", "payment": "cod", "accept_terms": true,
		"expected_total_cents": ` + strconv.FormatInt(co.Deliveries[0].Total.AmountCents, 10) + `}`
	placed := decodeJSON[OrderPlaced](t, cartRequest(t, app, http.MethodPost, "/api/checkout", body, guest), http.StatusCreated)
	o := decodeJSON[Order](t, get(t, app, "/api/orders/"+placed.Id.String()+"?token="+placed.Token), http.StatusOK)
	if o.DeliveryMethod != DeliveryMethodOffice || o.Office == nil || *o.Office != (OrderOffice{Id: "102", Name: "София Лозенец"}) ||
		o.City != "София" || o.Postcode != "1164" || o.Delivery != co.Deliveries[0].Price {
		t.Errorf("order %+v", o)
	}
}
