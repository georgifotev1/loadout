import type { components } from "#/lib/api/schema.gen";
import { type PreviewProduct, products } from "./-products";

export type { PreviewProduct };
export { products };

type ProductSummary = components["schemas"]["ProductSummary"];

export function summary(p: PreviewProduct): ProductSummary {
	return {
		slug: p.slug,
		name: p.name,
		brand: p.brand,
		category: p.category,
		image: p.images[0],
		price: p.price,
		in_stock: p.in_stock,
	};
}

// Which supplier spec rows to show on product cards, per category. The API doesn't return
// these yet; it would come from the canonical specs as a short "highlights" list.
const highlightKeys: Record<string, string[]> = {
	cpu: ["Сокет", "Физически ядра", "Turbo Boost честота"],
	gpu: ["Капацитет на паметта", "Тип на паметта", "Препоръчително захранване"],
	motherboard: ["Поддържана памет", "Брой слотове памет", "М2 слот"],
	ram: ["Капацитет", "Тип памет", "Честота (MHz)"],
	psu: ["Мощност", "Сертификати", "Модулен захранващ блок"],
	case: ["Вид кутия", "Формат дънна платка"],
	cooler: ["Тип охлаждане", "Съвместими сокети"],
	storage: ["Капацитет", "Форм фактор", "Скорост при четене (MB/s)"],
	"peripheral.monitor": ["Размер на дисплея", "Време за реакция"],
	"peripheral.mouse": ["Сензор", "Свързване"],
	"peripheral.keyboard": ["Тип клавиатура", "Свързване"],
	"peripheral.headset": ["Интерфейс", "Микрофон"],
};

// keySpecs is the category's most important spec rows, as [label, value].
export function keySpecs(p: PreviewProduct): [string, string][] {
	return (highlightKeys[p.category] ?? []).flatMap((k): [string, string][] => {
		const v = p.attributes[k];
		return v && v !== "-" ? [[k, v]] : [];
	});
}

export function highlights(p: PreviewProduct): string[] {
	return keySpecs(p)
		.map(([, v]) => v)
		.filter((v) => v.length <= 24);
}

export function bySlug(slug: string | undefined) {
	return products.find((p) => p.slug === slug);
}

// The cart shown on the cart, checkout and confirmation previews.
export const cartLines = [
	{ product: bySlug("amd-am5-r7-7800x3d-box"), qty: 1 },
	{ product: bySlug("pc-vc-rx9070gre-12g-e"), qty: 1 },
	{ product: bySlug("logite-mouse-g502-hero-bk"), qty: 2 },
].filter((l): l is { product: PreviewProduct; qty: number } => !!l.product);
