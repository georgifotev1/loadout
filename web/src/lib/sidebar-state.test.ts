import { describe, expect, it } from "vitest";
import { parseSidebarCookie } from "./sidebar-state";

describe("parseSidebarCookie", () => {
	it.each([
		["", true],
		["session=abc", true],
		["sidebar_state=true", true],
		["session=abc; sidebar_state=false", false],
		["sidebar_state=false; other=1", false],
		["my_sidebar_state=false", true],
	])("%j is open: %s", (header, want) => {
		expect(parseSidebarCookie(header)).toBe(want);
	});
});
