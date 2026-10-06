import { createIsomorphicFn } from "@tanstack/react-start";
import { getRequestHeader } from "@tanstack/react-start/server";
import createClient from "openapi-fetch";
import { apiInternalUrl } from "./internal-url";
import type { components, paths } from "./schema.gen";

// Types come from api/openapi.yaml (`make gen`), never written by hand.
export type Schemas = components["schemas"];
export type ErrorCode = Schemas["ErrorCode"];
export type FieldError = Schemas["FieldError"];

/**
 * The API's base URL. In the browser we call the same origin (`/api`), which the Vite dev proxy
 * (and the load balancer in production) routes to the Go API. On the server during SSR there is
 * no origin, so we call the API directly at API_INTERNAL_URL.
 */
export function apiBaseUrl(isServer = typeof window === "undefined"): string {
	return isServer ? new URL("/api", apiInternalUrl()).toString() : "/api";
}

export const api = createClient<paths>({
	baseUrl: apiBaseUrl(),
	headers: { Accept: "application/json" },
});

// During SSR the API call comes from this server, not the browser, so it would go out without
// the session cookie. Pass on the cookie of the page request being rendered. In the browser the
// build drops the server branch, and fetch sends the cookie itself (same origin).
const incomingCookie = createIsomorphicFn()
	.server(() => getRequestHeader("cookie"))
	.client(() => undefined);

api.use({
	onRequest({ request }) {
		const cookie = incomingCookie();
		if (cookie) request.headers.set("cookie", cookie);
	},
});

/**
 * A non-2xx API response. `status` is the broad category (401 → login, 5xx → generic message),
 * `code` is the stable reason to translate, and `message` is for developers only: never show it.
 */
export class ApiError extends Error {
	constructor(
		readonly status: number,
		readonly code: ErrorCode | "http_error",
		message: string,
		readonly fields: FieldError[] = [],
		readonly requestId?: string,
		/** Seconds until another try is allowed, from Retry-After on a 429. */
		readonly retryAfter?: number,
	) {
		super(message);
		this.name = "ApiError";
	}
}

/** Turns an error response into an ApiError. Bodies that aren't API errors become "http_error". */
export function toApiError(res: Response, body: unknown): ApiError {
	const detail = (body as Partial<Schemas["Error"]> | undefined)?.error;
	return new ApiError(
		res.status,
		detail?.code ?? "http_error",
		detail?.message ?? res.statusText,
		detail?.fields,
		res.headers.get("X-Request-Id") ?? undefined,
		Number(res.headers.get("Retry-After")) || undefined,
	);
}

/** Returns the data of an openapi-fetch result, or throws its error as an ApiError. */
export function unwrap<T>(result: {
	data?: T;
	error?: unknown;
	response: Response;
}): T {
	if (!result.response.ok || result.data === undefined) {
		throw toApiError(result.response, result.error);
	}
	return result.data;
}
