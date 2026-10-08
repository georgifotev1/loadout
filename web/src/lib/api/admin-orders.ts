import { queryOptions } from "@tanstack/react-query";
import { api, type Schemas, unwrap } from "./client";
import type { operations } from "./schema.gen";

export type AdminOrder = Schemas["AdminOrder"];
export type AdminOrderSummary = Schemas["AdminOrderSummary"];
export type AdminOrderEvent = Schemas["AdminOrderEvent"];
export type AdminOrdersParams = NonNullable<
	operations["listAdminOrders"]["parameters"]["query"]
>;

export const adminOrdersQuery = (params: AdminOrdersParams) =>
	queryOptions({
		queryKey: ["admin", "orders", "list", params],
		queryFn: async ({ signal }) =>
			unwrap(
				await api.GET("/admin/orders", { params: { query: params }, signal }),
			),
	});

export const adminOrderQuery = (id: string) =>
	queryOptions({
		queryKey: ["admin", "orders", id],
		queryFn: async ({ signal }) =>
			unwrap(
				await api.GET("/admin/orders/{order_id}", {
					params: { path: { order_id: id } },
					signal,
				}),
			),
	});

/** Fails with `invalid_transition` when the order can't take the event any more. */
export async function applyOrderEvent(
	id: string,
	body: Schemas["AdminOrderEventInput"],
) {
	return unwrap(
		await api.POST("/admin/orders/{order_id}/events", {
			params: { path: { order_id: id } },
			body,
		}),
	);
}
