// Minimal API client for Phase 0. Replaced by a generated openapi-fetch client in Phase 4.
// TODO(phase-4): openapi-typescript types + openapi-fetch; forward the incoming cookie during SSR.

/** Where server-side code (SSR, the Vite dev proxy) reaches the Go API. */
export function apiInternalUrl(): string {
	return process.env.API_INTERNAL_URL || "http://localhost:8080";
}

/**
 * Resolves an API path. In the browser we call the same origin (`/api/...`), which the Vite
 * dev proxy (and the load balancer in production) routes to the Go API. On the server during
 * SSR there is no origin, so we call the API directly at API_INTERNAL_URL.
 */
export function apiUrl(
	path: string,
	isServer = typeof window === "undefined",
): string {
	if (!path.startsWith("/")) {
		throw new Error(`api path must start with "/": ${path}`);
	}
	if (!isServer) {
		return path;
	}
	return new URL(path, apiInternalUrl()).toString();
}

/** One invalid input, from a 422 "validation_failed" response. `code` is an i18n key. */
export type FieldError = { field: string; code: string };

/**
 * A non-2xx API response. `status` is the broad category (401 → login, 5xx → generic message),
 * `code` is the stable reason to translate, and `message` is for developers only: never show it.
 */
export class ApiError extends Error {
	constructor(
		readonly status: number,
		readonly code: string,
		message: string,
		readonly fields: FieldError[] = [],
		readonly requestId?: string,
	) {
		super(message);
		this.name = "ApiError";
	}
}

/** GETs a JSON endpoint. Non-2xx responses become an ApiError with the API's stable error code. */
export async function getJSON<T>(path: string, init?: RequestInit): Promise<T> {
	const res = await fetch(apiUrl(path), {
		...init,
		headers: { Accept: "application/json", ...init?.headers },
	});
	const body = await res.json().catch(() => undefined);
	if (!res.ok) {
		throw toApiError(res, body);
	}
	return body as T;
}

// Mirrors httpx.ErrorBody in internal/platform/httpx/json.go.
// TODO(phase-4): generated from api/openapi.yaml, with `code` as a union of known codes.
type ErrorBody = {
	error?: { code?: string; message?: string; fields?: FieldError[] };
};

export function toApiError(res: Response, body: unknown): ApiError {
	const detail = (body as ErrorBody | undefined)?.error;
	return new ApiError(
		res.status,
		detail?.code ?? "http_error",
		detail?.message ?? res.statusText,
		detail?.fields,
		res.headers.get("X-Request-Id") ?? undefined,
	);
}
