import { createFileRoute } from "@tanstack/react-router";
import { LegalPage, legalHead } from "#/components/legal/legal-page";
import { company } from "#/lib/company";

// /contacts: how to reach us and who we are, as the E-Commerce Act (ЗЕТ) asks a shop to show.
export const Route = createFileRoute("/_shop/contacts")({
	head: () =>
		legalHead(
			"contacts",
			"Как да се свържете с Loadout: имейл, телефон, адрес и данни на фирмата.",
		),
	component: ContactsPage,
});

function ContactsPage() {
	return (
		<LegalPage page="contacts">
			<p>
				Отговаряме на имейли в работно време, обикновено до 1 работен ден.
				Когато пишете за поръчка, посочете номера ѝ.
			</p>
			<ul>
				<li>
					Имейл: <a href={`mailto:${company.email}`}>{company.email}</a>
				</li>
				<li>Телефон: {company.phone}</li>
				<li>Работно време: {company.hours}</li>
			</ul>
			<p>
				Нямаме физически магазин: поръчките се правят само онлайн и се доставят
				с куриер.
			</p>

			<h2>Данни на фирмата</h2>
			<ul>
				<li>{company.name}</li>
				<li>ЕИК: {company.id}</li>
				<li>ДДС номер: {company.vatId}</li>
				<li>Седалище и адрес на управление: {company.address}</li>
			</ul>

			<h2>Надзорни органи</h2>
			<ul>
				<li>
					Комисия за защита на потребителите: гр. София 1000, пл. „Славейков“ №
					4А, тел. 0700 111 22, <a href="https://kzp.bg">kzp.bg</a>
				</li>
				<li>
					Комисия за защита на личните данни: гр. София 1592, бул. „Проф. Цветан
					Лазаров“ № 2, <a href="https://www.cpdp.bg">cpdp.bg</a>
				</li>
			</ul>
		</LegalPage>
	);
}
