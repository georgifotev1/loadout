import { describe, expect, it } from "vitest";
import { pageNumbers, parseListingSearch } from "./listing";

describe("parseListingSearch", () => {
	it("keeps valid filters", () => {
		expect(
			parseListingSearch({
				q: "  rtx 5070 ",
				category: "gpu",
				brand: ["ASUS", "MSI"],
				price_min: 25000,
				price_max: "50000",
				sort: "price_asc",
				page: 3,
			}),
		).toEqual({
			q: "rtx 5070",
			category: "gpu",
			brand: ["ASUS", "MSI"],
			price_min: 25000,
			price_max: 50000,
			sort: "price_asc",
			page: 3,
		});
	});

	it("accepts a single brand and drops duplicates", () => {
		expect(parseListingSearch({ brand: "AMD" })).toEqual({ brand: ["AMD"] });
		expect(parseListingSearch({ brand: ["AMD", "AMD", ""] })).toEqual({
			brand: ["AMD"],
		});
	});

	it("drops what is malformed or the default", () => {
		expect(
			parseListingSearch({
				q: " ",
				category: "spaceship",
				price_min: -1,
				price_max: 1.5,
				sort: "relevance",
				page: 1,
			}),
		).toEqual({});
		expect(parseListingSearch({ page: "abc", brand: 7 })).toEqual({});
	});

	it("drops a price limit at or below the minimum", () => {
		expect(parseListingSearch({ price_min: 5000, price_max: 5000 })).toEqual({
			price_min: 5000,
		});
	});
});

describe("pageNumbers", () => {
	it("lists every page when there are few", () => {
		expect(pageNumbers(1, 1)).toEqual([1]);
		expect(pageNumbers(2, 3)).toEqual([1, 2, 3]);
	});

	it("collapses the gaps", () => {
		expect(pageNumbers(1, 12)).toEqual([1, 2, null, 12]);
		expect(pageNumbers(5, 12)).toEqual([1, null, 4, 5, 6, null, 12]);
		expect(pageNumbers(12, 12)).toEqual([1, null, 11, 12]);
	});
});
