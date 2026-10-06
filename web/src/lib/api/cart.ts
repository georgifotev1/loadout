import { queryOptions } from "@tanstack/react-query";
import { api, type Schemas, unwrap } from "./client";

export type Cart = Schemas["Cart"];
export type CartLine = Schemas["CartLine"];
export type CartAddLine = Schemas["CartAdd"]["lines"][number];

// The most of one product a cart takes (the API's limit too), for the quantity controls.
export const maxQuantity = 10;

// The cart belongs to the browser's cookie, so it's only fetched in the browser: a page rendered
// on the server (and maybe cached) never carries a shopper's cart.
export const cartQuery = queryOptions({
	queryKey: ["cart"],
	queryFn: async ({ signal }) => unwrap(await api.GET("/cart", { signal })),
});

export async function addToCart(lines: CartAddLine[]) {
	return unwrap(await api.POST("/cart/lines", { body: { lines } }));
}

export async function setCartQuantity(productId: string, quantity: number) {
	return unwrap(
		await api.PUT("/cart/lines/{product_id}", {
			params: { path: { product_id: productId } },
			body: { quantity },
		}),
	);
}

export async function removeFromCart(productId: string) {
	return unwrap(
		await api.DELETE("/cart/lines/{product_id}", {
			params: { path: { product_id: productId } },
		}),
	);
}
