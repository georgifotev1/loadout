/** Where server-side code (SSR, the Vite dev proxy) reaches the Go API. */
export function apiInternalUrl(): string {
	return process.env.API_INTERNAL_URL || "http://localhost:8080";
}
