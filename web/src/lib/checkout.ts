import { m } from "#/i18n";
import { ApiError } from "#/lib/api/client";

// The checkout form's fields, by the names the API's field errors use.
export type CheckoutField =
	| "name"
	| "email"
	| "phone"
	| "city"
	| "postcode"
	| "address";

export type CheckoutValues = Record<CheckoutField, string>;

/** The message for an input error on a checkout field, if err has one. */
export function checkoutFieldError(
	err: unknown,
	field: CheckoutField,
): string | undefined {
	if (!(err instanceof ApiError)) return undefined;
	return fieldMessage(err.fields.find((f) => f.field === field)?.code);
}

/**
 * Checks the trimmed checkout fields by the API's rules and returns the message for each wrong
 * one, so the customer sees every mistake at once, in Bulgarian, before a round trip. The API
 * checks again and decides.
 */
export function checkCheckoutFields(
	v: CheckoutValues,
): Partial<Record<CheckoutField, string>> {
	const codes: Partial<Record<CheckoutField, string>> = {};
	const text = (f: CheckoutField, max: number) => {
		if (v[f] === "") codes[f] = "required";
		else if ([...v[f]].length > max) codes[f] = "too_long";
	};
	text("name", 100);
	if (v.email === "") codes.email = "required";
	else if (!email(v.email)) codes.email = "invalid_email";
	if (v.phone === "") codes.phone = "required";
	else if (!phone(v.phone)) codes.phone = "invalid_phone";
	text("city", 100);
	if (v.postcode === "") codes.postcode = "required";
	else if (!/^[0-9]{4}$/.test(v.postcode)) codes.postcode = "invalid_postcode";
	text("address", 200);

	const out: Partial<Record<CheckoutField, string>> = {};
	for (const [f, code] of Object.entries(codes)) {
		out[f as CheckoutField] = fieldMessage(code);
	}
	return out;
}

// email is the API's shape check: one @, not at either end. Whether it works shows when we send.
function email(s: string) {
	return (
		s.length <= 254 &&
		s.split("@").length === 2 &&
		!s.startsWith("@") &&
		!s.endsWith("@")
	);
}

// phone is the API's: 9 to 15 digits, with spaces, dashes, dots, brackets and a leading +.
function phone(s: string) {
	if (!/^\+?[0-9 ().-]*$/.test(s)) return false;
	const digits = s.replace(/[^0-9]/g, "").length;
	return digits >= 9 && digits <= 15;
}

function fieldMessage(code: string | undefined): string | undefined {
	switch (code) {
		case undefined:
			return undefined;
		case "required":
			return m.auth.fields.required;
		case "invalid_email":
			return m.auth.fields.invalidEmail;
		case "invalid_phone":
			return m.checkout.fields.invalidPhone;
		case "invalid_postcode":
			return m.checkout.fields.invalidPostcode;
		case "too_long":
			return m.checkout.fields.tooLong;
		default:
			return m.checkout.errors.generic;
	}
}

/**
 * The message for a failed checkout that isn't about a field the customer typed: the cart or
 * its prices changed, or something broke. undefined when the fields say it all.
 */
export function checkoutError(err: unknown): string | undefined {
	if (!err) return undefined;
	if (!(err instanceof ApiError) || err.code !== "validation_failed") {
		return m.checkout.errors.generic;
	}
	const code = (field: string) =>
		err.fields.find((f) => f.field === field)?.code;
	if (code("cart") === "empty") return m.checkout.errors.empty;
	if (code("cart") === "unavailable") return m.checkout.errors.unavailable;
	if (code("expected_total_cents") === "changed")
		return m.checkout.errors.priceChanged;
	if (code("payment")) return m.checkout.errors.generic;
	return undefined;
}

/** Whether the failure means the cart on screen is out of date, so it should load again. */
export function cartChanged(err: unknown): boolean {
	return (
		err instanceof ApiError &&
		err.fields.some(
			(f) => f.field === "cart" || f.field === "expected_total_cents",
		)
	);
}
