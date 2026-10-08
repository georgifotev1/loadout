// Package notify sends the store's emails: the templates (in Bulgarian) and a Sender that
// delivers them. Other modules call its methods from their jobs, never inside a transaction.
package notify

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/google/uuid"

	"github.com/gfotev/loadout/internal/money"
)

// Message is one plain-text email.
type Message struct {
	To      string
	Subject string
	Text    string
}

// Sender delivers a message: SMTPSender (Mailpit locally), or a fake in tests.
type Sender interface {
	Send(ctx context.Context, m Message) error
}

type Service struct {
	sender  Sender
	siteURL string
}

// NewService returns the notify service. siteURL is the storefront's public origin, for links
// in emails.
func NewService(s Sender, siteURL string) *Service {
	return &Service{sender: s, siteURL: strings.TrimSuffix(siteURL, "/")}
}

// VerifyEmail sends a new customer the link that verifies their email.
func (s *Service) VerifyEmail(ctx context.Context, to, token string) error {
	return s.sender.Send(ctx, Message{
		To:      to,
		Subject: "Потвърдете имейла си в Loadout",
		Text: lines(
			"Здравейте,",
			"",
			"Благодарим, че си направихте профил в Loadout. Потвърдете имейла си от този линк:",
			"",
			s.link("/verify-email", token),
			"",
			"Линкът работи 48 часа.",
			"",
			"Ако не сте се регистрирали вие, не правете нищо: без потвърждение имейлът ви не се",
			"свързва с поръчки.",
		),
	})
}

// ResetPassword sends the link that sets a new password.
func (s *Service) ResetPassword(ctx context.Context, to, token string) error {
	return s.sender.Send(ctx, Message{
		To:      to,
		Subject: "Нова парола за Loadout",
		Text: lines(
			"Здравейте,",
			"",
			"Някой поиска нова парола за профила ви в Loadout. Задайте я от този линк:",
			"",
			s.link("/reset-password", token),
			"",
			"Линкът работи 1 час и само веднъж. Новата парола ви изписва от всички устройства.",
			"",
			"Ако не сте били вие, не правете нищо: паролата ви остава същата.",
		),
	})
}

// AccountExists answers a sign-up with an email that already has an account, instead of
// telling the person on the sign-up page.
func (s *Service) AccountExists(ctx context.Context, to string) error {
	return s.sender.Send(ctx, Message{
		To:      to,
		Subject: "Вече имате профил в Loadout",
		Text: lines(
			"Здравейте,",
			"",
			"Някой опита да направи нов профил в Loadout с този имейл, но вие вече имате профил.",
			"",
			"Влезте от тук: "+s.siteURL+"/login",
			"Забравена парола? Задайте нова от тук: "+s.siteURL+"/forgot-password",
			"",
			"Ако не сте били вие, не правете нищо.",
		),
	})
}

// Order is what an order email shows.
type Order struct {
	ID     uuid.UUID
	Number int64
	// Token opens the order page without logging in (the link in the email).
	Token          string
	Email          string
	Name           string
	Phone          string
	Address        string // street, postcode and city on one line
	Lines          []OrderLine
	SubtotalCents  money.Cents
	DeliveryCents  money.Cents
	TotalCents     money.Cents
	CashOnDelivery bool
	PaidByCard     bool
}

// OrderLine is one product of an order email.
type OrderLine struct {
	Name      string
	Quantity  int
	LineCents money.Cents
}

// OrderReceived confirms a new order: what was ordered, the totals, where it goes, and the link
// to follow it.
// TODO(phase-7): what the email must repeat by law (terms, withdrawal) comes with the lawyer's
// review (step 8).
func (s *Service) OrderReceived(ctx context.Context, o Order) error {
	ls := []string{
		"Здравейте, " + o.Name + ",",
		"",
		"Благодарим за поръчката! Получихме я и проверяваме наличността при доставчика.",
		"Ще ви пишем, когато я потвърдим и когато пратката тръгне.",
		"",
		fmt.Sprintf("Поръчка № %d", o.Number),
		"",
	}
	for _, l := range o.Lines {
		ls = append(ls, fmt.Sprintf("%d × %s: %s", l.Quantity, l.Name, money.Format(l.LineCents)))
	}
	ls = append(ls,
		"",
		"Междинна сума: "+money.Format(o.SubtotalCents),
		"Доставка: "+money.Format(o.DeliveryCents),
		"Общо: "+money.Format(o.TotalCents)+" (с ДДС)",
		"",
	)
	switch {
	case o.CashOnDelivery:
		ls = append(ls, "Плащане: наложен платеж, плащате на куриера при получаване.")
	case o.PaidByCard:
		ls = append(ls, "Плащане: с карта, получихме го.")
	}
	ls = append(ls,
		"Доставка до: "+o.Address,
		"Телефон: "+o.Phone,
		"",
		"Следете поръчката от тук:",
		"",
		s.orderLink(o),
	)
	return s.sender.Send(ctx, Message{
		To:      o.Email,
		Subject: fmt.Sprintf("Поръчка № %d е приета", o.Number),
		Text:    lines(ls...),
	})
}

// OrderConfirmed tells the customer the supplier has every part and the parcel is being prepared.
func (s *Service) OrderConfirmed(ctx context.Context, o Order) error {
	ls := []string{
		"Здравейте, " + o.Name + ",",
		"",
		fmt.Sprintf("Потвърдихме поръчка № %d: всички продукти са налични.", o.Number),
		"Подготвяме пратката и ще ви пишем, когато тръгне.",
		"",
	}
	if o.CashOnDelivery {
		ls = append(ls, "При получаване платете на куриера "+money.Format(o.TotalCents)+".", "")
	}
	ls = append(ls,
		"Следете поръчката от тук:",
		"",
		s.orderLink(o),
	)
	return s.sender.Send(ctx, Message{
		To:      o.Email,
		Subject: fmt.Sprintf("Поръчка № %d е потвърдена", o.Number),
		Text:    lines(ls...),
	})
}

// OrderCancelled tells the customer their order is cancelled and, if they paid by card, that the
// money is on its way back.
func (s *Service) OrderCancelled(ctx context.Context, o Order) error {
	ls := []string{
		"Здравейте, " + o.Name + ",",
		"",
		fmt.Sprintf("Поръчка № %d е отказана и няма да бъде изпратена.", o.Number),
		"",
	}
	if o.PaidByCard {
		ls = append(ls,
			"Връщаме "+money.Format(o.TotalCents)+" по картата, с която платихте. Според банката ви",
			"парите пристигат до 5–10 работни дни.",
			"",
		)
	}
	// TODO(phase-8): how to reach us, once the contacts page exists; replies go to noreply.
	ls = append(ls,
		"Поръчката:",
		"",
		s.orderLink(o),
	)
	return s.sender.Send(ctx, Message{
		To:      o.Email,
		Subject: fmt.Sprintf("Поръчка № %d е отказана", o.Number),
		Text:    lines(ls...),
	})
}

// orderLink opens the order page without logging in.
func (s *Service) orderLink(o Order) string {
	return s.siteURL + "/order/" + o.ID.String() + "?" + url.Values{"token": {o.Token}}.Encode()
}

func (s *Service) link(path, token string) string {
	return s.siteURL + path + "?" + url.Values{"token": {token}}.Encode()
}

func lines(ls ...string) string {
	return strings.Join(ls, "\n") + "\n"
}
