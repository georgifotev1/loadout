import { describe, expect, it } from "vitest";
import { m } from "#/i18n";
import { ApiError } from "#/lib/api/client";
import {
	type CheckoutValues,
	cartChanged,
	checkCheckoutFields,
	checkoutError,
	checkoutFieldError,
} from "./checkout";

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
		expect(
			checkoutFieldError(invalid(["office_id", "unknown"]), "office_id"),
		).toBe(m.checkout.fields.unknownOffice);
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
		expect(checkoutError(invalid(["delivery_method", "unavailable"]))).toBe(
			m.checkout.errors.deliveryUnavailable,
		);
	});

	it("falls back to a generic message", () => {
		expect(checkoutError(new ApiError(500, "internal", "boom"))).toBe(
			m.checkout.errors.generic,
		);
		expect(checkoutError(new Error("network"))).toBe(m.checkout.errors.generic);
	});
});

describe("checkCheckoutFields", () => {
	const valid: CheckoutValues = {
		name: "Иван Иванов",
		email: "ivan@example.bg",
		phone: "+359 (888) 123-456",
		delivery_method: "address",
		office_id: "",
		city: "София",
		postcode: "1000",
		address: "ул. Граф Игнатиев 1",
	};

	it("passes a filled-in form", () => {
		expect(checkCheckoutFields(valid)).toEqual({});
	});

	it("names every mistake at once", () => {
		const f = m.checkout.fields;
		expect(
			checkCheckoutFields({
				name: "",
				email: "ivan",
				phone: "0888 12",
				delivery_method: "address",
				office_id: "",
				city: "С".repeat(101),
				postcode: "10000",
				address: "",
			}),
		).toEqual({
			name: m.auth.fields.required,
			email: m.auth.fields.invalidEmail,
			phone: f.invalidPhone,
			city: f.tooLong,
			postcode: f.invalidPostcode,
			address: m.auth.fields.required,
		});
	});

	it("asks for an office instead of an address", () => {
		const toOffice = {
			...valid,
			delivery_method: "office" as const,
			city: "",
			postcode: "",
			address: "",
		};
		expect(checkCheckoutFields(toOffice)).toEqual({
			office_id: m.checkout.fields.officeRequired,
		});
		expect(checkCheckoutFields({ ...toOffice, office_id: "102" })).toEqual({});
	});

	it("follows the API's rules at the edges", () => {
		const check = (patch: Partial<CheckoutValues>) =>
			checkCheckoutFields({ ...valid, ...patch });
		expect(check({ name: "Я".repeat(100) })).toEqual({});
		expect(check({ phone: "0888123456789012" })).toHaveProperty("phone");
		expect(check({ phone: "0888 123 45x" })).toHaveProperty("phone");
		expect(check({ phone: "359+888123456" })).toHaveProperty("phone");
		expect(check({ email: "a@b@c" })).toHaveProperty("email");
		expect(check({ email: "@example.bg" })).toHaveProperty("email");
		expect(check({ postcode: "1ООО" })).toHaveProperty("postcode");
	});
});
