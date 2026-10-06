import { afterEach, describe, expect, it, vi } from "vitest";
import { apiBaseUrl, toApiError, unwrap } from "./client";

describe("apiBaseUrl", () => {
	afterEach(() => {
		vi.unstubAllEnvs();
	});

	it("uses a same-origin path in the browser", () => {
		expect(apiBaseUrl(false)).toBe("/api");
	});

	it("uses API_INTERNAL_URL on the server", () => {
		vi.stubEnv("API_INTERNAL_URL", "http://api:9000");
		expect(apiBaseUrl(true)).toBe("http://api:9000/api");
	});

	it("falls back to localhost:8080 on the server", () => {
		vi.stubEnv("API_INTERNAL_URL", "");
		expect(apiBaseUrl(true)).toBe("http://localhost:8080/api");
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
				message: "page: out_of_range",
				fields: [{ field: "page", code: "out_of_range" }],
			},
		});
		expect(err.status).toBe(422);
		expect(err.code).toBe("validation_failed");
		expect(err.fields).toEqual([{ field: "page", code: "out_of_range" }]);
		expect(err.requestId).toBe("abc123");
	});

	it("falls back to http_error when the body is not an API error", () => {
		const res = new Response(null, { status: 502, statusText: "Bad Gateway" });
		const err = toApiError(res, "<html>bad gateway</html>");
		expect(err.code).toBe("http_error");
		expect(err.message).toBe("Bad Gateway");
		expect(err.fields).toEqual([]);
	});

	it("reads Retry-After on a 429", () => {
		const res = new Response(null, {
			status: 429,
			headers: { "Retry-After": "30" },
		});
		const err = toApiError(res, { error: { code: "too_many_attempts" } });
		expect(err.retryAfter).toBe(30);
		expect(toApiError(new Response(null, { status: 401 }), {}).retryAfter).toBe(
			undefined,
		);
	});
});

describe("unwrap", () => {
	it("returns the data of a 2xx result", () => {
		const response = new Response(null, { status: 200 });
		expect(unwrap({ data: { ok: true }, response })).toEqual({ ok: true });
	});

	it("throws the error of a non-2xx result", () => {
		const response = new Response(null, { status: 404 });
		expect(() =>
			unwrap({ error: { error: { code: "not_found" } }, response }),
		).toThrow(expect.objectContaining({ status: 404, code: "not_found" }));
	});
});
