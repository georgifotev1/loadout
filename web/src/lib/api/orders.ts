import { queryOptions } from "@tanstack/react-query";
import { api, type Schemas, unwrap } from "./client";

export type Checkout = Schemas["Checkout"];
export type CheckoutRequest = Schemas["CheckoutRequest"];
export type Order = Schemas["Order"];
export type OrderLine = Schemas["OrderLine"];
export type OrderStatus = Schemas["OrderStatus"];
export type OrderSummary = Schemas["OrderSummary"];

// The cart with delivery and the total. Fetched in the browser only, like the cart.
export const checkoutQuery = queryOptions({
	queryKey: ["checkout"],
	queryFn: async ({ signal }) => unwrap(await api.GET("/checkout", { signal })),
});

export async function placeOrder(body: CheckoutRequest) {
	return unwrap(await api.POST("/checkout", { body }));
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
