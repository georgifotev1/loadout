import { queryOptions } from "@tanstack/react-query";
import { apiUrl, toApiError } from "./client";

// Mirrors healthResponse in cmd/loadout/handlers_health.go.
// TODO(phase-4): generated from api/openapi.yaml instead of hand-written.
export type Health = {
	status: "ok" | "degraded";
	version: string;
	env: string;
	database: "ok" | "unavailable";
	time: string;
};

export const healthQuery = queryOptions({
	queryKey: ["health"],
	// healthz answers 503 with a Health body when the database is down. That's data to show,
	// not an error, so this can't use getJSON (which throws on every non-2xx).
	queryFn: async () => {
		const res = await fetch(apiUrl("/api/healthz"), {
			headers: { Accept: "application/json" },
		});
		const body = await res.json().catch(() => undefined);
		if (res.ok || (res.status === 503 && body?.status === "degraded")) {
			return body as Health;
		}
		throw toApiError(res, body);
	},
	retry: false,
	// Keeps the SSR result fresh through hydration so the browser doesn't refetch it right away.
	staleTime: 30_000,
});
