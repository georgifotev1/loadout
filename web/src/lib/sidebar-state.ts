import { createIsomorphicFn } from "@tanstack/react-start";
import { getRequestHeader } from "@tanstack/react-start/server";
import { SIDEBAR_COOKIE_NAME } from "#/components/ui/sidebar";

// The cookie header: the page request's during SSR, the document's in the browser.
const cookies = createIsomorphicFn()
	.server(() => getRequestHeader("cookie") ?? "")
	.client(() => document.cookie);

/**
 * Whether the admin sidebar starts open. shadcn's sidebar remembers a collapse in a cookie;
 * reading it before render keeps a collapsed sidebar from flashing open on load.
 */
export function sidebarOpen(): boolean {
	return parseSidebarCookie(cookies());
}

export function parseSidebarCookie(header: string): boolean {
	for (const part of header.split(";")) {
		const [name, value] = part.trim().split("=");
		if (name === SIDEBAR_COOKIE_NAME) return value !== "false";
	}
	return true;
}
