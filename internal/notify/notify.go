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
	Token         string
	Email         string
	Name          string
	Phone         string
	Address       string // street, postcode and city on one line
	Lines         []OrderLine
	SubtotalCents money.Cents
	DeliveryCents money.Cents
	AssemblyCents money.Cents // 0 unless Assembly
	BuildCents    money.Cents // the assembled PC: its parts and the fee; 0 unless Assembly
	TotalCents    money.Cents
	// Assembly is whether we assemble the PC from the order's parts.
	Assembly       bool
	CashOnDelivery bool
	PaidByCard     bool
	// TrackingNumber is the courier's waybill, "" before it's made; TrackingURL follows it, ""
	// if the courier has no such page.
	TrackingNumber string
	TrackingURL    string
}

// OrderLine is one product of an order email.
type OrderLine struct {
	Name      string
	Quantity  int
	LineCents money.Cents
	InBuild   bool // a part of the assembled PC
}

// Seller is who the customer buys from, as the order email names them. The contacts page
// (web/src/lib/company.ts) shows the same.
// TODO(company): the registered company's details, once the company exists.
var Seller = struct {
	Name, ID, Address, Email, Phone string
}{
	Name:    "[Име на фирмата] ЕООД",
	ID:      "[ЕИК]",
	Address: "[Седалище и адрес на управление]",
	Email:   "[имейл за връзка]",
	Phone:   "[телефон]",
}

// OrderReceived confirms a new order: what was ordered, the totals, where it goes, and the link
// to follow it. It's the confirmation the law asks for on a durable medium, so it also repeats
// who the seller is, the right of withdrawal with the model form, and the legal guarantee: a link
// to a web page we can change wouldn't do. The terms themselves are linked, with the version the
// customer accepted.
// TODO(lawyer): confirm this is enough, or whether the terms must go along as a PDF.
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
	// The assembled PC first, as one item with its parts under it; then what ships as it is.
	if o.Assembly {
		ls = append(ls, "Компютър по поръчка, сглобен: "+money.Format(o.BuildCents))
		for _, l := range o.Lines {
			if l.InBuild {
				ls = append(ls, fmt.Sprintf("  %d × %s: %s", l.Quantity, l.Name, money.Format(l.LineCents)))
			}
		}
		ls = append(ls, "  Сглобяване: "+money.Format(o.AssemblyCents))
	}
	for _, l := range o.Lines {
		if !l.InBuild {
			ls = append(ls, fmt.Sprintf("%d × %s: %s", l.Quantity, l.Name, money.Format(l.LineCents)))
		}
	}
	ls = append(ls,
		"",
		"Междинна сума: "+money.Format(o.SubtotalCents),
		"Доставка: "+money.Format(o.DeliveryCents),
	)
	if o.Assembly {
		ls = append(ls, "Сглобяване на компютъра: "+money.Format(o.AssemblyCents))
	}
	ls = append(ls,
		"Общо: "+money.Format(o.TotalCents)+" (с ДДС)",
		"",
	)
	if o.Assembly {
		ls = append(ls,
			"Сглобяваме компютъра от частите в поръчката. Гаранциите на частите се запазват,",
			"а кутиите им пристигат заедно с него.",
			"",
		)
	}
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
		"",
	)
	ls = append(ls, s.orderTerms()...)
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
	}
	if o.Assembly {
		ls = append(ls, "Сглобяваме компютъра и ще ви пишем, когато пратката тръгне.", "")
	} else {
		ls = append(ls, "Подготвяме пратката и ще ви пишем, когато тръгне.", "")
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
	// Replies go to noreply, so the email says where to write.
	ls = append(ls,
		"Въпроси? Пишете ни на "+Seller.Email+" или вижте "+s.siteURL+"/contacts",
		"",
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

// OrderShipped tells the customer the courier has their parcel, and how to follow it.
func (s *Service) OrderShipped(ctx context.Context, o Order) error {
	ls := []string{
		"Здравейте, " + o.Name + ",",
		"",
		fmt.Sprintf("Поръчка № %d пътува към вас: куриерът взе пратката.", o.Number),
		"",
		"Товарителница: " + o.TrackingNumber,
	}
	if o.TrackingURL != "" {
		ls = append(ls, "Проследете пратката: "+o.TrackingURL)
	}
	ls = append(ls, "", "Доставка до: "+o.Address)
	if o.CashOnDelivery {
		ls = append(ls, "При получаване платете на куриера "+money.Format(o.TotalCents)+".")
	}
	ls = append(ls,
		"",
		"Поръчката:",
		"",
		s.orderLink(o),
	)
	return s.sender.Send(ctx, Message{
		To:      o.Email,
		Subject: fmt.Sprintf("Поръчка № %d е изпратена", o.Number),
		Text:    lines(ls...),
	})
}

// OrderDelivered thanks the customer once the courier has handed the parcel over.
func (s *Service) OrderDelivered(ctx context.Context, o Order) error {
	return s.sender.Send(ctx, Message{
		To:      o.Email,
		Subject: fmt.Sprintf("Поръчка № %d е доставена", o.Number),
		Text: lines(
			"Здравейте, "+o.Name+",",
			"",
			fmt.Sprintf("Поръчка № %d е доставена. Благодарим, че пазарувахте в Loadout!", o.Number),
			"",
			"Поръчката:",
			"",
			s.orderLink(o),
		),
	})
}

// orderTerms is what the order email repeats from the terms: the seller, the right of withdrawal
// with the model form (Annex 6 to the Consumer Protection Act), and the legal guarantee.
func (s *Service) orderTerms() []string {
	return []string{
		"Продавач: " + Seller.Name + ", ЕИК " + Seller.ID + ", " + Seller.Address,
		"Връзка с нас: " + Seller.Email + ", " + Seller.Phone,
		"Общи условия, които приехте с поръчката: " + s.siteURL + "/terms",
		"",
		"ПРАВО НА ОТКАЗ",
		"",
		"Имате право да се откажете от договора в 14-дневен срок, без да посочвате причина.",
		"Срокът изтича 14 дни след деня, в който вие или посочено от вас трето лице, различно от",
		"превозвача, получите стоките. Ако поръчката пристигне в няколко пратки, срокът тече от",
		"получаването на последната.",
		"",
		"За да се откажете, ни уведомете ясно преди срокът да изтече: на " + Seller.Email,
		"или с писмо до " + Seller.Address + ". Можете да използвате образеца по-долу, но не сте",
		"длъжни. Повече за връщането: " + s.siteURL + "/returns",
		"",
		"Последици от отказа: връщаме всичко, което сте платили, включително стандартната",
		"доставка, до 14 дни от деня, в който научим за отказа ви: по картата, с която сте",
		"платили, или по банкова сметка, която ни посочите, ако сте платили на куриера. Можем да",
		"задържим връщането, докато получим стоките или доказателство, че сте ги изпратили. Изпратете",
		"стоките до 14 дни след като ни уведомите. Разходите по връщането са за ваша сметка.",
		"Отговаряте само за намалената стойност на стоките, ако сте ги използвали повече,",
		"отколкото е нужно, за да разберете какви са, какви са характеристиките им и как работят.",
		"",
		"ЗАКОНОВА ГАРАНЦИЯ",
		"",
		"Отговаряме за всяко несъответствие на стоките с договора, което се прояви до 2 години",
		"след доставката. Тази отговорност е отделна от търговската гаранция на производителя",
		"и не зависи от нея.",
		"",
		"ОБРАЗЕЦ НА ФОРМУЛЯР ЗА ОТКАЗ",
		"(попълнете го и го изпратете само ако искате да се откажете от договора)",
		"",
		"До " + Seller.Name + ", " + Seller.Address + ", " + Seller.Email + ":",
		"С настоящото уведомявам/уведомяваме*, че се отказвам/отказваме* от сключения от",
		"мен/нас* договор за покупка на следните стоки:",
		"— Поръчани на*/получени на*:",
		"— Номер на поръчката:",
		"— Име на потребителя/ите:",
		"— Адрес на потребителя/ите:",
		"— Подпис на потребителя/ите (само ако формулярът е на хартия):",
		"— Дата:",
		"(*) Ненужното се зачертава.",
	}
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
