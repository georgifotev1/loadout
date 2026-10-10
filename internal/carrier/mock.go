package carrier

import (
	"bytes"
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Mock is a courier that exists only in our process. A shipment is picked up PickupAfter its
// waybill was made and delivered DeliverAfter it. The waybill number holds when it was made, so
// Mock keeps no state and its shipments survive a restart.
type Mock struct {
	PickupAfter  time.Duration
	DeliverAfter time.Duration
	Now          func() time.Time // time.Now if nil
}

// NewMock returns a Mock whose parcels are picked up after 2 minutes and delivered after 5, so a
// local order goes all the way in minutes.
func NewMock() *Mock {
	return &Mock{PickupAfter: 2 * time.Minute, DeliverAfter: 5 * time.Minute}
}

func (m *Mock) Name() string { return "mock" }

func (m *Mock) now() time.Time {
	if m.Now == nil {
		return time.Now()
	}
	return m.Now()
}

// CreateShipment makes a number like M1760000000-1001: "M", the Unix time it was made and the
// reference.
func (m *Mock) CreateShipment(ctx context.Context, r Request) (Shipment, error) {
	if r.Reference == "" || strings.Contains(r.Reference, "-") {
		return Shipment{}, fmt.Errorf("carrier: mock reference %q: must be set and have no '-'", r.Reference)
	}
	number := fmt.Sprintf("M%d-%s", m.now().Unix(), r.Reference)
	return Shipment{Number: number}, nil
}

func (m *Mock) Track(ctx context.Context, number string) (Tracking, error) {
	made, _, err := parseMockNumber(number)
	if err != nil {
		return Tracking{}, err
	}
	age := m.now().Sub(made)
	return Tracking{PickedUp: age >= m.PickupAfter, Delivered: age >= m.DeliverAfter}, nil
}

// Label is a one-page PDF with the waybill number and the reference. It has no recipient: Mock
// keeps nothing else, and the PDF's built-in font has no Cyrillic.
func (m *Mock) Label(ctx context.Context, number string) ([]byte, error) {
	made, ref, err := parseMockNumber(number)
	if err != nil {
		return nil, err
	}
	return labelPDF(
		"MOCK CARRIER - NOT A REAL WAYBILL",
		"Waybill "+number,
		"Order "+ref,
		"Made "+made.UTC().Format(time.RFC3339),
	), nil
}

func parseMockNumber(number string) (time.Time, string, error) {
	unix, ref, ok := strings.Cut(strings.TrimPrefix(number, "M"), "-")
	sec, err := strconv.ParseInt(unix, 10, 64)
	if !strings.HasPrefix(number, "M") || !ok || err != nil || ref == "" {
		return time.Time{}, "", fmt.Errorf("%w: %q", ErrUnknownShipment, number)
	}
	return time.Unix(sec, 0), ref, nil
}

// labelPDF writes an A6 page with ASCII lines in Helvetica.
func labelPDF(lines ...string) []byte {
	var content bytes.Buffer
	content.WriteString("BT /F1 12 Tf 20 380 Td 16 TL\n")
	for _, l := range lines {
		l = strings.NewReplacer(`\`, `\\`, "(", `\(`, ")", `\)`).Replace(l)
		fmt.Fprintf(&content, "(%s) '\n", l)
	}
	content.WriteString("ET\n")

	objects := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 298 420] /Resources << /Font << /F1 4 0 R >> >> /Contents 5 0 R >>",
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%sendstream", content.Len(), content.String()),
	}
	var b bytes.Buffer
	b.WriteString("%PDF-1.4\n")
	offsets := make([]int, len(objects))
	for i, o := range objects {
		offsets[i] = b.Len()
		fmt.Fprintf(&b, "%d 0 obj\n%s\nendobj\n", i+1, o)
	}
	xref := b.Len()
	fmt.Fprintf(&b, "xref\n0 %d\n0000000000 65535 f \n", len(objects)+1)
	for _, off := range offsets {
		fmt.Fprintf(&b, "%010d 00000 n \n", off)
	}
	fmt.Fprintf(&b, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objects)+1, xref)
	return b.Bytes()
}
