import { bg } from "./bg";
import { locale } from "./locale";

export { locale };

export type Messages = typeof bg;

export const m: Messages = bg;

const price = new Intl.NumberFormat(locale, {
	style: "currency",
	currency: "EUR",
});

// formatPrice formats euro cents as the API sends them, e.g. 36406 → "364,06 €".
export function formatPrice(cents: number): string {
	return price.format(cents / 100);
}

const shortDate = new Intl.DateTimeFormat(locale, {
	weekday: "short",
	day: "numeric",
	month: "long",
});

// formatShortDate formats a date like "чт, 8 октомври".
export function formatShortDate(d: Date): string {
	return shortDate.format(d);
}
