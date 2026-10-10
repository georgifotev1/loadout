package carrier

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"
)

func TestMock(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)
	m := NewMock()
	m.Now = func() time.Time { return now }

	s, err := m.CreateShipment(ctx, Request{Reference: "1001"})
	if err != nil {
		t.Fatal(err)
	}
	if want := "M1791547200-1001"; s.Number != want {
		t.Fatalf("number %q, want %q", s.Number, want)
	}

	for _, tc := range []struct {
		after time.Duration
		want  Tracking
	}{
		{0, Tracking{}},
		{2 * time.Minute, Tracking{PickedUp: true}},
		{5 * time.Minute, Tracking{PickedUp: true, Delivered: true}},
	} {
		now = time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC).Add(tc.after)
		got, err := m.Track(ctx, s.Number)
		if err != nil || got != tc.want {
			t.Errorf("after %v: %+v, %v; want %+v", tc.after, got, err, tc.want)
		}
	}

	pdf, err := m.Label(ctx, s.Number)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(pdf, []byte("%PDF-")) || !bytes.Contains(pdf, []byte("(Order 1001) '")) {
		t.Errorf("label:\n%s", pdf)
	}

	for _, n := range []string{"", "1001", "Mabc-1001", "M1791547200-", "X1791547200-1001"} {
		if _, err := m.Track(ctx, n); !errors.Is(err, ErrUnknownShipment) {
			t.Errorf("Track(%q) err %v", n, err)
		}
	}
	if _, err := m.CreateShipment(ctx, Request{Reference: "a-b"}); err == nil {
		t.Error("a reference with '-' was accepted")
	}
}
