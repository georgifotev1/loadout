import type { components } from "#/lib/api/schema.gen";

export type Category = components["schemas"]["Category"];

// The order the header, menu and footer list them in.
export const componentCategories: Category[] = [
	"cpu",
	"gpu",
	"motherboard",
	"ram",
	"storage",
	"psu",
	"case",
	"cooler",
];

export const peripheralCategories: Category[] = [
	"peripheral.monitor",
	"peripheral.keyboard",
	"peripheral.mouse",
	"peripheral.headset",
	"peripheral.mousepad",
	"peripheral.chair",
];

// A Record so a category added to api/openapi.yaml is a type error here until it's listed.
const known: Record<Category, true> = {
	cpu: true,
	motherboard: true,
	ram: true,
	gpu: true,
	psu: true,
	case: true,
	cooler: true,
	storage: true,
	"peripheral.chair": true,
	"peripheral.headset": true,
	"peripheral.keyboard": true,
	"peripheral.monitor": true,
	"peripheral.mouse": true,
	"peripheral.mousepad": true,
};

export const categories = Object.keys(known) as Category[];

export function isCategory(s: unknown): s is Category {
	return typeof s === "string" && Object.hasOwn(known, s);
}
