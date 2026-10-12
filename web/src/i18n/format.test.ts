import { describe, expect, it } from "vitest";
import { formatLongDate, formatRelativeTime, m } from "#/i18n";

describe("formatRelativeTime", () => {
	const now = Date.parse("2026-10-06T12:00:00Z");
	const ago = (s: number) => formatRelativeTime(new Date(now - s * 1000), now);

	it("picks the largest whole unit", () => {
		expect(ago(30)).toBe("преди 30 секунди");
		expect(ago(5 * 60)).toBe("преди 5 минути");
		expect(ago(2 * 3600 + 59 * 60)).toBe("преди 2 часа");
		expect(ago(86_400)).toBe("вчера");
	});

	it("says now for the same moment", () => {
		expect(ago(0)).toBe("сега");
	});
});

describe("admin duration", () => {
	it.each([
		[0, "< 1 с"],
		[0.4, "< 1 с"],
		[42, "42 с"],
		[72, "1 мин 12 с"],
		[3 * 3600 + 5 * 60 + 9, "3 ч 5 мин"],
	])("%i seconds is %s", (s, want) => {
		expect(m.admin.duration(s)).toBe(want);
	});
});

describe("formatLongDate", () => {
	it("writes the day out in Bulgarian, whatever the time zone", () => {
		expect(formatLongDate("2026-10-12")).toBe("12 октомври 2026 г.");
	});
});
