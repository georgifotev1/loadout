import { afterEach, describe, expect, it, vi } from "vitest";
import { apiUrl, toApiError } from "./client";

describe("apiUrl", () => {
	afterEach(() => {
		vi.unstubAllEnvs();
	});

	it("uses a same-origin path in the browser", () => {
		expect(apiUrl("/api/healthz", false)).toBe("/api/healthz");
	});

	it("uses API_INTERNAL_URL on the server", () => {
		vi.stubEnv("API_INTERNAL_URL", "http://api:9000");
		expect(apiUrl("/api/healthz", true)).toBe("http://api:9000/api/healthz");
	});

	it("falls back to localhost:8080 on the server", () => {
		vi.stubEnv("API_INTERNAL_URL", "");
		expect(apiUrl("/api/healthz", true)).toBe(
			"http://localhost:8080/api/healthz",
		);
	});

	it("rejects relative paths", () => {
		expect(() => apiUrl("api/healthz", false)).toThrow();
	});
});

describe("toApiError", () => {
	it("reads code, fields and request id from an error body", () => {
		const res = new Response(null, {
			status: 422,
			headers: { "X-Request-Id": "abc123" },
		});
		const err = toApiError(res, {
			error: {
				code: "validation_failed",
				message: "email: invalid_email",
				fields: [{ field: "email", code: "invalid_email" }],
			},
		});
		expect(err.status).toBe(422);
		expect(err.code).toBe("validation_failed");
		expect(err.fields).toEqual([{ field: "email", code: "invalid_email" }]);
		expect(err.requestId).toBe("abc123");
	});

	it("falls back to http_error when the body is not an API error", () => {
		const res = new Response(null, { status: 502, statusText: "Bad Gateway" });
		const err = toApiError(res, undefined);
		expect(err.code).toBe("http_error");
		expect(err.message).toBe("Bad Gateway");
		expect(err.fields).toEqual([]);
	});
});
