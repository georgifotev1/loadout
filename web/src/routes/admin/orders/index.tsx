import { useSuspenseQuery } from "@tanstack/react-query";
import { createFileRoute, Link, useNavigate } from "@tanstack/react-router";
import { AdminPage } from "#/components/admin/admin-page";
import { AdminError, AdminNotFound } from "#/components/admin/admin-status";
import { adminColumns, DataTable } from "#/components/admin/data-table";
import { OrderStatusDot } from "#/components/admin/order-status";
import { SearchInput } from "#/components/admin/search-input";
import { Pagination } from "#/components/site/pagination";
import {
	Select,
	SelectContent,
	SelectItem,
	SelectTrigger,
	SelectValue,
} from "#/components/ui/select";
import { formatDateTime, formatPrice, formatRelativeTime, m } from "#/i18n";
import {
	orderStatuses,
	ordersPageSize,
	ordersParams,
	parseOrdersSearch,
} from "#/lib/admin-orders";
import {
	type AdminOrderSummary,
	adminOrdersQuery,
} from "#/lib/api/admin-orders";

// /admin/orders: every order, or the ones waiting for an admin (the supplier's answer, a
// refund), the longest waiting first.
export const Route = createFileRoute("/admin/orders/")({
	validateSearch: parseOrdersSearch,
	loaderDeps: ({ search }) => search,
	loader: ({ context, deps }) =>
		context.queryClient.query(adminOrdersQuery(ordersParams(deps))),
	head: () => ({ meta: [{ title: m.site.title(m.admin.orders.title) }] }),
	errorComponent: AdminError,
	notFoundComponent: AdminNotFound,
	component: OrdersPage,
});

const t = m.admin.orders;

const col = adminColumns<AdminOrderSummary>();
const columns = col.columns([
	col.accessor("number", {
		header: t.order,
		cell: (c) => (
			<div className="flex flex-col gap-0.5">
				<Link
					to="/admin/orders/$orderId"
					params={{ orderId: c.row.original.id }}
					className="font-medium tabular-nums hover:underline"
				>
					№ {c.getValue()}
				</Link>
				<span className="text-subtle-foreground text-xs tabular-nums">
					{formatDateTime(new Date(c.row.original.created_at))}
				</span>
			</div>
		),
	}),
	col.accessor("name", {
		header: t.customer,
		cell: (c) => (
			<div className="flex flex-col gap-0.5">
				<span>{c.getValue()}</span>
				<span className="text-subtle-foreground text-xs">
					{c.row.original.email}
				</span>
			</div>
		),
		meta: { className: "w-full min-w-56 whitespace-normal" },
	}),
	col.accessor("status", {
		header: t.status,
		cell: (c) => (
			<div className="flex flex-col gap-0.5">
				<OrderStatusDot status={c.getValue()} stale={c.row.original.stale} />
				<span
					className="pl-3.5 text-subtle-foreground text-xs"
					suppressHydrationWarning
				>
					{formatRelativeTime(new Date(c.row.original.status_since))}
				</span>
			</div>
		),
	}),
	col.accessor("payment", {
		header: t.payment,
		cell: (c) => (
			<div className="flex flex-col gap-0.5">
				<span>{t.payments[c.getValue()]}</span>
				<span className="text-subtle-foreground text-xs">
					{c.row.original.paid_at ? t.paid : t.unpaid}
				</span>
			</div>
		),
	}),
	col.accessor("total", {
		header: t.total,
		cell: (c) => (
			<div className="flex flex-col gap-0.5">
				<span className="font-medium">
					{formatPrice(c.getValue().amount_cents)}
				</span>
				<span className="text-subtle-foreground text-xs">
					{t.units(c.row.original.count)}
				</span>
			</div>
		),
		meta: { className: "text-right tabular-nums" },
	}),
]);

const viewItems = [
	{ value: "", label: t.allOrders },
	{ value: "needs_action", label: t.needsAction },
	...orderStatuses.map((s) => ({ value: s, label: t.statuses[s] })),
];

function OrdersPage() {
	const search = Route.useSearch();
	const navigate = useNavigate({ from: Route.fullPath });
	const { data } = useSuspenseQuery(adminOrdersQuery(ordersParams(search)));

	// A filter change starts again from page 1.
	const filter = (patch: Record<string, unknown>) =>
		navigate({
			search: (prev) =>
				parseOrdersSearch({ ...prev, ...patch, page: undefined }),
		});

	return (
		<AdminPage title={t.title} lead={t.lead}>
			<div className="mb-3 flex flex-wrap items-center gap-2">
				<SearchInput
					value={search.q ?? ""}
					onChange={(q) => filter({ q })}
					label={t.search}
				/>
				<Select
					items={viewItems}
					value={search.view ?? ""}
					onValueChange={(view) => filter({ view })}
				>
					<SelectTrigger className="w-52 bg-card" aria-label={t.view}>
						<SelectValue />
					</SelectTrigger>
					<SelectContent>
						{viewItems.map((v) => (
							<SelectItem key={v.value} value={v.value}>
								{v.label}
							</SelectItem>
						))}
					</SelectContent>
				</Select>
				<p className="ml-auto text-muted-foreground text-sm">
					{t.count(data.total)}
				</p>
			</div>
			<DataTable
				columns={columns}
				data={data.items}
				getRowId={(o) => o.id}
				empty={search.view === "needs_action" ? t.emptyNeedsAction : t.empty}
			/>
			<Pagination
				page={data.page}
				last={Math.ceil(data.total / ordersPageSize)}
				className="mt-6"
			/>
		</AdminPage>
	);
}
