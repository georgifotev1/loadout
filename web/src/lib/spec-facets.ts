import { m } from "#/i18n";
import { capacityLabel, type Slot, slotOrder } from "#/lib/builder";

// How spec facet values read, for the shop listings (from GET /api/search's `specs` facets) and
// the builder's part picker (from each option's specs). Values arrive as the canonical text the
// API uses ("AM5", "16", "nvme").

// specFacetFields are the spec fields each category's listing can be filtered by, as the API
// lists them in internal/specs/facets.go. Filters on any other field are dropped from old or
// hand-edited links instead of failing the page.
export const specFacetFields: Record<Slot, string[]> = {
	cpu: ["socket"],
	motherboard: ["socket", "chipset", "form_factor", "memory_type"],
	ram: ["type", "capacity_gb_total", "modules", "speed_mhz"],
	gpu: ["chipset", "vram_gb"],
	cooler: ["type", "sockets", "radiator_mm"],
	storage: ["interface", "capacity_gb", "form_factor"],
	psu: ["wattage_w", "form_factor"],
	case: ["form_factors_supported", "psu_form_factor"],
};

export function isSlot(c: unknown): c is Slot {
	return slotOrder.includes(c as Slot);
}

export function specFacetLabel(slot: Slot, field: string): string {
	const t = m.builder.filters;
	switch (field) {
		case "socket":
		case "sockets":
			return t.socket;
		case "chipset":
			return slot === "gpu" ? t.gpuChip : t.chipset;
		case "form_factor":
			return t.formFactor;
		case "memory_type":
			return t.memoryType;
		case "type":
			return slot === "cooler" ? t.coolerType : t.memoryType;
		case "capacity_gb_total":
		case "capacity_gb":
			return t.capacity;
		case "vram_gb":
			return t.vram;
		case "modules":
			return t.modules;
		case "speed_mhz":
			return t.speed;
		case "radiator_mm":
			return t.radiator;
		case "interface":
			return t.interface;
		case "wattage_w":
			return t.wattage;
		case "form_factors_supported":
			return t.boardSize;
		case "psu_form_factor":
			return t.psuSize;
	}
	return field;
}

// A facet value: the label shown and the number it sorts by (so 512 GB comes before 1 TB).
export type SpecValue = { label: string; rank: number };

const boardRank: Record<string, number> = {
	ITX: 1,
	mATX: 2,
	ATX: 3,
	"E-ATX": 4,
};

export function specValue(slot: Slot, field: string, value: string): SpecValue {
	const t = m.builder.filters;
	const n = Number(value);
	switch (field) {
		case "capacity_gb_total":
		case "capacity_gb":
			return { label: capacityLabel(n), rank: n };
		case "vram_gb":
			return { label: `${n} GB`, rank: n };
		case "modules":
			return { label: t.moduleCount(n), rank: n };
		case "speed_mhz":
			return { label: `${n} MHz`, rank: n };
		case "wattage_w":
			return { label: `${n} W`, rank: n };
		case "radiator_mm":
			return { label: `${n} mm`, rank: n };
		case "type":
			if (slot === "cooler") {
				if (value === "air") return { label: t.air, rank: 1 };
				if (value === "aio") return { label: t.aio, rank: 2 };
			}
			break;
		case "interface":
			if (value === "nvme") return { label: "NVMe", rank: 1 };
			if (value === "sata") return { label: "SATA", rank: 2 };
			break;
		case "form_factor":
		case "form_factors_supported":
			if (slot === "storage") {
				if (value === "m2_2280") return { label: "M.2", rank: 1 };
				if (value === "2.5") return { label: "2,5″", rank: 2 };
			}
			return { label: value, rank: boardRank[value] ?? 0 };
	}
	return { label: value, rank: 0 };
}

// specValues turns a raw spec value into the facet values it counts under, as the API does: one
// for a string or number, one per entry for a list.
export function specValues(raw: unknown): string[] {
	if (typeof raw === "string" && raw !== "") return [raw];
	if (typeof raw === "number") return [String(raw)];
	if (Array.isArray(raw)) {
		return raw.filter((v): v is string => typeof v === "string" && v !== "");
	}
	return [];
}

// byRank orders facet values by rank, then by label with numbers in number order.
export function byRank(a: SpecValue, b: SpecValue): number {
	return (
		a.rank - b.rank ||
		a.label.localeCompare(b.label, undefined, { numeric: true })
	);
}
