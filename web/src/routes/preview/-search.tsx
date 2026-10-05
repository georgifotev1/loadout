import { useNavigate } from "@tanstack/react-router";
import { useState } from "react";
import { type SearchItem, SiteSearch } from "#/components/site/search";
import { m } from "#/i18n";
import type { components } from "#/lib/api/schema.gen";
import { products, summary } from "./-data";

type Category = components["schemas"]["Category"];

const popular = [
	"RTX 5070",
	"Ryzen 7 7800X3D",
	"DDR5 32GB",
	'Монитор 27" 180Hz',
	"Безжична мишка",
];
const defaultCategories: Category[] = [
	"gpu",
	"cpu",
	"peripheral.monitor",
	"ram",
];

// PreviewSearch feeds SiteSearch from the sample data. The real one calls GET /api/search
// (debounced) and takes categories from its facets.
export function PreviewSearch() {
	const [query, setQuery] = useState("");
	const navigate = useNavigate();
	const words = query.toLowerCase().split(/\s+/).filter(Boolean);

	const matches = words.length
		? products.filter((p) => {
				const hay =
					`${p.name} ${p.brand} ${p.sku} ${m.categories[p.category]}`.toLowerCase();
				return words.every((w) => hay.includes(w));
			})
		: [];

	const counts = new Map<Category, number>();
	for (const p of matches)
		counts.set(p.category, (counts.get(p.category) ?? 0) + 1);
	const categories = words.length
		? [...counts].map(([category, count]) => ({ category, count })).slice(0, 3)
		: defaultCategories.map((category) => ({ category }));

	const onSelect = (item: SearchItem) => {
		switch (item.kind) {
			case "product":
				navigate({
					to: "/preview/product",
					search: { slug: item.product.slug },
				});
				break;
			case "category":
				navigate({
					to: "/preview/category",
					search: { category: item.category },
				});
				break;
			case "query":
				setQuery(item.label);
				break;
			case "all":
				navigate({
					to: "/preview/category",
					search: { category: matches[0]?.category ?? "gpu" },
				});
				break;
		}
	};

	return (
		<SiteSearch
			query={query}
			onQueryChange={setQuery}
			products={matches.slice(0, 5).map(summary)}
			categories={categories}
			popular={popular}
			onSelect={onSelect}
		/>
	);
}
