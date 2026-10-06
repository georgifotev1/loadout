import { m } from "#/i18n";
import type { CartAddLine } from "#/lib/api/cart";
import type { Schemas } from "#/lib/api/client";

export type Slot = Schemas["Slot"];
export type Finding = Schemas["Finding"];
export type BuildCheck = Schemas["BuildCheck"];
export type BuildPart = Schemas["BuildPart"];
export type Severity = Finding["severity"];

// BuildSearch is the builder's URL state: the picked parts by product slug, the same shape as
// the API's build query parameters and the saved-build body.
export type BuildSearch = Schemas["BuildSelection"];

// The order the builder lists the slots in: each part narrows the next (the CPU picks the
// socket, the board the memory type), so following it shows the fewest dead ends.
export const slotOrder: Slot[] = [
	"cpu",
	"motherboard",
	"ram",
	"cooler",
	"gpu",
	"storage",
	"psu",
	"case",
];

type MultiSlot = "ram" | "storage";

// The slots that take several parts, with the API's limit. RAM is several of one kit (mixed kits
// rarely run at their rated speed); storage can mix drives.
export const maxParts: Record<MultiSlot, number> = { ram: 4, storage: 8 };

export function isMultiSlot(slot: Slot): slot is MultiSlot {
	return slot === "ram" || slot === "storage";
}

// parseBuildSearch reads BuildSearch from untrusted URL params. Anything malformed is dropped,
// so an old or hand-edited link still opens the builder.
export function parseBuildSearch(s: Record<string, unknown>): BuildSearch {
	const out: BuildSearch = {};
	for (const slot of slotOrder) {
		if (isMultiSlot(slot)) {
			const list = (Array.isArray(s[slot]) ? s[slot] : [s[slot]])
				.filter(isSlug)
				.slice(0, maxParts[slot]);
			if (list.length) out[slot] = list;
		} else if (isSlug(s[slot])) {
			out[slot] = s[slot];
		}
	}
	return out;
}

function isSlug(v: unknown): v is string {
	return typeof v === "string" && v !== "" && v.length <= 200;
}

// slugsIn lists a slot's picked slugs, in order and with repeats.
export function slugsIn(b: BuildSearch, slot: Slot): string[] {
	if (isMultiSlot(slot)) return b[slot] ?? [];
	const v = b[slot];
	return v ? [v] : [];
}

export function isEmptyBuild(b: BuildSearch): boolean {
	return slotOrder.every((s) => slugsIn(b, s).length === 0);
}

// pick puts slug in slot. For RAM it replaces the kit (one kit, see maxParts). For storage it
// replaces the drive `replacing`, or adds a drive when that's not given.
export function pick(
	b: BuildSearch,
	slot: Slot,
	slug: string,
	replacing?: string,
): BuildSearch {
	if (slot === "ram") return { ...b, ram: [slug] };
	if (slot === "storage") {
		const list = b.storage ?? [];
		if (replacing === undefined) {
			return { ...b, storage: [...list, slug].slice(0, maxParts.storage) };
		}
		return { ...b, storage: list.map((s) => (s === replacing ? slug : s)) };
	}
	return { ...b, [slot]: slug };
}

// remove takes slug (every copy of it) out of slot.
export function remove(b: BuildSearch, slot: Slot, slug: string): BuildSearch {
	if (isMultiSlot(slot)) {
		const list = (b[slot] ?? []).filter((s) => s !== slug);
		return { ...b, [slot]: list.length ? list : undefined };
	}
	return without(b, slot);
}

// setCount buys n copies of slug in a multi-part slot, keeping where it was in the list.
export function setCount(
	b: BuildSearch,
	slot: MultiSlot,
	slug: string,
	n: number,
): BuildSearch {
	const list = b[slot] ?? [];
	if (!list.includes(slug)) return b;
	const room = maxParts[slot] - list.filter((s) => s !== slug).length;
	const copies: string[] = Array(Math.max(1, Math.min(n, room))).fill(slug);
	const out: string[] = [];
	for (const s of list) {
		if (s !== slug) out.push(s);
		else if (!out.includes(slug)) out.push(...copies);
	}
	return { ...b, [slot]: out };
}

export function without(b: BuildSearch, slot: Slot): BuildSearch {
	const { [slot]: _, ...rest } = b;
	return rest;
}

// groupSlugs folds repeats into counts, in first-seen order: [a, b, a] → a×2, b×1.
export function groupSlugs(list: string[]): { slug: string; count: number }[] {
	const counts = new Map<string, number>();
	for (const s of list) counts.set(s, (counts.get(s) ?? 0) + 1);
	return [...counts].map(([slug, count]) => ({ slug, count }));
}

// selectionOf turns a checked build back into a BuildSearch, e.g. to edit a saved build. Parts
// that aren't for sale any more are kept, so the builder shows what needs replacing.
export function selectionOf(check: BuildCheck): BuildSearch {
	let b: BuildSearch = {};
	const all = [
		...check.parts.map((p) => ({ slot: p.slot, slug: p.product.slug })),
		...check.unavailable,
	];
	for (const { slot, slug } of all) {
		b = isMultiSlot(slot)
			? { ...b, [slot]: [...(b[slot] ?? []), slug] }
			: { ...b, [slot]: slug };
	}
	return b;
}

// cartLinesOf turns a build's parts into cart lines: one per product, a part bought twice as
// quantity 2.
export function cartLinesOf(check: BuildCheck): CartAddLine[] {
	const counts = new Map<string, number>();
	for (const p of check.parts) {
		counts.set(p.product.slug, (counts.get(p.product.slug) ?? 0) + 1);
	}
	return [...counts].map(([slug, quantity]) => ({ slug, quantity }));
}

// canOrder says whether a build can go into the cart: complete, without compatibility errors,
// and every part still for sale.
export function canOrder(check: BuildCheck, status: BuildStatus): boolean {
	return (
		status.missing.length === 0 &&
		status.errors === 0 &&
		check.unavailable.length === 0
	);
}

// The findings that only say a part is still missing. The builder shows them on the empty slot
// itself, not as problems with the parts that are picked.
const missingRules = new Set<Finding["rule"]>([
	"PART_REQUIRED",
	"COOLER_REQUIRED",
	"GPU_REQUIRED",
]);

export function isMissingPart(f: Finding): boolean {
	return missingRules.has(f.rule);
}

// BuildStatus is what the builder needs to know about a checked build, worked out once.
export type BuildStatus = {
	// The slots that need a part before the build can be ordered.
	missing: Slot[];
	// The slots a complete build needs: always six, plus a cooler for a CPU sold without one
	// and a graphics card for a CPU without integrated graphics.
	required: Slot[];
	// Compatibility errors, warnings and notes about the picked parts, worst first.
	problems: Finding[];
	errors: number;
	warnings: number;
	// The first missing slot in slotOrder: where the builder points next.
	next?: Slot;
};

export function buildStatus(check: BuildCheck): BuildStatus {
	const all = [...check.errors, ...check.warnings, ...check.info];
	const missingSet = new Set(
		all.filter(isMissingPart).map((f) => f.slots[0] as Slot),
	);
	const missing = slotOrder.filter((s) => missingSet.has(s));
	const cpu = check.parts.find((p) => p.slot === "cpu")?.specs;
	const required = slotOrder.filter((s) => {
		if (s === "cooler") return cpu?.cooler_included === false;
		if (s === "gpu") return cpu?.igpu === false;
		return true;
	});
	const problems = all.filter((f) => !isMissingPart(f));
	return {
		missing,
		required,
		problems,
		errors: problems.filter((f) => f.severity === "error").length,
		warnings: problems.filter((f) => f.severity === "warning").length,
		next: missing[0],
	};
}

// hasPowerParts says whether the power estimate means anything yet: without a CPU or GPU it's
// only the fixed allowance for the board and fans.
export function hasPowerParts(check: BuildCheck): boolean {
	return check.parts.some((p) => p.slot === "cpu" || p.slot === "gpu");
}

const severityRank: Record<Severity, number> = {
	error: 3,
	warning: 2,
	info: 1,
};

// worstSeverity is the most serious severity among the findings involving slot.
export function worstSeverity(
	findings: Finding[],
	slot: Slot,
): Severity | undefined {
	let worst: Severity | undefined;
	for (const f of findings) {
		if (!f.slots.includes(slot)) continue;
		if (!worst || severityRank[f.severity] > severityRank[worst]) {
			worst = f.severity;
		}
	}
	return worst;
}

// ownFindings are the findings to spell out on slot's row. A finding names its main part first
// (the case for a GPU that's too long, the RAM for the wrong memory type), so it's written
// once there; the other parts involved only get the severity marker.
export function ownFindings(findings: Finding[], slot: Slot): Finding[] {
	return findings.filter((f) => f.slots[0] === slot);
}

// relatedSlots are the other rows that spell out a problem involving slot, worst first, so a
// part marked red or amber can point to the row that says why.
export function relatedSlots(findings: Finding[], slot: Slot): Slot[] {
	const out: { slot: Slot; rank: number }[] = [];
	for (const f of findings) {
		const owner = f.slots[0];
		if (!owner || owner === slot || !f.slots.includes(slot)) continue;
		if (f.severity === "info") continue;
		const seen = out.find((o) => o.slot === owner);
		const rank = severityRank[f.severity];
		if (!seen) out.push({ slot: owner, rank });
		else seen.rank = Math.max(seen.rank, rank);
	}
	return out.sort((a, b) => b.rank - a.rank).map((o) => o.slot);
}

// findingText is the shopper-facing message for a finding, filled from its params.
export function findingText(f: Finding): string {
	const p = f.params as Record<string, unknown>;
	const num = (k: string) => Number(p[k]);
	const str = (k: string) => String(p[k] ?? "");
	const list = (k: string) =>
		Array.isArray(p[k]) ? (p[k] as unknown[]).join(", ") : str(k);
	const t = m.builder.findings;
	switch (f.rule) {
		case "CPU_MB_SOCKET":
			return t.cpuSocket(str("cpu_socket"), str("motherboard_socket"));
		case "CPU_MB_GENERATION":
			return t.cpuGeneration;
		case "RAM_MB_TYPE":
			return t.ramType(list("ram_types"), str("motherboard_type"));
		case "RAM_SLOTS":
			return t.ramSlots(num("modules"), num("slots"));
		case "RAM_CAPACITY":
			return t.ramCapacity(num("capacity_gb"), num("max_gb"));
		case "RAM_SPEED":
			return t.ramSpeed(num("ram_mhz"), num("max_mhz"));
		case "RAM_DUAL_CHANNEL":
			return t.ramDualChannel;
		case "CASE_MB_FORM":
			return t.caseBoard(str("form_factor"), list("supported"));
		case "CASE_GPU_LENGTH":
			return f.severity === "error"
				? t.caseGpu(num("gpu_mm"), num("max_mm"))
				: t.caseGpuTight(num("max_mm") - num("gpu_mm"));
		case "CASE_COOLER_HEIGHT":
			return t.caseCooler(num("cooler_mm"), num("max_mm"));
		case "CASE_RADIATOR":
			return t.caseRadiator(num("radiator_mm"));
		case "CASE_PSU_FORM":
			return t.casePsu(str("psu_form_factor"), str("case_form_factor"));
		case "COOLER_SOCKET":
			return t.coolerSocket(str("cpu_socket"));
		case "COOLER_TDP":
			return t.coolerTdp(num("cooler_w"), num("cpu_w"));
		case "PSU_WATTAGE":
			return f.severity === "error"
				? t.psuTooWeak(num("psu_w"), num("estimate_w"), num("recommended_w"))
				: t.psuHigh(num("psu_w"), num("estimate_w"), num("recommended_w"));
		case "PSU_CONNECTORS":
			return t.psuConnector(str("connector"));
		case "STORAGE_M2_SLOTS":
			return t.m2Slots(num("drives"), num("slots"));
		case "MISSING_SPEC":
			return t.missingSpec(
				m.builder.slot[f.slots[0] as Slot],
				m.builder.specField[str("field")] ?? str("field"),
			);
		case "PART_REQUIRED":
			return t.partRequired(m.builder.slot[f.slots[0] as Slot]);
		case "COOLER_REQUIRED":
			return t.coolerRequired;
		case "GPU_REQUIRED":
			return t.gpuRequired;
	}
}

type Specs = Record<string, unknown>;

// keySpecs are the few values that decide whether a part fits, shown as chips next to it.
export function keySpecs(slot: Slot, s: Specs): string[] {
	const n = (k: string) => (typeof s[k] === "number" ? s[k] : undefined);
	const str = (k: string) =>
		typeof s[k] === "string" && s[k] !== "" ? s[k] : undefined;
	const t = m.builder.spec;
	const out: (string | false | undefined)[] = (() => {
		switch (slot) {
			case "cpu":
				return [
					str("socket"),
					n("tdp_w") !== undefined && `${n("tdp_w")} W`,
					s.igpu === true && t.igpu,
					s.cooler_included === true && t.boxCooler,
				];
			case "motherboard":
				return [
					str("socket"),
					str("chipset"),
					str("form_factor"),
					str("memory_type"),
				];
			case "ram": {
				const gb = n("capacity_gb_total");
				const mods = n("modules");
				return [
					str("type"),
					gb !== undefined &&
						(mods && mods > 1 ? `${gb} GB (${mods}×${gb / mods})` : `${gb} GB`),
					n("speed_mhz") !== undefined && `${n("speed_mhz")} MHz`,
				];
			}
			case "gpu":
				return [
					str("chipset"),
					n("vram_gb") !== undefined && `${n("vram_gb")} GB`,
					n("tdp_w") !== undefined && `${n("tdp_w")} W`,
					n("length_mm") !== undefined && `${n("length_mm")} mm`,
				];
			case "cooler":
				return s.type === "aio"
					? [t.aio, n("radiator_mm") !== undefined && `${n("radiator_mm")} mm`]
					: [
							s.type === "air" && t.air,
							n("height_mm") !== undefined &&
								t.height(n("height_mm") as number),
						];
			case "storage":
				return [
					n("capacity_gb") !== undefined &&
						capacityLabel(n("capacity_gb") as number),
					{ nvme: "NVMe", sata: "SATA" }[str("interface") ?? ""],
					{ m2_2280: "M.2", "2.5": "2,5″" }[str("form_factor") ?? ""],
				];
			case "psu":
				return [
					n("wattage_w") !== undefined && `${n("wattage_w")} W`,
					str("form_factor"),
				];
			case "case":
				return [
					Array.isArray(s.form_factors_supported) &&
						s.form_factors_supported.join(" / "),
					n("max_gpu_length_mm") !== undefined &&
						t.gpuUpTo(n("max_gpu_length_mm") as number),
				];
		}
	})();
	return out.filter((v): v is string => typeof v === "string" && v !== "");
}

export function capacityLabel(gb: number): string {
	return gb >= 1000 && gb % 1000 === 0 ? `${gb / 1000} TB` : `${gb} GB`;
}

// recommendedPSU picks the option to badge in the PSU picker: the cheapest one with at least
// the recommended wattage and nothing to warn about. Options come cheapest first.
export function recommendedPSU(
	options: Schemas["BuilderOption"][],
	recommendedW: number,
): string | undefined {
	if (recommendedW <= 0) return undefined;
	return options.find(
		(o) =>
			Number(o.specs.wattage_w) >= recommendedW &&
			!o.findings.some((f) => f.severity !== "info"),
	)?.product.slug;
}
