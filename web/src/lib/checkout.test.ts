import { describe, expect, it } from "vitest";
import { m } from "#/i18n";
import { ApiError } from "#/lib/api/client";
import { cartChanged, checkoutError, checkoutFieldError } from "./checkout";

const invalid = (...fields: [string, string][]) =>
	new ApiError(
		422,
		"validation_failed",
		"validation failed",
		fields.map(([field, code]) => ({ field, code })),
	);

describe("checkoutFieldError", () => {
	it("names what's wrong with the field", () => {
		const err = invalid(["phone", "invalid_phone"], ["name", "required"]);
		expect(checkoutFieldError(err, "phone")).toBe(
			m.checkout.fields.invalidPhone,
		);
		expect(checkoutFieldError(err, "name")).toBe(m.auth.fields.required);
		expect(checkoutFieldError(err, "city")).toBeUndefined();
	});
});

describe("checkoutError", () => {
	it("says nothing when the fields say it all", () => {
		expect(checkoutError(null)).toBeUndefined();
		expect(checkoutError(invalid(["phone", "invalid_phone"]))).toBeUndefined();
	});

	it("explains a changed cart", () => {
		const err = invalid(["expected_total_cents", "changed"]);
		expect(checkoutError(err)).toBe(m.checkout.errors.priceChanged);
		expect(cartChanged(err)).toBe(true);
		expect(checkoutError(invalid(["cart", "unavailable"]))).toBe(
			m.checkout.errors.unavailable,
		);
		expect(cartChanged(invalid(["phone", "required"]))).toBe(false);
	});

	it("falls back to a generic message", () => {
		expect(checkoutError(new ApiError(500, "internal", "boom"))).toBe(
			m.checkout.errors.generic,
		);
		expect(checkoutError(new Error("network"))).toBe(m.checkout.errors.generic);
	});
});
