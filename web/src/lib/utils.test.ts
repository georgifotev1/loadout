import { describe, expect, it } from "vitest";
import { cn } from "./utils";

describe("cn", () => {
	it("joins conditional classes and resolves conflicts", () => {
		expect(
			cn("px-2 py-1", false && "hidden", { "text-sm": true }, "px-4"),
		).toBe("py-1 text-sm px-4");
	});

	it("knows the custom theme tokens", () => {
		// text-meta is a font size: it replaces text-sm and keeps the colour.
		expect(cn("text-sm text-muted-foreground", "text-meta")).toBe(
			"text-muted-foreground text-meta",
		);
		expect(cn("h-9", "h-header")).toBe("h-header");
		expect(cn("max-w-3xl", "max-w-page")).toBe("max-w-page");
	});
});
