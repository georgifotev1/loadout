import { describe, expect, it } from "vitest";
import { parseProductsSearch } from "./admin-products";

describe("parseProductsSearch", () => {
	it("keeps valid params", () => {
		expect(
			parseProductsSearch({
				q: "  ryzen ",
				category: "cpu",
				status: "inactive",
				page: "3",
			}),
		).toEqual({ q: "ryzen", category: "cpu", status: "inactive", page: 3 });
	});

	it("drops malformed and default params", () => {
		expect(
			parseProductsSearch({
				q: "   ",
				category: "laptop",
				status: "hidden",
				page: "1",
			}),
		).toEqual({});
	});
});
