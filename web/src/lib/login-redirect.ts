import type { Me } from "#/lib/api/auth";

/**
 * Where to go after logging in: the page that sent the user to /login, or else the admin for
 * admins and the home page for everyone else. `redirect` comes from the URL, so only a path on
 * this site is followed; "//evil.example" or "https://…" would send a fresh login elsewhere.
 */
export function afterLogin(me: Me, redirect?: string): string {
	if (redirect && isLocalPath(redirect)) return redirect;
	return me.role === "admin" ? "/admin" : "/";
}

function isLocalPath(path: string): boolean {
	return (
		path.startsWith("/") && !path.startsWith("//") && !path.startsWith("/\\")
	);
}
