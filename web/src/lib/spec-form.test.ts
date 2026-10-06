import { describe, expect, it } from "vitest";
import type { SpecField } from "#/lib/api/admin-specs";
import { changedValues, draftOf, fromDraft, toText } from "./spec-form";

const field = (f: Partial<SpecField>): SpecField => ({
	name: "socket",
	kind: "string",
	required: true,
	supplier: null,
	admin: null,
	value: null,
	...f,
});

describe("toText and draftOf", () => {
	it("formats each kind", () => {
		expect(toText(["DDR4", "DDR5"])).toBe("DDR4, DDR5");
		expect(toText(false)).toBe("false");
		expect(toText(65)).toBe("65");
		expect(toText(null)).toBe("");
	});
	it("drafts only the admin value", () => {
		expect(draftOf(field({ supplier: "AM4" }))).toBe("");
		expect(draftOf(field({ supplier: "AM4", admin: "AM5" }))).toBe("AM5");
	});
});

describe("fromDraft", () => {
	it("parses each kind", () => {
		expect(fromDraft("integer", " 65 ")).toBe(65);
		expect(fromDraft("boolean", "true")).toBe(true);
		expect(fromDraft("boolean", "false")).toBe(false);
		expect(fromDraft("string_list", "top:360, front:280,")).toEqual([
			"top:360",
			"front:280",
		]);
		expect(fromDraft("string", " AM5 ")).toBe("AM5");
	});
	it("is null when blank", () => {
		expect(fromDraft("integer", "  ")).toBeNull();
	});
	it("leaves a draft that isn't a number for the API to reject", () => {
		expect(fromDraft("integer", "65 W")).toBe("65 W");
	});
});

describe("changedValues", () => {
	const fields = [
		field({ name: "socket", admin: "AM5" }),
		field({ name: "tdp_w", kind: "integer" }),
		field({ name: "igpu", kind: "boolean", admin: true }),
		field({ name: "memory_types", kind: "string_list", admin: ["DDR5"] }),
	];
	it("sends only what changed", () => {
		expect(
			changedValues(fields, {
				socket: "AM5",
				tdp_w: "65",
				igpu: "true",
				memory_types: "DDR5",
			}),
		).toEqual({ tdp_w: 65 });
	});
	it("clears an admin value that was emptied", () => {
		expect(changedValues(fields, { socket: "", tdp_w: "" })).toEqual({
			socket: null,
		});
	});
});
