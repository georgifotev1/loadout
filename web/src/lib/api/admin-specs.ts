import { queryOptions } from "@tanstack/react-query";
import { api, type Schemas, unwrap } from "./client";
import type { operations } from "./schema.gen";

export type ReviewQueueParams = NonNullable<
	operations["listSpecsReviewQueue"]["parameters"]["query"]
>;
export type ProductSpecs = Schemas["ProductSpecs"];
export type SpecField = Schemas["SpecField"];
export type SpecValue = Schemas["SpecValue"];

export const reviewQueueQuery = (params: ReviewQueueParams) =>
	queryOptions({
		queryKey: ["admin", "specs", "queue", params],
		queryFn: async ({ signal }) =>
			unwrap(
				await api.GET("/admin/specs/review-queue", {
					params: { query: params },
					signal,
				}),
			),
	});

export const productSpecsQuery = (productId: string) =>
	queryOptions({
		queryKey: ["admin", "specs", "product", productId],
		queryFn: async ({ signal }) =>
			unwrap(
				await api.GET("/admin/specs/{product_id}", {
					params: { path: { product_id: productId } },
					signal,
				}),
			),
	});

/** Sets admin values; `null` clears one. Returns the product's specs after the change. */
export async function updateProductSpecs(
	productId: string,
	values: Record<string, SpecValue>,
) {
	return unwrap(
		await api.PATCH("/admin/specs/{product_id}", {
			params: { path: { product_id: productId } },
			body: { values },
		}),
	);
}
