import { m } from "#/i18n";
import type { Schemas } from "#/lib/api/client";
import type { Slot } from "#/lib/builder";
import {
	byRank,
	type SpecValue,
	specFacetLabel,
	specValue,
	specValues,
} from "#/lib/spec-facets";

type Option = Schemas["BuilderOption"];

// The part picker's filters, worked out from the options that fit, as PC builders and shops
// usually do it: a few spec facets per part type (socket, memory type, capacity, ...) where
// values in one facet widen the list and separate facets narrow it, yes/no toggles, and a sort.

export type Facet = {
	id: string;
	label: string;
	values: (o: Option) => SpecValue[];
};

export type Toggle = {
	id: string;
	label: string;
	test: (o: Option) => boolean;
};

export type Sort = {
	id: string;
	label: string;
	compare: (a: Option, b: Option) => number;
};

export type PickerFilters = {
	facets: Facet[];
	toggles: Toggle[];
	sorts: Sort[];
};

export type FilterState = {
	facets: Record<string, string[]>;
	toggles: string[];
	sort: string;
};

export const emptyFilters: FilterState = {
	facets: {},
	toggles: [],
	sort: "price",
};

const num = (o: Option, k: string) =>
	typeof o.specs[k] === "number" ? (o.specs[k] as number) : undefined;
const price = (o: Option) => o.product.price.amount_cents;

// spec is a facet over one spec field, read the same way as the shop listings' spec facets.
function spec(slot: Slot, id: string, field: string): Facet {
	return {
		id,
		label: specFacetLabel(slot, field),
		values: (o) =>
			specValues(o.specs[field]).map((v) => specValue(slot, field, v)),
	};
}

// byDesc sorts by a spec, largest first; parts without it go last.
function byDesc(id: string, label: string, key: string): Sort {
	return {
		id,
		label,
		compare: (a, b) => (num(b, key) ?? -1) - (num(a, key) ?? -1),
	};
}

function byAsc(id: string, label: string, key: string): Sort {
	return {
		id,
		label,
		compare: (a, b) =>
			(num(a, key) ?? Number.POSITIVE_INFINITY) -
			(num(b, key) ?? Number.POSITIVE_INFINITY),
	};
}

// pickerFilters are the filters for slot. recommendedPsuW enables the PSU's "enough power"
// toggle once the build has a power estimate.
export function pickerFilters(slot: Slot, recommendedPsuW = 0): PickerFilters {
	const t = m.builder.filters;
	const brand: Facet = {
		id: "brand",
		label: t.brand,
		values: (o) => [{ label: o.product.brand, rank: 0 }],
	};
	const facet = (id: string, field: string) => spec(slot, id, field);
	const priceAsc: Sort = {
		id: "price",
		label: t.sortPriceAsc,
		compare: (a, b) => price(a) - price(b),
	};
	const priceDesc: Sort = {
		id: "price_desc",
		label: t.sortPriceDesc,
		compare: (a, b) => price(b) - price(a),
	};
	const base = [priceAsc, priceDesc];

	switch (slot) {
		case "cpu":
			return {
				facets: [brand, facet("socket", "socket")],
				toggles: [
					{ id: "igpu", label: t.igpu, test: (o) => o.specs.igpu === true },
					{
						id: "cooler",
						label: t.boxCooler,
						test: (o) => o.specs.cooler_included === true,
					},
				],
				sorts: [...base, byAsc("tdp", t.sortTdp, "tdp_w")],
			};
		case "motherboard":
			return {
				facets: [
					brand,
					facet("form", "form_factor"),
					facet("memory", "memory_type"),
					facet("chipset", "chipset"),
				],
				toggles: [],
				sorts: [...base, byDesc("m2", t.sortM2, "m2_slots")],
			};
		case "ram":
			return {
				facets: [
					facet("type", "type"),
					facet("capacity", "capacity_gb_total"),
					facet("modules", "modules"),
					facet("speed", "speed_mhz"),
					brand,
				],
				toggles: [],
				sorts: [
					...base,
					byDesc("capacity", t.sortCapacity, "capacity_gb_total"),
					byDesc("speed", t.sortSpeed, "speed_mhz"),
				],
			};
		case "cooler":
			return {
				facets: [
					facet("type", "type"),
					facet("radiator", "radiator_mm"),
					brand,
				],
				toggles: [],
				sorts: [...base, byAsc("height", t.sortHeight, "height_mm")],
			};
		case "gpu":
			return {
				facets: [facet("chipset", "chipset"), facet("vram", "vram_gb"), brand],
				toggles: [],
				sorts: [
					...base,
					byAsc("length", t.sortLength, "length_mm"),
					byAsc("tdp", t.sortTdp, "tdp_w"),
				],
			};
		case "storage":
			return {
				facets: [
					facet("interface", "interface"),
					facet("capacity", "capacity_gb"),
					facet("form", "form_factor"),
					brand,
				],
				toggles: [],
				sorts: [
					...base,
					byDesc("capacity", t.sortCapacity, "capacity_gb"),
					{
						id: "per_gb",
						label: t.sortPerGb,
						compare: (a, b) =>
							price(a) / (num(a, "capacity_gb") ?? 1) -
							price(b) / (num(b, "capacity_gb") ?? 1),
					},
				],
			};
		case "psu":
			return {
				facets: [
					facet("wattage", "wattage_w"),
					facet("form", "form_factor"),
					brand,
				],
				toggles:
					recommendedPsuW > 0
						? [
								{
									id: "enough",
									label: t.enoughPower(recommendedPsuW),
									test: (o) => (num(o, "wattage_w") ?? 0) >= recommendedPsuW,
								},
							]
						: [],
				sorts: [...base, byDesc("wattage", t.sortWattage, "wattage_w")],
			};
		case "case":
			return {
				facets: [
					facet("board", "form_factors_supported"),
					facet("psu", "psu_form_factor"),
					brand,
				],
				toggles: [],
				sorts: [...base, byDesc("gpu", t.sortGpuRoom, "max_gpu_length_mm")],
			};
	}
}

// Toggles every picker has: shown only when they'd change the list.
export function commonToggles(): Toggle[] {
	const t = m.builder.filters;
	return [
		{ id: "in_stock", label: t.inStock, test: (o) => o.product.in_stock },
		{
			id: "no_notes",
			label: t.noNotes,
			test: (o) => !o.findings.some((f) => f.severity !== "info"),
		},
	];
}

// applyFilters keeps the options that match every active facet (any of its values) and toggle.
// skipFacet leaves one facet out, to count what picking another of its values would show.
export function applyFilters(
	options: Option[],
	defs: { facets: Facet[]; toggles: Toggle[] },
	state: FilterState,
	skipFacet?: string,
): Option[] {
	return options.filter((o) => {
		for (const f of defs.facets) {
			const want = state.facets[f.id];
			if (f.id === skipFacet || !want?.length) continue;
			if (!f.values(o).some((v) => want.includes(v.label))) return false;
		}
		for (const t of defs.toggles) {
			if (state.toggles.includes(t.id) && !t.test(o)) return false;
		}
		return true;
	});
}

export type FacetValueCount = { label: string; count: number };

// facetValues lists a facet's values in order, each with how many options it would show given
// the other active filters (the usual faceted-search count). Values of active filters are kept
// even at 0, so they can still be unticked.
export function facetValues(
	options: Option[],
	defs: { facets: Facet[]; toggles: Toggle[] },
	state: FilterState,
	facet: Facet,
): FacetValueCount[] {
	const ranks = new Map<string, number>();
	for (const o of options) {
		for (const v of facet.values(o)) ranks.set(v.label, v.rank);
	}
	const counts = new Map<string, number>();
	for (const o of applyFilters(options, defs, state, facet.id)) {
		for (const v of facet.values(o)) {
			counts.set(v.label, (counts.get(v.label) ?? 0) + 1);
		}
	}
	return [...ranks]
		.map(([label, rank]) => ({ label, rank }))
		.sort(byRank)
		.map(({ label }) => ({ label, count: counts.get(label) ?? 0 }));
}

// narrowing drops the facets and toggles that wouldn't narrow the list: a facet whose options all
// share one value, or a toggle every option passes (or none does).
export function narrowing(
	options: Option[],
	defs: PickerFilters,
): { facets: Facet[]; toggles: Toggle[] } {
	return {
		facets: defs.facets.filter(
			(f) =>
				new Set(options.flatMap((o) => f.values(o).map((v) => v.label))).size >
				1,
		),
		toggles: defs.toggles.filter((t) => {
			const n = options.filter(t.test).length;
			return n > 0 && n < options.length;
		}),
	};
}

export function sortOptions(options: Option[], sort?: Sort): Option[] {
	return sort ? [...options].sort(sort.compare) : options;
}

export function activeCount(state: FilterState): number {
	return (
		Object.values(state.facets).reduce((n, v) => n + v.length, 0) +
		state.toggles.length
	);
}
