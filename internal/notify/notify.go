// Package notify sends the store's emails: the templates (in Bulgarian) and a Sender that
// delivers them. Other modules call its methods from their jobs, never inside a transaction.
package notify

import (
	"context"
	"net/url"
	"strings"
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

func (s *Service) link(path, token string) string {
	return s.siteURL + path + "?" + url.Values{"token": {token}}.Encode()
}

func lines(ls ...string) string {
	return strings.Join(ls, "\n") + "\n"
}
