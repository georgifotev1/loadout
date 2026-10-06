import { useRouterState } from "@tanstack/react-router";

// useRefreshing is true while the current page loads new data for a change of its own search
// params (a filter, the sort, the page), not while navigating to another page. The old results
// stay on screen meanwhile; dim them with it so the change visibly registered.
export function useRefreshing() {
	return useRouterState({
		select: (s) =>
			s.isLoading && s.location.pathname === s.resolvedLocation?.pathname,
	});
}
