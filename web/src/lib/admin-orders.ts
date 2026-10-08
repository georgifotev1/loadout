import { wholeNumber } from "#/lib/listing";
import type { AdminOrdersParams } from "./api/admin-orders";
import type { OrderStatus } from "./api/orders";

export const ordersPageSize = 50;

export const orderStatuses: OrderStatus[] = [
	"awaiting_payment",
	"supplier_pending",
	"ready_to_ship",
	"shipped",
	"delivered",
	"refunding",
	"cancelled",
];

// OrdersView picks the admin's orders: the ones waiting for them, or one status. Absent: all.
export type OrdersView = "needs_action" | OrderStatus;

// OrdersSearch is the admin order list's URL state. Defaults are left out.
export type OrdersSearch = {
	q?: string;
	view?: OrdersView;
	page?: number;
};

const isView = (v: unknown): v is OrdersView =>
	v === "needs_action" || orderStatuses.includes(v as OrderStatus);

// parseOrdersSearch reads OrdersSearch from untrusted URL params; anything malformed is dropped.
export function parseOrdersSearch(s: Record<string, unknown>): OrdersSearch {
	const out: OrdersSearch = {};
	if (typeof s.q === "string" && s.q.trim()) out.q = s.q.trim().slice(0, 200);
	if (isView(s.view)) out.view = s.view;
	const page = wholeNumber(s.page, 2);
	if (page !== undefined) out.page = page;
	return out;
}

// ordersParams turns the URL state into the API's query.
export function ordersParams(s: OrdersSearch): AdminOrdersParams {
	const { view, ...rest } = s;
	return {
		...rest,
		...(view === "needs_action"
			? { needs_action: true }
			: view
				? { status: view }
				: {}),
		page_size: ordersPageSize,
	};
}
