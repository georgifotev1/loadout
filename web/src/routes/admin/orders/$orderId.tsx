import { useSuspenseQuery } from "@tanstack/react-query";
import { createFileRoute, Link, notFound } from "@tanstack/react-router";
import { ArrowLeft, ExternalLink } from "lucide-react";
import { AdminPage } from "#/components/admin/admin-page";
import { AdminError, AdminNotFound } from "#/components/admin/admin-status";
import { OrderActions } from "#/components/admin/order-actions";
import { OrderStatusDot } from "#/components/admin/order-status";
import { Button } from "#/components/ui/button";
import {
	Table,
	TableBody,
	TableCell,
	TableHead,
	TableHeader,
	TableRow,
} from "#/components/ui/table";
import { formatDateTime, formatPrice, formatRelativeTime, m } from "#/i18n";
import { type AdminOrder, adminOrderQuery } from "#/lib/api/admin-orders";
import { ApiError } from "#/lib/api/client";
import { cn } from "#/lib/utils";

// /admin/orders/:orderId: one order with what we pay the supplier, its history, and what the
// admin can do next.
export const Route = createFileRoute("/admin/orders/$orderId")({
	loader: async ({ context, params }) => {
		try {
			return await context.queryClient.query(adminOrderQuery(params.orderId));
		} catch (err) {
			// 404: no such order; 422: not an order ID at all.
			if (
				err instanceof ApiError &&
				(err.status === 404 || err.status === 422)
			) {
				throw notFound();
			}
			throw err;
		}
	},
	head: ({ loaderData }) => ({
		meta: [
			{
				title: m.site.title(
					loaderData
						? m.admin.orders.orderTitle(loaderData.number)
						: m.admin.orders.title,
				),
			},
		],
	}),
	errorComponent: AdminError,
	notFoundComponent: AdminNotFound,
	component: OrderPage,
});

const t = m.admin.orders;

function OrderPage() {
	const { orderId } = Route.useParams();
	const { data: o } = useSuspenseQuery(adminOrderQuery(orderId));

	return (
		<AdminPage
			title={t.orderTitle(o.number)}
			lead={t.placedOn(formatDateTime(new Date(o.created_at)))}
			actions={
				<>
					<Button
						variant="ghost"
						size="sm"
						render={<Link to="/admin/orders" search={{ view: undefined }} />}
					>
						<ArrowLeft />
						{t.back}
					</Button>
					{o.token ? (
						<Button
							variant="outline"
							size="sm"
							render={
								<Link
									to="/order/$id"
									params={{ id: o.id }}
									search={{ token: o.token }}
									target="_blank"
									rel="noreferrer"
								/>
							}
						>
							<ExternalLink />
							{t.customerPage}
						</Button>
					) : null}
				</>
			}
		>
			<StatusPanel order={o} />
			<div className="mt-6 grid gap-6 xl:grid-cols-3">
				<div className="flex min-w-0 flex-col gap-6 xl:col-span-2">
					<Lines order={o} />
					<History order={o} />
				</div>
				<aside className="grid content-start gap-6 sm:grid-cols-3 xl:grid-cols-1">
					<Panel title={t.contact}>
						<p>{o.name}</p>
						<p>
							<a href={`mailto:${o.email}`} className="hover:underline">
								{o.email}
							</a>
						</p>
						<p>
							<a href={`tel:${o.phone}`} className="hover:underline">
								{o.phone}
							</a>
						</p>
						<p className="mt-1 text-muted-foreground text-xs">
							{o.user_id ? t.account : t.guest}
						</p>
					</Panel>
					<Panel title={t.address}>
						<p>{o.address}</p>
						<p>
							{o.postcode} {o.city}
						</p>
					</Panel>
					<Panel title={t.paymentTitle}>
						<p>{t.payments[o.payment]}</p>
						<p className="text-muted-foreground">
							{o.paid_at
								? t.paidOn(formatDateTime(new Date(o.paid_at)))
								: t.unpaid}
						</p>
					</Panel>
				</aside>
			</div>
		</AdminPage>
	);
}

// StatusPanel says where the order is, what it waits for, and the admin's actions.
function StatusPanel({ order: o }: { order: AdminOrder }) {
	const todo = o.stale
		? t.todo.supplier_pending_stale
		: (t.todo[o.status] ?? null);
	// The same statuses as the list's "needs action" view.
	const waiting = o.status === "supplier_pending" || o.status === "refunding";
	return (
		<section
			className={cn(
				"flex flex-col gap-4 rounded-lg border bg-card p-5",
				waiting && "border-l-4",
				waiting && (o.stale ? "border-l-destructive" : "border-l-warning"),
			)}
		>
			<div className="max-w-2xl">
				<p className="flex flex-wrap items-center gap-x-2 font-medium">
					<OrderStatusDot status={o.status} stale={o.stale} />
					<span
						className="font-normal text-muted-foreground text-sm"
						suppressHydrationWarning
					>
						{t.since(formatRelativeTime(new Date(o.status_since)))}
					</span>
				</p>
				{todo ? (
					<p className="mt-2 text-muted-foreground text-sm">{todo}</p>
				) : null}
			</div>
			<OrderActions order={o} />
		</section>
	);
}

function Lines({ order: o }: { order: AdminOrder }) {
	return (
		<Panel title={t.lines} flush>
			<Table>
				<TableHeader>
					<TableRow>
						<TableHead>{t.product}</TableHead>
						<TableHead>{t.supplierSku}</TableHead>
						<TableHead className="text-right">{t.quantity}</TableHead>
						<TableHead className="text-right">{t.dealer}</TableHead>
						<TableHead className="text-right">{t.retail}</TableHead>
						<TableHead className="text-right">{t.lineTotal}</TableHead>
					</TableRow>
				</TableHeader>
				<TableBody>
					{o.lines.map((l) => (
						<TableRow key={l.product_id}>
							<TableCell className="min-w-56 whitespace-normal">
								{l.name}
							</TableCell>
							<TableCell className="font-mono text-xs">
								{l.sku}
								<span className="ml-1.5 font-sans text-subtle-foreground">
									{l.provider}
								</span>
							</TableCell>
							<TableCell className="text-right tabular-nums">
								{l.quantity}
							</TableCell>
							<TableCell className="text-right text-muted-foreground tabular-nums">
								{formatPrice(l.dealer_unit.amount_cents)}
							</TableCell>
							<TableCell className="text-right tabular-nums">
								{formatPrice(l.unit_price.amount_cents)}
							</TableCell>
							<TableCell className="text-right font-medium tabular-nums">
								{formatPrice(l.line_total.amount_cents)}
							</TableCell>
						</TableRow>
					))}
				</TableBody>
			</Table>
			<dl className="ml-auto flex max-w-sm flex-col gap-2 border-t p-5 text-sm">
				<Sum label={t.subtotal} cents={o.subtotal.amount_cents} />
				<Sum label={t.delivery} cents={o.delivery.amount_cents} />
				<Sum label={t.grandTotal} cents={o.total.amount_cents} strong />
				<div className="mt-2 flex flex-col gap-2 border-t pt-3">
					<Sum
						label={t.dealerTotal}
						cents={o.dealer_total.amount_cents}
						muted
					/>
					<Sum label={t.margin} cents={o.margin.amount_cents} muted />
				</div>
			</dl>
		</Panel>
	);
}

function Sum({
	label,
	cents,
	strong = false,
	muted = false,
}: {
	label: string;
	cents: number;
	strong?: boolean;
	muted?: boolean;
}) {
	return (
		<div
			className={cn(
				"flex justify-between gap-4",
				muted && "text-muted-foreground",
			)}
		>
			<dt className={cn(strong && "font-medium")}>{label}</dt>
			<dd className={cn("tabular-nums", strong && "font-semibold")}>
				{formatPrice(cents)}
			</dd>
		</div>
	);
}

function History({ order: o }: { order: AdminOrder }) {
	return (
		<Panel title={t.history}>
			<ol className="flex flex-col gap-4">
				{o.transitions.map((tr) => (
					<li key={tr.seq} className="flex gap-3 text-sm">
						<span
							aria-hidden
							className="mt-1.5 size-2 shrink-0 rounded-full bg-border-strong"
						/>
						<div className="min-w-0">
							<p className="font-medium">{t.events[tr.event] ?? tr.event}</p>
							<p className="text-muted-foreground text-xs">
								{formatDateTime(new Date(tr.at))} · {t.by(tr.actor || t.system)}
								{tr.from !== tr.to
									? ` · ${t.statuses[tr.from]} → ${t.statuses[tr.to]}`
									: null}
							</p>
							{tr.note ? (
								<p className="mt-1 whitespace-pre-wrap break-words">
									{tr.note}
								</p>
							) : null}
						</div>
					</li>
				))}
			</ol>
		</Panel>
	);
}

function Panel({
	title,
	flush = false,
	children,
}: {
	title: string;
	flush?: boolean;
	children: React.ReactNode;
}) {
	return (
		<section className="rounded-lg border bg-card">
			<h2 className="border-b px-5 py-3 font-medium text-sm">{title}</h2>
			<div className={cn(!flush && "flex flex-col gap-0.5 p-5 text-sm")}>
				{children}
			</div>
		</section>
	);
}
