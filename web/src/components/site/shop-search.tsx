import { keepPreviousData, useQuery } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import { useState } from "react";
import { useDebouncedValue } from "#/hooks/use-debounced-value";
import { searchQuery } from "#/lib/api/products";
import type { Category } from "#/lib/categories";
import { type SearchItem, SiteSearch } from "./search";

// Suggested while the box is empty.
const suggestedCategories: Category[] = [
	"gpu",
	"cpu",
	"peripheral.monitor",
	"ram",
];

// ShopSearch feeds the header search from GET /api/search: the top products for what the
// shopper typed, and the categories with the most matches.
export function ShopSearch() {
	const [query, setQuery] = useState("");
	const navigate = useNavigate();
	const q = useDebouncedValue(query.trim(), 200);

	const { data, isFetching, isError } = useQuery({
		...searchQuery({ q, page_size: 5 }),
		enabled: q !== "",
		placeholderData: keepPreviousData,
	});
	const typed = query.trim();
	const results = typed && data ? data : undefined;
	// The previous results stay up while the next ones load (or the typing settles).
	const status = !typed
		? undefined
		: isError && !data
			? "error"
			: typed !== q || isFetching
				? "searching"
				: undefined;

	const onSelect = (item: SearchItem) => {
		switch (item.kind) {
			case "product":
				navigate({ to: "/p/$slug", params: { slug: item.product.slug } });
				break;
			case "category":
				if (query.trim())
					navigate({
						to: "/search",
						search: { q: query.trim(), category: item.category },
					});
				else
					navigate({
						to: "/c/$category",
						params: { category: item.category },
					});
				break;
			case "query":
				setQuery(item.label);
				break;
			case "all":
				// The item's label is the "all results" text; the query is what was typed.
				if (query.trim())
					navigate({ to: "/search", search: { q: query.trim() } });
				break;
		}
	};

	return (
		<SiteSearch
			query={query}
			onQueryChange={setQuery}
			products={results?.products ?? []}
			categories={
				results
					? results.facets.categories.slice(0, 3).map((c) => ({
							category: c.value,
							count: c.count,
						}))
					: typed
						? []
						: suggestedCategories.map((category) => ({ category }))
			}
			status={status}
			onSelect={onSelect}
		/>
	);
}
