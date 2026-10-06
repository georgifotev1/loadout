import { describe, expect, it } from "vitest";
import type { MarkupRule } from "#/lib/api/admin-catalog";
import {
	draftOf,
	formatHundredths,
	newRuleDraft,
	parseHundredths,
	toRuleInput,
} from "./markup-rules";

describe("parseHundredths", () => {
	it.each([
		["15", 1500],
		["12,5", 1250],
		["12.50", 1250],
		[" 0 ", 0],
		["4,9", 490],
		["0,05", 5],
	])("%s → %d", (text, want) => {
		expect(parseHundredths(text)).toBe(want);
	});

	it.each(["", "-5", "1,234", "abc", "1e3", "1,2,3"])("rejects %j", (text) => {
		expect(parseHundredths(text)).toBeNull();
	});
});

describe("formatHundredths", () => {
	it.each([
		[1500, "15"],
		[1250, "12,5"],
		[490, "4,9"],
		[5, "0,05"],
		[0, "0"],
	])("%d → %s", (n, want) => {
		expect(formatHundredths(n)).toBe(want);
	});
});

describe("toRuleInput", () => {
	it("sends only the scope's target fields", () => {
		const draft = {
			...newRuleDraft,
			scope: "sku" as const,
			category: "gpu" as const,
			brand: "MSI",
			sku: "ABC-1",
			markup: "12,5",
			minMargin: "5",
		};
		expect(toRuleInput(draft)).toEqual({
			input: {
				scope: "sku",
				sku: "ABC-1",
				markup_bp: 1250,
				min_margin_cents: 500,
				rounding: "90",
			},
		});
	});

	it("leaves an unpicked category for the API to call required", () => {
		const out = toRuleInput({ ...newRuleDraft, markup: "10" });
		expect(out).toEqual({
			input: {
				scope: "category",
				markup_bp: 1000,
				min_margin_cents: 0,
				rounding: "90",
			},
		});
	});

	it("reports numbers that aren't", () => {
		expect(
			toRuleInput({ ...newRuleDraft, markup: "x", minMargin: "-1" }),
		).toEqual({ errors: { markup_bp: "number", min_margin_cents: "number" } });
	});

	it("round-trips a rule through a draft", () => {
		const rule: MarkupRule = {
			id: 1,
			scope: "brand_category",
			category: "gpu",
			brand: "Gigabyte",
			markup_bp: 1850,
			min_margin_cents: 990,
			rounding: "99",
			version: 3,
			products: 4,
			updated_at: "2026-10-06T10:00:00Z",
		};
		expect(toRuleInput(draftOf(rule))).toEqual({
			input: {
				scope: "brand_category",
				category: "gpu",
				brand: "Gigabyte",
				markup_bp: 1850,
				min_margin_cents: 990,
				rounding: "99",
			},
		});
	});
});
