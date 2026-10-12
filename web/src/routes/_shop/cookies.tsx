import { createFileRoute, Link } from "@tanstack/react-router";
import { LegalPage, legalHead } from "#/components/legal/legal-page";
import { termsEffective } from "#/lib/company";

// /cookies: the cookie policy. Every cookie here is strictly necessary for something the visitor
// asked for (the cart, logging in, the admin's own sidebar), which ePrivacy art. 5(3) and the
// Electronic Communications Act (ЗЕС) exempt from consent, so there's no consent banner. Adding
// analytics, ads or anything third party needs a banner with "reject" as easy as "accept", before
// the cookie is set. Keep this list in step with the cookies the API and the web app set.
export const Route = createFileRoute("/_shop/cookies")({
	head: () =>
		legalHead(
			"cookies",
			"Кои бисквитки ползва Loadout: само необходимите за количката и входа, без проследяване и реклами.",
		),
	component: CookiesPage,
});

const cookies = [
	{
		name: "session",
		purpose: "Пази входа ви в профила.",
		lifetime: "До 30 дни (12 часа за администратори) или до изход",
	},
	{
		name: "cart",
		purpose: "Пази количката ви, когато не сте влезли в профил.",
		lifetime: "30 дни от последната промяна",
	},
	{
		name: "sidebar_state",
		purpose:
			"Само в администрацията: дали страничната лента е отворена или свита.",
		lifetime: "7 дни",
	},
];

function CookiesPage() {
	return (
		<LegalPage page="cookies" effective={termsEffective}>
			<p>
				Бисквитките са малки файлове, които сайтът записва в браузъра ви.
				Loadout ползва само бисквитки, без които сайтът не може да изпълни това,
				което сте поискали: да помни количката ви и да ви държи влезли в
				профила. За тях законът не изисква съгласие, затова не ви питаме.
			</p>
			<p>
				Не ползваме бисквитки за анализ, реклама или проследяване и няма
				бисквитки от други сайтове. Ако някога добавим такива, ще ви питаме,
				преди да ги запишем, и ще можете да откажете също толкова лесно, колкото
				да приемете.
			</p>

			<h2>Бисквитките на сайта</h2>
			<table>
				<thead>
					<tr>
						<th>Име</th>
						<th>За какво</th>
						<th>Срок</th>
					</tr>
				</thead>
				<tbody>
					{cookies.map((c) => (
						<tr key={c.name}>
							<td>
								<code>{c.name}</code>
							</td>
							<td>{c.purpose}</td>
							<td>{c.lifetime}</td>
						</tr>
					))}
				</tbody>
			</table>
			<p>
				В работната версия на сайта имената на първите две започват с{" "}
				<code>__Host-</code>, знак, че браузърът ги изпраща само на нашия сайт и
				само по защитена връзка. Скриптовете в страницата не могат да ги четат.
			</p>

			<h2>Плащане с карта</h2>
			<p>
				Когато плащате с карта, ви пренасочваме към страницата на Stripe. Тя
				ползва свои бисквитки, например срещу измами, по{" "}
				<a href="https://stripe.com/legal/cookies-policy">
					политиката на Stripe
				</a>
				.
			</p>

			<h2>Как да ги изтриете</h2>
			<p>
				Можете да изтриете бисквитките от настройките на браузъра си. Тогава
				излизате от профила, а количката на гост се изпразва. Как обработваме
				данните ви, е описано в{" "}
				<Link to="/privacy">Политиката за поверителност</Link>.
			</p>
		</LegalPage>
	);
}
