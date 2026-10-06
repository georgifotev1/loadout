import type { SearchParams } from "#/lib/api/products";
import { type Category, isCategory } from "#/lib/categories";

export const pageSize = 24;

// ListingSearch is the URL state of a category or search page: what the shopper filtered on,
// so a filtered listing can be shared and reloaded. Defaults are left out to keep URLs short.
export type ListingSearch = {
	q?: string;
	category?: Category;
	brand?: string[];
	price_min?: number;
	price_max?: number;
	sort?: "price_asc" | "price_desc"; // absent: relevance
	page?: number;
};

// parseListingSearch reads ListingSearch from untrusted URL params. Anything malformed is
// dropped rather than an error: an old or hand-edited link should still show a listing.
export function parseListingSearch(s: Record<string, unknown>): ListingSearch {
	const out: ListingSearch = {};
	if (typeof s.q === "string" && s.q.trim()) out.q = s.q.trim().slice(0, 200);
	if (isCategory(s.category)) out.category = s.category;
	const brands = (Array.isArray(s.brand) ? s.brand : [s.brand]).filter(
		(b): b is string => typeof b === "string" && b !== "",
	);
	if (brands.length) out.brand = [...new Set(brands)].slice(0, 20);
	const min = wholeNumber(s.price_min, 0);
	const max = wholeNumber(s.price_max, 1);
	if (min !== undefined) out.price_min = min;
	if (max !== undefined && max > (min ?? 0)) out.price_max = max;
	if (s.sort === "price_asc" || s.sort === "price_desc") out.sort = s.sort;
	const page = wholeNumber(s.page, 2); // page 1 is the default

	if (page !== undefined) out.page = page;
	return out;
}

// wholeNumber reads an integer URL param of at least min; anything else is undefined.
export function wholeNumber(v: unknown, min: number): number | undefined {
	const n = typeof v === "string" ? Number(v) : v;
	return typeof n === "number" && Number.isSafeInteger(n) && n >= min
		? n
		: undefined;
}

// toSearchParams is the GET /api/search query for a listing.
export function toSearchParams(s: ListingSearch): SearchParams {
	return {
		q: s.q,
		category: s.category,
		brand: s.brand,
		price_min: s.price_min,
		price_max: s.price_max,
		sort: s.sort,
		page: s.page,
		page_size: pageSize,
	};
}

// pageNumbers is the pagination to show: the first, last and current page with its
// neighbours, and null for each gap, e.g. [1, null, 4, 5, 6, null, 12].
export function pageNumbers(current: number, last: number): (number | null)[] {
	const out: (number | null)[] = [];
	for (let n = 1; n <= last; n++) {
		if (n === 1 || n === last || Math.abs(n - current) <= 1) out.push(n);
		else if (out.at(-1) !== null) out.push(null);
	}
	return out;
}
