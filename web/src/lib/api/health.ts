import { queryOptions } from "@tanstack/react-query";
import { api, type Schemas, toApiError } from "./client";

export type Health = Schemas["Health"];

export const healthQuery = queryOptions({
	queryKey: ["health"],
	queryFn: async ({ signal }) => {
		const { data, error, response } = await api.GET("/healthz", { signal });
		if (data) {
			return data;
		}
		// healthz answers 503 with a Health body when the database is down. That's data to show,
		// not an error.
		if (response.status === 503 && error?.status === "degraded") {
			return error;
		}
		throw toApiError(response, error);
	},
	retry: false,
	// Keeps the SSR result fresh through hydration so the browser doesn't refetch it right away.
	staleTime: 30_000,
});
