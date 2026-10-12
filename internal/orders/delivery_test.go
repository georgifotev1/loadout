package orders

import (
	"context"
	"slices"
	"testing"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/gfotev/loadout/internal/carrier"
	"github.com/gfotev/loadout/internal/configurator"
)

// syncOffices copies the mock courier's offices, as the daily job does.
func syncOffices(t *testing.T, s *Service) {
	t.Helper()
	w := NewOfficeSyncWorker(s, carrier.NewMock())
	if err := w.Work(context.Background(), &river.Job[OfficeSyncArgs]{JobRow: &rivertype.JobRow{}}); err != nil {
		t.Fatal(err)
	}
}

func officeIDs(list []Office) []string {
	ids := make([]string, len(list))
	for i, o := range list {
		ids[i] = o.ID
	}
	return ids
}

func TestOffices(t *testing.T) {
	s := newOrderService(t)
	ctx := context.Background()
	syncOffices(t, s)

	for _, tc := range []struct {
		text string
		kind carrier.OfficeKind
		want []string
	}{
		{"софия", carrier.KindOffice, []string{"102", "104", "103", "101"}}, // by name
		{"СОФИЯ", carrier.KindAPT, []string{"106", "105"}},                  // by name: "…Изток" before "…Студентски град"
		{"  лозенец  софия ", carrier.KindOffice, []string{"102"}},
		{"1164", carrier.KindOffice, []string{"102"}},
		{"позитано", carrier.KindOffice, []string{"101"}},
		{"варна", carrier.KindAPT, []string{"303"}},
		{"с", carrier.KindOffice, nil}, // too short
		{"%", carrier.KindOffice, nil},
		{"с_фия", carrier.KindOffice, nil},
		{"лондон", carrier.KindOffice, nil},
	} {
		got, err := s.SearchOffices(ctx, tc.text, tc.kind)
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(officeIDs(got), tc.want) {
			t.Errorf("%q %s: got %v, want %v", tc.text, tc.kind, officeIDs(got), tc.want)
		}
	}
	got, err := s.SearchOffices(ctx, "позитано", carrier.KindOffice)
	if err != nil {
		t.Fatal(err)
	}
	if o := got[0]; o.Name != "София Център" || o.City != "София" || o.Postcode != "1000" ||
		o.Address != "ул. Позитано 5" || o.Hours == "" || o.Kind != carrier.KindOffice {
		t.Errorf("office %+v", o)
	}

	t.Run("a sync replaces the offices", func(t *testing.T) {
		moved := Office{ID: "101", Kind: carrier.KindOffice, Name: "София Нов център", City: "София", Postcode: "1000", Address: "ул. Позитано 7"}
		if err := s.replaceOffices(ctx, []Office{moved}); err != nil {
			t.Fatal(err)
		}
		got, err := s.SearchOffices(ctx, "софия", carrier.KindOffice)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 || got[0] != moved {
			t.Errorf("after the sync: %+v", got)
		}
	})

	t.Run("an empty list is refused", func(t *testing.T) {
		if err := s.replaceOffices(ctx, nil); err == nil {
			t.Fatal("no error")
		}
		got, err := s.SearchOffices(ctx, "софия", carrier.KindOffice)
		if err != nil || len(got) != 1 {
			t.Errorf("offices %v, %v; want yesterday's", officeIDs(got), err)
		}
	})
}

func TestCheckoutToOffice(t *testing.T) {
	s, _ := newCheckoutService(t)
	ctx := context.Background()
	syncOffices(t, s)
	owner := guestCart(t, s, AddLine{Slug: "cpu", Quantity: 1})

	q, err := s.Quote(ctx, owner)
	if err != nil {
		t.Fatal(err)
	}
	want := []DeliveryQuote{
		{DeliveryOffice, OfficeDeliveryCents, 200_00 + OfficeDeliveryCents, true},
		{DeliveryAPT, APTDeliveryCents, 200_00 + APTDeliveryCents, true},
		{DeliveryAddress, AddressDeliveryCents, 200_00 + AddressDeliveryCents, true},
	}
	if !slices.Equal(q.Deliveries, want) {
		t.Errorf("deliveries %+v", q.Deliveries)
	}

	in := checkoutInput(200_00 + OfficeDeliveryCents)
	in.Delivery = DeliveryOffice
	in.City, in.Postcode, in.Address = "", "", "" // not asked for an office

	for _, tc := range []struct {
		name     string
		delivery Delivery
		officeID string
	}{
		{"an office the courier doesn't have", DeliveryOffice, "999"},
		{"a locker as an office", DeliveryOffice, "105"},
		{"an office as a locker", DeliveryAPT, "101"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := in
			in.Delivery, in.OfficeID = tc.delivery, tc.officeID
			in.ExpectedTotalCents = 200_00 + tc.delivery.Cents()
			_, err := s.Checkout(ctx, owner, in)
			if got := fieldCodes(t, err); !slices.Equal(got, []string{"office_id: unknown"}) {
				t.Errorf("errors %v", got)
			}
		})
	}

	t.Run("the total is the office's", func(t *testing.T) {
		in := in
		in.OfficeID = "102"
		in.ExpectedTotalCents = addressTotal(q)
		_, err := s.Checkout(ctx, owner, in)
		if got := fieldCodes(t, err); !slices.Equal(got, []string{"expected_total_cents: changed"}) {
			t.Errorf("errors %v", got)
		}
	})

	in.OfficeID = " 102 "
	placed, err := s.Checkout(ctx, owner, in)
	if err != nil {
		t.Fatal(err)
	}
	o, err := s.Order(ctx, placed.ID, Viewer{Token: placed.Token})
	if err != nil {
		t.Fatal(err)
	}
	if o.Delivery != DeliveryOffice || o.OfficeID != "102" || o.OfficeName != "София Лозенец" ||
		o.City != "София" || o.Postcode != "1164" || o.Address != "бул. Черни връх 32" ||
		o.DeliveryCents != OfficeDeliveryCents || o.TotalCents != 200_00+OfficeDeliveryCents {
		t.Errorf("order %+v", o)
	}
	if r := shipmentRequest(o); r.OfficeID != "102" {
		t.Errorf("shipment request %+v", r)
	}
	if got, want := orderEmail(o).Address, "офис „София Лозенец“, бул. Черни връх 32, 1164 София"; got != want {
		t.Errorf("email address %q, want %q", got, want)
	}
}

func TestAssembledPCToALocker(t *testing.T) {
	s, cat := newCheckoutService(t)
	cat.parts = append(cat.parts, categoryPart("pc-cpu", "cpu", 200_00))
	s.builds = &fakeBuilds{ok: []string{"pc-cpu"}}
	ctx := context.Background()
	syncOffices(t, s)
	_, token, err := s.AddToCart(ctx, GuestCart(""), []AddLine{{Slug: "pc-cpu", Quantity: 1}}, true)
	if err != nil {
		t.Fatal(err)
	}
	owner := GuestCart(token)

	q, err := s.Quote(ctx, owner)
	if err != nil {
		t.Fatal(err)
	}
	if !q.Assembly.Chosen {
		t.Fatalf("assembly %+v", q.Assembly)
	}
	for _, d := range q.Deliveries {
		if d.Available != (d.Delivery != DeliveryAPT) {
			t.Errorf("%s: available %v", d.Delivery, d.Available)
		}
		if d.TotalCents != 200_00+configurator.AssemblyCents+d.Cents {
			t.Errorf("%s: total %d", d.Delivery, d.TotalCents)
		}
	}

	in := checkoutInput(200_00 + configurator.AssemblyCents + APTDeliveryCents)
	in.Delivery, in.OfficeID = DeliveryAPT, "105"
	_, err = s.Checkout(ctx, owner, in)
	if got := fieldCodes(t, err); !slices.Equal(got, []string{"delivery_method: unavailable"}) {
		t.Errorf("errors %v", got)
	}
}
