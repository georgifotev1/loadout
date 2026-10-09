import { describe, expect, it } from "vitest";
import {
	type BuildCheck,
	type BuildPart,
	buildHighlights,
	buildSearchOfParts,
	buildStatus,
	cartLinesOf,
	cpuModel,
	type Finding,
	findingText,
	groupSlugs,
	keySpecs,
	ownFindings,
	parseBuildSearch,
	pick,
	recommendedPSU,
	relatedSlots,
	remove,
	selectionOf,
	setCount,
	worstSeverity,
} from "./builder";

describe("parseBuildSearch", () => {
	it("keeps valid slugs and caps the multi-part slots", () => {
		expect(
			parseBuildSearch({
				cpu: "r5-7600",
				ram: "kit-a",
				storage: ["a", "b", "c", "d", "e", "f", "g", "h", "i"],
				gpu: "",
				case: 42,
				psu: ["not-a-list-slot"],
				other: "x",
			}),
		).toEqual({
			cpu: "r5-7600",
			ram: ["kit-a"],
			storage: ["a", "b", "c", "d", "e", "f", "g", "h"],
		});
	});
});

describe("build edits", () => {
	it("replaces the RAM kit and resets its count", () => {
		expect(pick({ ram: ["a", "a"] }, "ram", "b")).toEqual({ ram: ["b"] });
	});

	it("adds a drive, or swaps one", () => {
		expect(pick({ storage: ["a"] }, "storage", "b")).toEqual({
			storage: ["a", "b"],
		});
		expect(pick({ storage: ["a", "b", "a"] }, "storage", "c", "a")).toEqual({
			storage: ["c", "b", "c"],
		});
	});

	it("removes every copy, and the slot when it's empty", () => {
		expect(remove({ storage: ["a", "b", "a"] }, "storage", "a")).toEqual({
			storage: ["b"],
		});
		expect(remove({ cpu: "x", ram: ["a"] }, "ram", "a")).toEqual({
			cpu: "x",
			ram: undefined,
		});
		expect(remove({ cpu: "x", gpu: "y" }, "gpu", "y")).toEqual({ cpu: "x" });
	});

	it("sets a count in place and within the limit", () => {
		expect(setCount({ storage: ["a", "b"] }, "storage", "a", 3)).toEqual({
			storage: ["a", "a", "a", "b"],
		});
		expect(setCount({ ram: ["a", "a"] }, "ram", "a", 1)).toEqual({
			ram: ["a"],
		});
		expect(setCount({ ram: ["a"] }, "ram", "a", 9)).toEqual({
			ram: ["a", "a", "a", "a"],
		});
		expect(setCount({ ram: ["a"] }, "ram", "b", 2)).toEqual({ ram: ["a"] });
	});

	it("groups repeats in first-seen order", () => {
		expect(groupSlugs(["a", "b", "a"])).toEqual([
			{ slug: "a", count: 2 },
			{ slug: "b", count: 1 },
		]);
	});
});

const eur = (amount_cents: number) => ({
	amount_cents,
	currency: "EUR" as const,
});

function part(slot: BuildPart["slot"], slug: string, specs = {}): BuildPart {
	return {
		slot,
		specs,
		product: {
			slug,
			name: slug,
			brand: "B",
			category: slot,
			in_stock: true,
			price: eur(1000),
		},
	};
}

function check(c: Partial<BuildCheck>): BuildCheck {
	return {
		parts: [],
		unavailable: [],
		out_of_stock: [],
		errors: [],
		warnings: [],
		info: [],
		estimated_power_w: 0,
		recommended_psu_w: 0,
		total: eur(0),
		orderable: false,
		assembly: { offered: false, price: eur(3500), total: eur(3500) },
		...c,
	};
}

const finding = (
	rule: Finding["rule"],
	severity: Finding["severity"],
	slots: Finding["slots"],
	params = {},
): Finding => ({ rule, severity, slots, params });

describe("buildStatus", () => {
	it("separates missing parts from problems", () => {
		const s = buildStatus(
			check({
				parts: [part("cpu", "c", { cooler_included: false, igpu: true })],
				errors: [
					finding("COOLER_REQUIRED", "error", ["cooler", "cpu"]),
					finding("PART_REQUIRED", "error", ["motherboard"]),
					finding("PART_REQUIRED", "error", ["psu"]),
				],
				warnings: [finding("RAM_DUAL_CHANNEL", "warning", ["ram"])],
			}),
		);
		expect(s.missing).toEqual(["motherboard", "cooler", "psu"]);
		expect(s.next).toBe("motherboard");
		expect(s.required).toEqual([
			"cpu",
			"motherboard",
			"ram",
			"cooler",
			"storage",
			"psu",
			"case",
		]);
		expect(s.problems.map((f) => f.rule)).toEqual(["RAM_DUAL_CHANNEL"]);
		expect(s.errors).toBe(0);
		expect(s.warnings).toBe(1);
	});

	it("needs a GPU for a CPU without graphics", () => {
		const s = buildStatus(
			check({
				parts: [part("cpu", "c", { cooler_included: true, igpu: false })],
			}),
		);
		expect(s.required).toContain("gpu");
		expect(s.required).not.toContain("cooler");
	});
});

describe("findings per slot", () => {
	const fs = [
		finding("CASE_GPU_LENGTH", "warning", ["case", "gpu"]),
		finding("PSU_WATTAGE", "error", ["psu"]),
		finding("RAM_SPEED", "info", ["ram", "motherboard"]),
	];

	it("spells a finding out on its main part only", () => {
		expect(ownFindings(fs, "case").map((f) => f.rule)).toEqual([
			"CASE_GPU_LENGTH",
		]);
		expect(ownFindings(fs, "gpu")).toEqual([]);
	});

	it("points the other parts involved to the row that explains", () => {
		const more = [
			...fs,
			finding("CASE_PSU_FORM", "error", ["case", "psu"]),
			finding("CASE_GPU_LENGTH", "error", ["case", "gpu"]),
		];
		expect(relatedSlots(more, "psu")).toEqual(["case"]);
		expect(relatedSlots(more, "gpu")).toEqual(["case"]);
		expect(relatedSlots(more, "case")).toEqual([]);
		// Notes don't need a pointer.
		expect(relatedSlots(more, "motherboard")).toEqual([]);
	});

	it("marks every part involved with the worst severity", () => {
		expect(worstSeverity(fs, "gpu")).toBe("warning");
		expect(worstSeverity(fs, "psu")).toBe("error");
		expect(worstSeverity(fs, "motherboard")).toBe("info");
		expect(worstSeverity(fs, "cpu")).toBeUndefined();
	});
});

describe("findingText", () => {
	it("fills the message from params", () => {
		expect(
			findingText(
				finding("CASE_GPU_LENGTH", "error", ["case", "gpu"], {
					gpu_mm: 340,
					max_mm: 320,
				}),
			),
		).toContain("340 mm");
		expect(
			findingText(
				finding("CASE_GPU_LENGTH", "warning", ["case", "gpu"], {
					gpu_mm: 316,
					max_mm: 320,
				}),
			),
		).toContain("4 mm");
		expect(
			findingText(
				finding("MISSING_SPEC", "warning", ["gpu"], {
					field: "length_mm",
					check: "CASE_GPU_LENGTH",
				}),
			),
		).toContain("дължината");
	});
});

describe("keySpecs", () => {
	it("shows what decides the fit", () => {
		expect(
			keySpecs("ram", {
				type: "DDR5",
				capacity_gb_total: 32,
				modules: 2,
				speed_mhz: 6000,
			}),
		).toEqual(["DDR5", "32 GB (2×16)", "6000 MHz"]);
		expect(
			keySpecs("storage", {
				capacity_gb: 1000,
				interface: "nvme",
				form_factor: "m2_2280",
			}),
		).toEqual(["1 TB", "NVMe", "M.2"]);
		expect(keySpecs("gpu", { tdp_w: null })).toEqual([]);
	});
});

describe("recommendedPSU", () => {
	const psu = (slug: string, w: number, findings: Finding[] = []) => ({
		product: part("psu", slug).product,
		specs: { wattage_w: w },
		findings,
	});

	it("picks the cheapest PSU with enough watts and no warnings", () => {
		const options = [
			psu("small", 450),
			psu("warned", 650, [
				finding("PSU_CONNECTORS", "warning", ["psu", "gpu"]),
			]),
			psu("ok", 650),
			psu("big", 850),
		];
		expect(recommendedPSU(options, 600)).toBe("ok");
		expect(recommendedPSU(options, 0)).toBeUndefined();
	});
});

describe("selectionOf", () => {
	it("keeps unavailable parts so they can be replaced", () => {
		expect(
			selectionOf(
				check({
					parts: [part("cpu", "c"), part("ram", "r"), part("ram", "r")],
					unavailable: [{ slot: "storage", slug: "gone" }],
				}),
			),
		).toEqual({ cpu: "c", ram: ["r", "r"], storage: ["gone"] });
	});
});

describe("cartLinesOf", () => {
	it("counts a part bought twice", () => {
		expect(
			cartLinesOf(
				check({
					parts: [part("cpu", "c"), part("ram", "r"), part("ram", "r")],
				}),
			),
		).toEqual([
			{ slug: "c", quantity: 1 },
			{ slug: "r", quantity: 2 },
		]);
	});
});

describe("buildHighlights", () => {
	it("sums the build up", () => {
		const cpu = part("cpu", "r5", { igpu: false, cooler_included: true });
		cpu.product.name = "Процесор AMD RYZEN 5 7600, 6-Core, 3.8 GHz, AM5, BOX";
		const lines = buildHighlights(
			check({
				parts: [
					cpu,
					part("motherboard", "b650", { chipset: "B650", form_factor: "ATX" }),
					part("ram", "kit", {
						type: "DDR5",
						capacity_gb_total: 32,
						speed_mhz: 6000,
					}),
					part("ram", "kit", {
						type: "DDR5",
						capacity_gb_total: 32,
						speed_mhz: 6000,
					}),
					part("gpu", "rtx", { chipset: "GeForce RTX 5070", vram_gb: 12 }),
					part("storage", "ssd", { capacity_gb: 1000, interface: "nvme" }),
					part("storage", "ssd", { capacity_gb: 1000, interface: "nvme" }),
					part("storage", "hdd", { capacity_gb: 2000, interface: "sata" }),
					part("psu", "psu", { wattage_w: 750 }),
				],
			}),
		);
		expect(lines).toEqual([
			{ slot: "cpu", value: "AMD RYZEN 5 7600" },
			{ slot: "gpu", value: "GeForce RTX 5070 · 12 GB" },
			{ slot: "ram", value: "64 GB · DDR5 · 6000 MHz" },
			{ slot: "storage", value: "2 × 1 TB · NVMe + 2 TB · SATA" },
			{ slot: "motherboard", value: "B650 · ATX" },
			{ slot: "psu", value: "750 W" },
			{ slot: "cooler", value: "Охладителят на процесора" },
		]);
	});

	it("says when graphics are in the processor, and leaves out what isn't known", () => {
		const lines = buildHighlights(
			check({ parts: [part("cpu", "apu", { igpu: true }), part("psu", "p")] }),
		);
		expect(lines.map((l) => l.slot)).toEqual(["cpu", "gpu"]);
		expect(lines[1].value).toBe("Вградена в процесора");
	});
});

describe("cpuModel", () => {
	it("keeps the model", () => {
		expect(
			cpuModel("Процесор Intel Core Ultra 5 245K, 14-Core (6P+8E) 3.6 GHz"),
		).toBe("Intel Core Ultra 5 245K");
		expect(cpuModel("AMD Ryzen 7 7800X3D")).toBe("AMD Ryzen 7 7800X3D");
	});
});

describe("buildSearchOfParts", () => {
	it("puts each part in its slot, RAM and drives once per unit", () => {
		expect(
			buildSearchOfParts([
				{ category: "cpu", slug: "r5", quantity: 1 },
				{ category: "ram", slug: "kit", quantity: 2 },
				{ category: "storage", slug: "ssd", quantity: 1 },
				{ category: "peripheral.mouse", slug: "mouse", quantity: 1 },
			]),
		).toEqual({ cpu: "r5", ram: ["kit", "kit"], storage: ["ssd"] });
	});
});
