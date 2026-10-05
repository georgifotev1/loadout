import { QueryClient } from "@tanstack/react-query";

export function getContext() {
	const queryClient = new QueryClient({
		defaultOptions: {
			// Catalog data changes on the supplier sync, not by the second. A minute also keeps
			// what the server rendered from being fetched again as soon as the page hydrates.
			queries: { staleTime: 60_000 },
		},
	});

	return {
		queryClient,
	};
}
export default function TanstackQueryProvider() {}
