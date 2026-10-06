import type {
	MarkupRule,
	MarkupRuleInput,
	Rounding,
	RuleScope,
} from "#/lib/api/admin-catalog";
import type { Category } from "#/lib/categories";

// The markup rule dialog keeps the form as text. These turn a rule into that text and back into
// the body the API takes. They only convert: whether a rule makes sense (a category for a
// category rule, limits) is the API's call and comes back as field errors. Numbers are the
// exception, since a field typed as an integer can't carry text to the API.

export type RuleDraft = {
	scope: RuleScope;
	category: Category | "";
	brand: string;
	sku: string;
	markup: string; // percent, e.g. "15" or "12,5"
	minMargin: string; // euros, e.g. "5" or "4,90"
	rounding: Rounding;
};

export const newRuleDraft: RuleDraft = {
	scope: "category",
	category: "",
	brand: "",
	sku: "",
	markup: "",
	minMargin: "0",
	rounding: "90",
};

export function draftOf(r: MarkupRule): RuleDraft {
	return {
		scope: r.scope,
		category: r.category ?? "",
		brand: r.brand ?? "",
		sku: r.sku ?? "",
		markup: formatHundredths(r.markup_bp),
		minMargin: formatHundredths(r.min_margin_cents),
		rounding: r.rounding,
	};
}

export type DraftErrors = Partial<
	Record<"markup_bp" | "min_margin_cents", "number">
>;

/**
 * The API body for a draft, with only the target fields its scope uses, or the number fields
 * that aren't numbers.
 */
export function toRuleInput(
	d: RuleDraft,
): { input: MarkupRuleInput } | { errors: DraftErrors } {
	const markup = parseHundredths(d.markup);
	const minMargin = parseHundredths(d.minMargin);
	if (markup === null || minMargin === null) {
		const errors: DraftErrors = {};
		if (markup === null) errors.markup_bp = "number";
		if (minMargin === null) errors.min_margin_cents = "number";
		return { errors };
	}
	const input: MarkupRuleInput = {
		scope: d.scope,
		markup_bp: markup,
		min_margin_cents: minMargin,
		rounding: d.rounding,
	};
	if ((d.scope === "category" || d.scope === "brand_category") && d.category) {
		input.category = d.category;
	}
	if (d.scope === "brand_category") input.brand = d.brand;
	if (d.scope === "sku") input.sku = d.sku;
	return { input };
}

/**
 * A non-negative decimal with up to two places ("12,5", "12.50", "12") in hundredths: percent
 * to basis points, euros to cents. null if it isn't one.
 */
export function parseHundredths(text: string): number | null {
	const t = text.trim().replace(",", ".");
	if (!/^\d{1,9}(\.\d{1,2})?$/.test(t)) return null;
	const [whole, frac = ""] = t.split(".");
	return Number(whole) * 100 + Number(frac.padEnd(2, "0"));
}

/** Hundredths as the form shows them: 1500 → "15", 1250 → "12,5", 490 → "4,9". */
export function formatHundredths(n: number): string {
	const whole = Math.trunc(n / 100);
	const frac = String(n % 100)
		.padStart(2, "0")
		.replace(/0+$/, "");
	return frac ? `${whole},${frac}` : String(whole);
}
