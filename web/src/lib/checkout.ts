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

/** The message for an input error on a checkout field, if err has one. */
export function checkoutFieldError(
	err: unknown,
	field: CheckoutField,
): string | undefined {
	if (!(err instanceof ApiError)) return undefined;
	switch (err.fields.find((f) => f.field === field)?.code) {
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
