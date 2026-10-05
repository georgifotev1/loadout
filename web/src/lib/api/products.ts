import { queryOptions } from "@tanstack/react-query";
import { api, unwrap } from "./client";
import type { operations } from "./schema.gen";

export type ProductListParams = NonNullable<
	operations["listProducts"]["parameters"]["query"]
>;

export const productsQuery = (params: ProductListParams) =>
	queryOptions({
		queryKey: ["products", params],
		queryFn: async ({ signal }) =>
			unwrap(await api.GET("/products", { params: { query: params }, signal })),
	});

export const productQuery = (slug: string) =>
	queryOptions({
		queryKey: ["product", slug],
		queryFn: async ({ signal }) =>
			unwrap(
				await api.GET("/products/{slug}", {
					params: { path: { slug } },
					signal,
				}),
			),
	});
