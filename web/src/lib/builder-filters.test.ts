import { describe, expect, it } from "vitest";
import type { Schemas } from "#/lib/api/client";
import {
	applyFilters,
	commonToggles,
	emptyFilters,
	type FilterState,
	facetValues,
	narrowing,
	pickerFilters,
	sortOptions,
} from "./builder-filters";

type Option = Schemas["BuilderOption"];

function option(
	slug: string,
	brand: string,
	cents: number,
	specs: Record<string, unknown>,
	extra: Partial<Option> = {},
): Option {
	return {
		product: {
			slug,
			name: slug,
			brand,
			category: "ram",
			in_stock: true,
			price: { amount_cents: cents, currency: "EUR" },
		},
		specs,
		findings: [],
		...extra,
	};
}

const ram = [
	option("a", "Kingston", 3000, {
		type: "DDR4",
		capacity_gb_total: 16,
		modules: 2,
	}),
	option("b", "Kingston", 6000, {
		type: "DDR5",
		capacity_gb_total: 32,
		modules: 2,
	}),
	option("c", "Corsair", 5000, {
		type: "DDR5",
		capacity_gb_total: 16,
		modules: 1,
	}),
	option("d", "Corsair", 9000, {
		type: "DDR5",
		capacity_gb_total: 64,
		modules: 2,
	}),
];
const defs = pickerFilters("ram");
const state = (s: Partial<FilterState>): FilterState => ({
	...emptyFilters,
	...s,
});
const slugs = (os: Option[]) => os.map((o) => o.product.slug);

describe("applyFilters", () => {
	it("ORs values within a facet and ANDs facets", () => {
		expect(
			slugs(
				applyFilters(
					ram,
					defs,
					state({ facets: { capacity: ["16 GB", "64 GB"] } }),
				),
			),
		).toEqual(["a", "c", "d"]);
		expect(
			slugs(
				applyFilters(
					ram,
					defs,
					state({
						facets: { capacity: ["16 GB", "64 GB"], brand: ["Corsair"] },
					}),
				),
			),
		).toEqual(["c", "d"]);
	});
});

describe("facetValues", () => {
	it("orders values by size and counts them against the other filters", () => {
		const capacity = defs.facets.find((f) => f.id === "capacity");
		if (!capacity) throw new Error("no capacity facet");
		expect(
			facetValues(
				ram,
				defs,
				state({ facets: { brand: ["Corsair"], capacity: ["32 GB"] } }),
				capacity,
			),
		).toEqual([
			{ label: "16 GB", count: 1 },
			// No Corsair kit is 32 GB, but a ticked value stays listed so it can be unticked.
			{ label: "32 GB", count: 0 },
			{ label: "64 GB", count: 1 },
		]);
	});
});

describe("narrowing", () => {
	it("hides facets with one value and toggles that change nothing", () => {
		const ddr5 = ram.slice(1);
		const shown = narrowing(ddr5, { ...defs, toggles: commonToggles() });
		// All DDR5, so no memory type facet.
		expect(shown.facets.map((f) => f.id)).toEqual([
			"capacity",
			"modules",
			"brand",
		]);
		expect(shown.toggles).toEqual([]);
	});
});

describe("PSU filters", () => {
	it("offer the recommended wattage once there's an estimate", () => {
		expect(pickerFilters("psu").toggles).toEqual([]);
		const enough = pickerFilters("psu", 650).toggles[0];
		expect(enough?.test(option("p", "X", 1, { wattage_w: 650 }))).toBe(true);
		expect(enough?.test(option("p", "X", 1, { wattage_w: 550 }))).toBe(false);
	});
});

describe("sortOptions", () => {
	it("sorts by a spec, with unknown values last", () => {
		const capacity = defs.sorts.find((s) => s.id === "capacity");
		expect(
			slugs(sortOptions([...ram, option("e", "X", 1, {})], capacity)),
		).toEqual(["d", "b", "a", "c", "e"]);
	});
});
