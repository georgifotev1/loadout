import { type Category, isCategory } from "#/lib/categories";
import { wholeNumber } from "#/lib/listing";

export const productsPageSize = 50;

// ProductsSearch is the admin product list's URL state. Defaults are left out.
export type ProductsSearch = {
	q?: string;
	category?: Category;
	status?: "active" | "inactive";
	page?: number;
};

// parseProductsSearch reads ProductsSearch from untrusted URL params; anything malformed is
// dropped.
export function parseProductsSearch(
	s: Record<string, unknown>,
): ProductsSearch {
	const out: ProductsSearch = {};
	if (typeof s.q === "string" && s.q.trim()) out.q = s.q.trim().slice(0, 200);
	if (isCategory(s.category)) out.category = s.category;
	if (s.status === "active" || s.status === "inactive") out.status = s.status;
	const page = wholeNumber(s.page, 2);
	if (page !== undefined) out.page = page;
	return out;
}
