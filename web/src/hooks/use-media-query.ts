import { useSyncExternalStore } from "react";

// useMediaQuery reports whether a CSS media query matches. It is false during SSR and the
// first render after hydration, so only use it for things that start closed (drawers, menus).
export function useMediaQuery(query: string): boolean {
	return useSyncExternalStore(
		(onChange) => {
			const mql = window.matchMedia(query);
			mql.addEventListener("change", onChange);
			return () => mql.removeEventListener("change", onChange);
		},
		() => window.matchMedia(query).matches,
		() => false,
	);
}
