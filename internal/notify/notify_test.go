package notify

import (
	"bytes"
	"context"
	"io"
	"mime"
	"mime/quotedprintable"
	"net/mail"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

type recorder struct{ sent []Message }

func (r *recorder) Send(_ context.Context, m Message) error {
	r.sent = append(r.sent, m)
	return nil
}

func TestLinks(t *testing.T) {
	r := &recorder{}
	s := NewService(r, "https://loadout.bg/")
	ctx := context.Background()
	if err := s.VerifyEmail(ctx, "a@b.bg", "tok+/="); err != nil {
		t.Fatal(err)
	}
	if err := s.ResetPassword(ctx, "a@b.bg", "abc"); err != nil {
		t.Fatal(err)
	}
	if err := s.AccountExists(ctx, "a@b.bg"); err != nil {
		t.Fatal(err)
	}

	for i, want := range []string{
		"https://loadout.bg/verify-email?token=tok%2B%2F%3D\n",
		"https://loadout.bg/reset-password?token=abc\n",
		"https://loadout.bg/forgot-password\n",
	} {
		if !strings.Contains(r.sent[i].Text, want) {
			t.Errorf("email %d doesn't contain %q:\n%s", i, want, r.sent[i].Text)
		}
	}
}

func TestOrderReceived(t *testing.T) {
	r := &recorder{}
	s := NewService(r, "https://loadout.bg")
	id := uuid.MustParse("6f1c2b8e-0d4a-4c1e-9a77-2b1f0c3d4e5f")
	err := s.OrderReceived(context.Background(), Order{
		ID:             id,
		Number:         1001,
		Token:          "t+k",
		Email:          "ivan@example.bg",
		Name:           "Иван Иванов",
		Phone:          "0888 123 456",
		Address:        "ул. Граф Игнатиев 1, 1000 София",
		Lines:          []OrderLine{{Name: "Процесор AMD Ryzen 5 7600", Quantity: 2, LineCents: 400_00}},
		SubtotalCents:  400_00,
		DeliveryCents:  5_99,
		TotalCents:     405_99,
		CashOnDelivery: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	m := r.sent[0]
	if m.To != "ivan@example.bg" || m.Subject != "Поръчка № 1001 е приета" {
		t.Errorf("to %q, subject %q", m.To, m.Subject)
	}
	for _, want := range []string{
		"2 × Процесор AMD Ryzen 5 7600: 400,00\u00a0€\n",
		"Общо: 405,99\u00a0€ (с ДДС)\n",
		"наложен платеж",
		"https://loadout.bg/order/" + id.String() + "?token=t%2Bk\n",
	} {
		if !strings.Contains(m.Text, want) {
			t.Errorf("email doesn't contain %q:\n%s", want, m.Text)
		}
	}
}

func TestOrderReceivedPaidByCard(t *testing.T) {
	r := &recorder{}
	s := NewService(r, "https://loadout.bg")
	err := s.OrderReceived(context.Background(), Order{ID: uuid.New(), Number: 1002, PaidByCard: true})
	if err != nil {
		t.Fatal(err)
	}
	if text := r.sent[0].Text; !strings.Contains(text, "Плащане: с карта") || strings.Contains(text, "наложен платеж") {
		t.Errorf("email:\n%s", text)
	}
}

func TestBuildMessage(t *testing.T) {
	from := &mail.Address{Name: "Loadout", Address: "noreply@loadout.bg"}
	to := &mail.Address{Address: "иван@example.bg"}
	text := "Здравейте,\n\n" + strings.Repeat("дълъг ред ", 20) + "\n"
	raw := buildMessage(from, to, Message{Subject: "Потвърдете имейла си", Text: text}, time.Unix(0, 0))

	msg, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	subject, err := new(mime.WordDecoder).DecodeHeader(msg.Header.Get("Subject"))
	if err != nil || subject != "Потвърдете имейла си" {
		t.Errorf("subject %q, %v", subject, err)
	}
	if !strings.HasSuffix(msg.Header.Get("Message-ID"), "@loadout.bg>") {
		t.Errorf("Message-ID %q", msg.Header.Get("Message-ID"))
	}
	body, err := io.ReadAll(quotedprintable.NewReader(msg.Body))
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.ReplaceAll(string(body), "\r\n", "\n"); got != text {
		t.Errorf("body %q, want %q", got, text)
	}
	for _, line := range strings.Split(string(raw), "\r\n") {
		if len(line) > 78 {
			t.Errorf("line longer than 78 bytes: %q", line)
		}
	}
}

func TestOrderConfirmed(t *testing.T) {
	r := &recorder{}
	s := NewService(r, "https://loadout.bg")
	id := uuid.New()
	if err := s.OrderConfirmed(context.Background(), Order{ID: id, Number: 1003, Token: "t", Email: "ivan@example.bg", TotalCents: 405_99, CashOnDelivery: true}); err != nil {
		t.Fatal(err)
	}
	m := r.sent[0]
	if m.To != "ivan@example.bg" || m.Subject != "Поръчка № 1003 е потвърдена" {
		t.Errorf("to %q, subject %q", m.To, m.Subject)
	}
	for _, want := range []string{"платете на куриера 405,99\u00a0€", "https://loadout.bg/order/" + id.String() + "?token=t\n"} {
		if !strings.Contains(m.Text, want) {
			t.Errorf("email doesn't contain %q:\n%s", want, m.Text)
		}
	}
}

func TestOrderCancelled(t *testing.T) {
	for _, tt := range []struct {
		name   string
		paid   bool
		refund bool
	}{
		{"unpaid", false, false},
		{"paid by card", true, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			r := &recorder{}
			s := NewService(r, "https://loadout.bg")
			if err := s.OrderCancelled(context.Background(), Order{ID: uuid.New(), Number: 1004, TotalCents: 405_99, PaidByCard: tt.paid}); err != nil {
				t.Fatal(err)
			}
			m := r.sent[0]
			if m.Subject != "Поръчка № 1004 е отказана" {
				t.Errorf("subject %q", m.Subject)
			}
			if got := strings.Contains(m.Text, "Връщаме 405,99\u00a0€ по картата"); got != tt.refund {
				t.Errorf("mentions the refund: %v, want %v:\n%s", got, tt.refund, m.Text)
			}
		})
	}
}

func TestOrderEmailsWithAssembly(t *testing.T) {
	r := &recorder{}
	s := NewService(r, "https://loadout.bg")
	o := Order{
		ID:     uuid.New(),
		Number: 1004,
		Lines: []OrderLine{
			{Name: "Процесор", Quantity: 1, LineCents: 200_00, InBuild: true},
			{Name: "Мишка", Quantity: 1, LineCents: 40_00},
			{Name: "Памет", Quantity: 2, LineCents: 160_00, InBuild: true},
		},
		DeliveryCents: 5_99,
		AssemblyCents: 35_00,
		BuildCents:    395_00,
		TotalCents:    440_99,
		Assembly:      true,
	}
	if err := s.OrderReceived(context.Background(), o); err != nil {
		t.Fatal(err)
	}
	if err := s.OrderConfirmed(context.Background(), o); err != nil {
		t.Fatal(err)
	}
	// The PC first, its parts under it, then the mouse on its own.
	wantGroup := "Компютър по поръчка, сглобен: 395,00\u00a0€\n" +
		"  1 × Процесор: 200,00\u00a0€\n" +
		"  2 × Памет: 160,00\u00a0€\n" +
		"  Сглобяване: 35,00\u00a0€\n" +
		"1 × Мишка: 40,00\u00a0€\n"
	for _, tc := range []struct {
		email int // 0: received, 1: confirmed
		want  string
	}{
		{0, wantGroup},
		{0, "Сглобяване на компютъра: 35,00\u00a0€\n"},
		{1, "Сглобяваме компютъра и ще ви пишем"},
	} {
		if text := r.sent[tc.email].Text; !strings.Contains(text, tc.want) {
			t.Errorf("email doesn't contain %q:\n%s", tc.want, text)
		}
	}
}
