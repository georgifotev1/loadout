import { hashKey, type QueryClient, queryOptions } from "@tanstack/react-query";
import { cartQuery } from "./cart";
import { ApiError, api, type Schemas, toApiError, unwrap } from "./client";

export type Me = Schemas["Me"];

/** The logged-in user, or null without a session. */
export const meQuery = queryOptions({
	queryKey: ["auth", "me"],
	queryFn: async ({ signal }): Promise<Me | null> => {
		try {
			return unwrap(await api.GET("/auth/me", { signal }));
		} catch (err) {
			if (err instanceof ApiError && err.status === 401) return null;
			throw err;
		}
	},
});

const meQueryHash = hashKey(meQuery.queryKey);
const cartQueryHash = hashKey(cartQuery.queryKey);

/** Whether the session is done logging in: no second factor left to enter or set up. */
export function signedIn(me: Me | null): me is Me {
	return (
		me !== null &&
		(me.second_factor === "verified" || me.second_factor === "not_required")
	);
}

export async function login(body: Schemas["LoginRequest"]) {
	return unwrap(await api.POST("/auth/login", { body }));
}

export async function logout(queryClient: QueryClient) {
	const { response } = await api.POST("/auth/logout");
	if (!response.ok) throw new ApiError(response.status, "http_error", "");
	queryClient.setQueryData(meQuery.queryKey, null);
	// Anything else cached may have been the logged-in user's. The cart is in use on every shop
	// page, so it's fetched again (empty: it stays with the account) rather than dropped.
	queryClient.removeQueries({
		predicate: (q) =>
			q.queryHash !== meQueryHash && q.queryHash !== cartQueryHash,
	});
	void queryClient.invalidateQueries({ queryKey: cartQuery.queryKey });
}

/**
 * After a login (or a password reset, which logs in): remember the user, and fetch the cart
 * again, since the API moved the guest cart into the account's.
 */
export function loggedIn(queryClient: QueryClient, me: Me) {
	queryClient.setQueryData(meQuery.queryKey, me);
	void queryClient.invalidateQueries({ queryKey: cartQuery.queryKey });
}

/** Signs up. The answer is the same whether or not the email is taken: check your email. */
export async function register(body: Schemas["RegisterRequest"]) {
	const { response, error } = await api.POST("/auth/register", { body });
	if (!response.ok) throw toApiError(response, error);
}

export async function verifyEmail(token: string) {
	const { response, error } = await api.POST("/auth/verify-email", {
		body: { token },
	});
	if (!response.ok) throw toApiError(response, error);
}

export async function resendVerification() {
	const { response, error } = await api.POST("/auth/verify-email/resend");
	if (!response.ok) throw toApiError(response, error);
}

export async function forgotPassword(email: string) {
	const { response, error } = await api.POST("/auth/password/forgot", {
		body: { email },
	});
	if (!response.ok) throw toApiError(response, error);
}

export async function resetPassword(body: Schemas["ResetPasswordRequest"]) {
	return unwrap(await api.POST("/auth/password/reset", { body }));
}

export async function changePassword(body: Schemas["ChangePasswordRequest"]) {
	return unwrap(await api.POST("/auth/password", { body }));
}

// Each call makes a new secret and replaces the unconfirmed one, so this is a query: it runs once
// per setup screen, not again on a re-render or a second mount in StrictMode.
export const totpEnrollQuery = queryOptions({
	queryKey: ["auth", "totp", "enroll"],
	queryFn: async () => unwrap(await api.POST("/auth/totp/enroll")),
	staleTime: Number.POSITIVE_INFINITY,
	gcTime: 0,
	retry: false,
	refetchOnWindowFocus: false,
});

export async function confirmTotp(code: string) {
	return unwrap(await api.POST("/auth/totp/confirm", { body: { code } }));
}

export async function verifyTotp(code: string) {
	return unwrap(await api.POST("/auth/totp/verify", { body: { code } }));
}

/** Replaces the admin's recovery codes; the old ones stop working. Shown only this once. */
export async function regenerateRecoveryCodes() {
	return unwrap(await api.POST("/auth/totp/recovery-codes"));
}
