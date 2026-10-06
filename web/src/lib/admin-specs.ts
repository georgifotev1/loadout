import { type Category, componentCategories } from "#/lib/categories";

export const reviewPageSize = 50;

// SpecsSearch is the review queue's URL state, also carried to the editor so "back" and "next"
// stay in the same category.
export type SpecsSearch = {
	category?: Category;
	page?: number;
};

// parseSpecsSearch reads SpecsSearch from untrusted URL params; anything malformed is dropped.
// Only builder parts have canonical specs.
export function parseSpecsSearch(s: Record<string, unknown>): SpecsSearch {
	const out: SpecsSearch = {};
	if (componentCategories.includes(s.category as Category)) {
		out.category = s.category as Category;
	}
	const page = typeof s.page === "string" ? Number(s.page) : s.page;
	if (typeof page === "number" && Number.isSafeInteger(page) && page >= 2) {
		out.page = page;
	}
	return out;
}

/** The label for a canonical field, falling back to its name. */
export function fieldLabel(
	labels: Record<string, string | undefined>,
	name: string,
): string {
	return labels[name] ?? name;
}
