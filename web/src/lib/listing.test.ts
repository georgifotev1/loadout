import { describe, expect, it } from "vitest";
import { pageNumbers, parseListingSearch, toSearchParams } from "./listing";

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

describe("spec filters", () => {
	it("keeps well-formed field:value pairs", () => {
		expect(
			parseListingSearch({
				spec: ["socket:AM5", "socket:AM5", "vram_gb:12", "nope", "Bad:1", 7],
			}),
		).toEqual({ spec: ["socket:AM5", "vram_gb:12"] });
		expect(parseListingSearch({ spec: "chipset:GeForce RTX 5070" })).toEqual({
			spec: ["chipset:GeForce RTX 5070"],
		});
	});

	it("sends only the category's facet fields", () => {
		const spec = ["chipset:GeForce RTX 5070", "socket:AM5"];
		expect(toSearchParams({ category: "gpu", spec }).spec).toEqual([
			"chipset:GeForce RTX 5070",
		]);
		expect(toSearchParams({ spec }).spec).toBeUndefined();
		expect(
			toSearchParams({ category: "peripheral.mouse", spec }).spec,
		).toBeUndefined();
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
