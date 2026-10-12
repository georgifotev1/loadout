import { queryOptions } from "@tanstack/react-query";
import { api, type Schemas, unwrap } from "./client";

export type Checkout = Schemas["Checkout"];
export type CheckoutRequest = Schemas["CheckoutRequest"];
export type DeliveryMethod = Schemas["DeliveryMethod"];
export type DeliveryOption = Schemas["DeliveryOption"];
export type Office = Schemas["Office"];
export type Order = Schemas["Order"];
export type OrderLine = Schemas["OrderLine"];
export type OrderStatus = Schemas["OrderStatus"];
export type OrderSummary = Schemas["OrderSummary"];
export type PaymentMethod = Schemas["PaymentMethod"];

// The cart with the ways to deliver it and the total of each. Fetched in the browser only, like the cart.
export const checkoutQuery = queryOptions({
	queryKey: ["checkout"],
	queryFn: async ({ signal }) => unwrap(await api.GET("/checkout", { signal })),
});

// The courier's offices or lockers matching what the shopper typed. The API finds none for
// fewer than two letters, so the query waits for them.
export const officesQuery = (q: string, method: "office" | "apt") =>
	queryOptions({
		queryKey: ["delivery", "offices", method, q],
		queryFn: async ({ signal }) =>
			unwrap(
				await api.GET("/delivery/offices", {
					params: { query: { q, method } },
					signal,
				}),
			),
		enabled: [...q.replace(/\s/g, "")].length >= 2,
		staleTime: 5 * 60_000,
	});

export async function placeOrder(body: CheckoutRequest) {
	return unwrap(await api.POST("/checkout", { body }));
}

// Stripe's payment page for a card order waiting for its payment.
export async function payOrder(id: string, token?: string) {
	return unwrap(
		await api.POST("/orders/{order_id}/payment", {
			params: { path: { order_id: id }, query: token ? { token } : {} },
		}),
	);
}

// Asks Stripe about a card order's payment and returns the order after: the payment counts as
// soon as Stripe says so, without waiting for its webhook.
export async function syncOrderPayment(id: string, token?: string) {
	return unwrap(
		await api.POST("/orders/{order_id}/payment/sync", {
			params: { path: { order_id: id }, query: token ? { token } : {} },
		}),
	);
}

// An order, opened with its link's token or as the logged-in customer it belongs to. Private, so
// it's fetched in the browser only, never in cached server HTML.
export const orderQuery = (id: string, token?: string) =>
	queryOptions({
		queryKey: ["orders", id, token ?? null],
		queryFn: async ({ signal }) =>
			unwrap(
				await api.GET("/orders/{order_id}", {
					params: { path: { order_id: id }, query: token ? { token } : {} },
					signal,
				}),
			),
	});

export const myOrdersQuery = queryOptions({
	queryKey: ["orders", "mine"],
	queryFn: async ({ signal }) => unwrap(await api.GET("/orders", { signal })),
});
