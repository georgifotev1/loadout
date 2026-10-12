import { createFileRoute, Link } from "@tanstack/react-router";
import { LegalPage, legalHead } from "#/components/legal/legal-page";
import { company, termsEffective } from "#/lib/company";

// /returns: the right of withdrawal (ЗЗП) and the model withdrawal form (Annex 6). The order email
// (internal/notify, orderTerms) repeats the same, so change both together.
// TODO(lawyer): whether an assembled PC is "made to the consumer's specifications" and so exempt
// (PLAN.md §9). Until they answer, the page grants the right for everything, which is the safe side.
// TODO(phase-7): the online withdrawal function (Directive 2023/2673, from 19 June 2026): a
// "withdraw here" button and form, with an acknowledgement by email.
export const Route = createFileRoute("/_shop/returns")({
	head: () =>
		legalHead(
			"returns",
			"Право на отказ от поръчка в 14-дневен срок: как да се откажете, как да върнете стоката и кога връщаме парите.",
		),
	component: ReturnsPage,
});

function ReturnsPage() {
	return (
		<LegalPage page="returns" effective={termsEffective}>
			<h2>Право на отказ</h2>
			<p>
				Имате право да се откажете от договора в 14-дневен срок, без да
				посочвате причина. Срокът изтича 14 дни след деня, в който вие или
				посочено от вас трето лице, различно от превозвача, получите стоките.
				Ако поръчката пристигне в няколко пратки, срокът тече от получаването на
				последната.
			</p>

			<h2>Как да се откажете</h2>
			<p>
				Уведомете ни ясно, преди срокът да изтече: на {company.email} или с
				писмо до {company.address}. Посочете номера на поръчката. Можете да
				използвате <a href="#form">образеца по-долу</a>, но не сте длъжни.
				Достатъчно е уведомлението да е изпратено преди края на срока.
			</p>

			<h2>Как да върнете стоките</h2>
			<ul>
				<li>
					Изпратете ги до 14 дни след като ни уведомите. Ще ви пишем адреса, на
					който да ги изпратите.
				</li>
				<li>Разходите по връщането са за ваша сметка.</li>
				<li>
					Можете да разгледате и пробвате стоките, както бихте го направили в
					магазин. Отговаряте за намалената им стойност само ако сте ги
					използвали повече от това.
				</li>
				<li>
					Изпратете ги добре опаковани, за предпочитане в оригиналната опаковка,
					с всички аксесоари.
				</li>
			</ul>

			<h2>Кога и как връщаме парите</h2>
			<p>
				Връщаме всичко, което сте платили, включително стандартната доставка, до
				14 дни от деня, в който научим за отказа ви. При плащане с карта връщаме
				по картата, а при наложен платеж — по банкова сметка, която ни посочите.
				Можем да задържим връщането, докато получим стоките или доказателство,
				че сте ги изпратили, което от двете е по-рано.
			</p>

			<h2>Кога правото на отказ не важи</h2>
			<p>
				Законът изключва някои случаи, например запечатан компютърен софтуер,
				разпечатан след доставката.
			</p>

			<h2>Стоки с дефект</h2>
			<p>
				Ако стоката не работи или не отговаря на описанието, това не е отказ, а
				рекламация: вижте <Link to="/warranty">Гаранция и рекламации</Link>.
			</p>

			<h2 id="form">Образец на формуляр за отказ</h2>
			<p className="text-muted-foreground">
				Попълнете го и го изпратете само ако искате да се откажете от договора.
			</p>
			<blockquote className="not-italic">
				<p>
					До {company.name}, {company.address}, {company.email}:
				</p>
				<p>
					С настоящото уведомявам/уведомяваме*, че се отказвам/отказваме* от
					сключения от мен/нас* договор за покупка на следните стоки:
				</p>
				<ul>
					<li>Поръчани на*/получени на*:</li>
					<li>Номер на поръчката:</li>
					<li>Име на потребителя/ите:</li>
					<li>Адрес на потребителя/ите:</li>
					<li>Подпис на потребителя/ите (само ако формулярът е на хартия):</li>
					<li>Дата:</li>
				</ul>
				<p>(*) Ненужното се зачертава.</p>
			</blockquote>
		</LegalPage>
	);
}
