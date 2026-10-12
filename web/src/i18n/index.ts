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

const percent = new Intl.NumberFormat(locale, {
	style: "percent",
	maximumFractionDigits: 2,
});

// formatPercent formats basis points as the API sends them, e.g. 1250 → "12,5%".
export function formatPercent(bp: number): string {
	return percent.format(bp / 10000);
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

const longDate = new Intl.DateTimeFormat(locale, {
	day: "numeric",
	month: "long",
	year: "numeric",
	timeZone: "UTC",
});

// formatLongDate formats a calendar day, "2026-10-12", as "12 октомври 2026 г.".
export function formatLongDate(day: string): string {
	return longDate.format(new Date(`${day}T00:00:00Z`));
}

const dateTime = new Intl.DateTimeFormat(locale, {
	day: "numeric",
	month: "numeric",
	year: "numeric",
	hour: "2-digit",
	minute: "2-digit",
});

// formatDateTime formats a moment like "6.10.2026 г., 14:02".
export function formatDateTime(d: Date): string {
	return dateTime.format(d);
}

const relative = new Intl.RelativeTimeFormat(locale, { numeric: "auto" });

const relativeUnits: [Intl.RelativeTimeFormatUnit, number][] = [
	["day", 86_400],
	["hour", 3_600],
	["minute", 60],
];

// formatRelativeTime says how long ago d was, e.g. "преди 5 минути", "вчера". It depends on
// the clock, so the server and the browser can disagree: render it with
// suppressHydrationWarning.
export function formatRelativeTime(d: Date, now = Date.now()): string {
	const seconds = Math.round((d.getTime() - now) / 1000);
	for (const [unit, size] of relativeUnits) {
		if (Math.abs(seconds) >= size) {
			return relative.format(Math.trunc(seconds / size), unit);
		}
	}
	return relative.format(seconds, "second");
}
