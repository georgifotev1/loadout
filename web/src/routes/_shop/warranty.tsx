import { createFileRoute, Link } from "@tanstack/react-router";
import { LegalPage, legalHead } from "#/components/legal/legal-page";
import { company, termsEffective } from "#/lib/company";

// /warranty: the legal guarantee of conformity, the manufacturers' commercial warranty, and how
// to make a claim. The ЗЗП asks the seller to keep a register of the claims it receives.
// TODO(lawyer): the terms for repair and replacement, and what the claim must contain.
export const Route = createFileRoute("/_shop/warranty")({
	head: () =>
		legalHead(
			"warranty",
			"Законова гаранция от 2 години, търговска гаранция на производителя и как да предявите рекламация.",
		),
	component: WarrantyPage,
});

function WarrantyPage() {
	return (
		<LegalPage page="warranty" effective={termsEffective}>
			<h2>Законова гаранция</h2>
			<p>
				Отговаряме за всяко несъответствие на стоките с договора, което се
				прояви до 2 години след доставката: ако стоката не работи, не е каквато
				е описана или не става за обичайната си употреба. Тази отговорност е по
				закон и не зависи от търговската гаранция.
			</p>
			<p>При несъответствие имате право:</p>
			<ol>
				<li>
					стоката да бъде поправена или заменена, без разходи за вас, в разумен
					срок;
				</li>
				<li>
					ако поправката или замяната е невъзможна, не стане в разумен срок или
					несъответствието е съществено — на намаление на цената или да
					развалите договора и да получите парите си обратно.
				</li>
			</ol>

			<h2>Търговска гаранция</h2>
			<p>
				Повечето части имат и гаранция от производителя или вносителя, често
				по-дълга от 2 години. Срокът и условията ѝ са в гаранционната карта.
				Търговската гаранция не ограничава законовата: можете да ползвате която
				и да е от двете. При сглобен от нас компютър гаранцията на всяка част се
				запазва.
			</p>

			<h2>Как да предявите рекламация</h2>
			<ol>
				<li>
					Пишете ни на {company.email} с номера на поръчката, кой продукт и
					какъв е проблемът. Снимка или видео помагат.
				</li>
				<li>
					Ще ви кажем как да изпратите продукта. Когато нещо е по наша вина,
					разходите по изпращането са за наша сметка.
				</li>
				<li>
					Записваме рекламацията и ви изпращаме номера ѝ. Ще ви пишем какво
					установим и какво предлагаме.
				</li>
			</ol>
			<p>
				Ако искате просто да върнете продукт, който работи, вижте{" "}
				<Link to="/returns">Връщане и право на отказ</Link>.
			</p>
		</LegalPage>
	);
}
