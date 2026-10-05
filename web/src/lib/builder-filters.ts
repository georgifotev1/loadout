import { m } from "#/i18n";
import type { Schemas } from "#/lib/api/client";
import { capacityLabel, type Slot } from "#/lib/builder";

type Option = Schemas["BuilderOption"];

// The part picker's filters, worked out from the options that fit, as PC builders and shops
// usually do it: a few spec facets per part type (socket, memory type, capacity, ...) where
// values in one facet widen the list and separate facets narrow it, yes/no toggles, and a sort.

// A facet value: the label shown and the number it sorts by (so 512 GB comes before 1 TB).
type Value = { label: string; rank: number };

export type Facet = {
	id: string;
	label: string;
	values: (o: Option) => Value[];
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
const str = (o: Option, k: string) =>
	typeof o.specs[k] === "string" && o.specs[k] !== ""
		? (o.specs[k] as string)
		: undefined;
const price = (o: Option) => o.product.price.amount_cents;

// one turns a spec into a single facet value, or none when it's unknown.
function one(label: string | undefined, rank = 0): Value[] {
	return label === undefined ? [] : [{ label, rank }];
}

function spec(id: string, label: string, key: string): Facet {
	return { id, label, values: (o) => one(str(o, key)) };
}

function amount(
	id: string,
	label: string,
	key: string,
	format: (n: number) => string,
): Facet {
	return {
		id,
		label,
		values: (o) => {
			const n = num(o, key);
			return n === undefined ? [] : one(format(n), n);
		},
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
		values: (o) => one(o.product.brand),
	};
	const watts = (n: number) => `${n} W`;
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
				facets: [brand, spec("socket", t.socket, "socket")],
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
					spec("form", t.formFactor, "form_factor"),
					spec("memory", t.memoryType, "memory_type"),
					spec("chipset", t.chipset, "chipset"),
				],
				toggles: [],
				sorts: [...base, byDesc("m2", t.sortM2, "m2_slots")],
			};
		case "ram":
			return {
				facets: [
					spec("type", t.memoryType, "type"),
					amount("capacity", t.capacity, "capacity_gb_total", capacityLabel),
					amount("modules", t.modules, "modules", (n) => t.moduleCount(n)),
					amount("speed", t.speed, "speed_mhz", (n) => `${n} MHz`),
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
					{
						id: "type",
						label: t.coolerType,
						values: (o) =>
							o.specs.type === "aio"
								? one(t.aio, 2)
								: o.specs.type === "air"
									? one(t.air, 1)
									: [],
					},
					amount("radiator", t.radiator, "radiator_mm", (n) => `${n} mm`),
					brand,
				],
				toggles: [],
				sorts: [...base, byAsc("height", t.sortHeight, "height_mm")],
			};
		case "gpu":
			return {
				facets: [brand],
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
					{
						id: "interface",
						label: t.interface,
						values: (o) =>
							({ nvme: one("NVMe", 1), sata: one("SATA", 2) })[
								str(o, "interface") ?? ""
							] ?? [],
					},
					amount("capacity", t.capacity, "capacity_gb", capacityLabel),
					{
						id: "form",
						label: t.formFactor,
						values: (o) =>
							({ m2_2280: one("M.2", 1), "2.5": one("2,5″", 2) })[
								str(o, "form_factor") ?? ""
							] ?? [],
					},
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
					amount("wattage", t.wattage, "wattage_w", watts),
					spec("form", t.formFactor, "form_factor"),
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
					{
						id: "board",
						label: t.boardSize,
						values: (o) =>
							Array.isArray(o.specs.form_factors_supported)
								? (o.specs.form_factors_supported as string[]).map((f) => ({
										label: f,
										rank: boardRank[f] ?? 9,
									}))
								: [],
					},
					spec("psu", t.psuSize, "psu_form_factor"),
					brand,
				],
				toggles: [],
				sorts: [...base, byDesc("gpu", t.sortGpuRoom, "max_gpu_length_mm")],
			};
	}
}

const boardRank: Record<string, number> = {
	ITX: 1,
	mATX: 2,
	ATX: 3,
	"E-ATX": 4,
};

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
		.sort(
			([a, ra], [b, rb]) =>
				ra - rb || a.localeCompare(b, undefined, { numeric: true }),
		)
		.map(([label]) => ({ label, count: counts.get(label) ?? 0 }));
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
