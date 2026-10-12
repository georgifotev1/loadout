import { createFileRoute, Link } from "@tanstack/react-router";
import { LegalPage, legalHead } from "#/components/legal/legal-page";
import { formatPrice, m } from "#/i18n";

// /delivery: how, where and for how much we deliver.
export const Route = createFileRoute("/_shop/delivery")({
	head: () =>
		legalHead(
			"delivery",
			"Доставка в България със Спиди: до офис, автомат или адрес, цени и срокове.",
		),
	component: DeliveryPage,
});

// The prices checkout charges, VAT included: keep them in step with internal/orders/delivery.go.
const methods = [
	{
		name: m.checkout.methods.office,
		cents: 3_99,
		detail: "Взимате пратката от избран офис.",
	},
	{
		name: m.checkout.methods.apt,
		cents: 3_49,
		detail:
			"Взимате пратката от автомат с код. Сглобен компютър не се побира в автомат.",
	},
	{
		name: m.checkout.methods.address,
		cents: 5_99,
		detail: "Куриерът носи пратката до посочения от вас адрес.",
	},
];

function DeliveryPage() {
	return (
		<LegalPage page="delivery">
			<p>
				Доставяме в цяла България с куриерска фирма Спиди. Засега не доставяме в
				други държави.
			</p>

			<h2>Начини и цени</h2>
			<table>
				<thead>
					<tr>
						<th>Начин</th>
						<th>Цена с ДДС</th>
					</tr>
				</thead>
				<tbody>
					{methods.map((d) => (
						<tr key={d.name}>
							<td>
								<strong>{d.name}</strong>
								<br />
								{d.detail}
							</td>
							<td className="whitespace-nowrap">{formatPrice(d.cents)}</td>
						</tr>
					))}
				</tbody>
			</table>
			<p>
				Цената е една, без значение колко продукта поръчвате, и се вижда при
				поръчката, преди да я потвърдите.
			</p>

			<h2>Срокове</h2>
			<ol>
				<li>
					Потвърждаваме наличността при доставчика ни до 1 работен ден след
					поръчката.
				</li>
				<li>
					Изпращаме пратката и ви пишем номера на товарителницата, за да я
					проследите. Сглобяването на компютър може да добави няколко работни
					дни.
				</li>
				<li>Куриерът обикновено доставя до 2 работни дни.</li>
			</ol>

			<h2>Плащане при доставка</h2>
			<p>
				При наложен платеж плащате сумата от поръчката на куриера, когато
				получите пратката. Сумата е в имейла за изпращане.
			</p>

			<h2>При получаване</h2>
			<p>
				Огледайте пратката пред куриера. Ако опаковката е видимо повредена,
				опишете го в товарителницата или откажете да я приемете и ни пишете.
				Отказът от договора е описан на страницата{" "}
				<Link to="/returns">Връщане и право на отказ</Link>.
			</p>
		</LegalPage>
	);
}
