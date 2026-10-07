import { useQuery, useQueryClient } from "@tanstack/react-query";
import { createFileRoute, Link, useLocation } from "@tanstack/react-router";
import { Check, CircleX, PackageCheck, Truck } from "lucide-react";
import { CompactLines } from "#/components/order/compact-lines";
import { Totals } from "#/components/order/totals";
import { Container } from "#/components/site/container";
import { PageSkeleton } from "#/components/site/page-skeletons";
import { StatusPage } from "#/components/site/status-page";
import { Button } from "#/components/ui/button";
import { formatDateTime, m } from "#/i18n";
import { meQuery, signedIn } from "#/lib/api/auth";
import { ApiError } from "#/lib/api/client";
import { type Order, type OrderStatus, orderQuery } from "#/lib/api/orders";
import { cn } from "#/lib/utils";

const t = m.order;

// /order/$id: an order for its customer, opened with the token from checkout and the emails, or
// from the account's order history. Private, so it loads in the browser only. The token is in
// the URL, so the page sends no Referer to the product images' hosts.
export const Route = createFileRoute("/_shop/order/$id")({
	validateSearch: (search): { token?: string } =>
		typeof search.token === "string" ? { token: search.token } : {},
	head: () => ({
		meta: [
			{ title: m.site.title(m.checkout.title) },
			{ name: "robots", content: "noindex" },
			{ name: "referrer", content: "no-referrer" },
		],
	}),
	component: OrderPage,
});

function OrderPage() {
	const { id } = Route.useParams();
	const { token } = Route.useSearch();
	const queryClient = useQueryClient();
	const { data: order, error } = useQuery({
		...orderQuery(id, token),
		enabled: typeof window !== "undefined",
		retry: (n, err) =>
			!(err instanceof ApiError && err.status === 404) && n < 3,
	});

	if (error instanceof ApiError && error.status === 404)
		return <OrderNotFound />;
	if (error && !order) {
		return (
			<StatusPage title={m.error.title} detail={m.error.detail}>
				<Button
					size="lg"
					onClick={() =>
						queryClient.refetchQueries({ queryKey: ["orders", id] })
					}
				>
					{m.error.retry}
				</Button>
			</StatusPage>
		);
	}
	if (!order) return <PageSkeleton />;
	return <OrderView order={order} />;
}

function OrderNotFound() {
	const { data: me } = useQuery({
		...meQuery,
		enabled: typeof window !== "undefined",
	});
	const href = useLocation({ select: (l) => l.href });
	return (
		<StatusPage title={t.notFoundTitle} detail={t.notFound}>
			{signedIn(me ?? null) ? (
				<Button size="lg" render={<Link to="/" />}>
					{t.backToShop}
				</Button>
			) : (
				<Button
					size="lg"
					render={<Link to="/login" search={{ redirect: href }} />}
				>
					{m.auth.login.title}
				</Button>
			)}
		</StatusPage>
	);
}

// Where an order on its way is: the step it's at, and the ones done before it.
const steps = [
	{ icon: Check, title: t.stepConfirmed, detail: t.stepConfirmedDetail },
	{ icon: Truck, title: t.stepShipped, detail: t.stepShippedDetail },
	{ icon: PackageCheck, title: t.stepDelivered, detail: t.stepDeliveredDetail },
];

const currentStep: Partial<Record<OrderStatus, number>> = {
	awaiting_payment: 0,
	supplier_pending: 0,
	ready_to_ship: 1,
	shipped: 2,
	delivered: 3,
};

function OrderView({ order }: { order: Order }) {
	const step = currentStep[order.status];
	const received = order.status === "supplier_pending";

	return (
		<Container className="max-w-3xl pt-12 pb-16 sm:pt-16">
			{received ? (
				<div className="mb-6 flex size-12 items-center justify-center rounded-full bg-success/10 text-success">
					<Check className="size-6" strokeWidth={2.25} />
				</div>
			) : null}
			<p className="font-mono text-muted-foreground text-sm">
				{t.title(order.number)}
			</p>
			<h1 className="mt-1 font-semibold text-3xl tracking-[-0.03em] sm:text-4xl">
				{received ? t.received : t.status[order.status]}
			</h1>
			<p className="mt-3 max-w-xl text-muted-foreground">
				{received
					? t.emailSent(order.email)
					: t.placedOn(formatDateTime(new Date(order.created_at)))}
			</p>

			{step === undefined ? (
				<p className="mt-10 flex items-start gap-3 rounded-xl border bg-card p-6 text-sm">
					<CircleX className="mt-0.5 size-4 shrink-0 text-muted-foreground" />
					{t.stopped}
				</p>
			) : (
				<section className="mt-10 rounded-xl border bg-card p-6">
					<h2 className="font-medium">{t.next}</h2>
					<ol className="mt-5 grid gap-5 sm:grid-cols-3">
						{steps.map((s, i) => (
							<li
								key={s.title}
								aria-current={i === step ? "step" : undefined}
								className="flex gap-3 sm:flex-col"
							>
								<span
									className={cn(
										"flex size-8 shrink-0 items-center justify-center rounded-full border",
										i < step && "border-success/40 bg-success/10 text-success",
										i === step &&
											"border-foreground bg-primary text-primary-foreground",
										i > step && "text-muted-foreground",
									)}
								>
									<s.icon className="size-4" />
								</span>
								<div>
									<p className="font-medium text-sm">{s.title}</p>
									<p className="mt-0.5 text-muted-foreground text-meta">
										{s.detail}
									</p>
								</div>
							</li>
						))}
					</ol>
				</section>
			)}

			<section className="mt-4 rounded-xl border bg-card p-6">
				<h2 className="mb-5 font-medium">{t.items}</h2>
				<CompactLines
					lines={order.lines.map((l) => ({
						key: l.product_id,
						name: l.name,
						image: l.image,
						quantity: l.quantity,
						cents: l.line_total.amount_cents,
					}))}
				/>
				<Totals
					subtotal={order.subtotal.amount_cents}
					delivery={order.delivery.amount_cents}
					total={order.total.amount_cents}
					className="mt-6 border-t pt-5"
				/>
			</section>

			<section className="mt-4 grid gap-6 rounded-xl border bg-card p-6 text-sm sm:grid-cols-3">
				<Detail title={t.deliveryTo}>
					<p>{order.name}</p>
					<p>{order.address}</p>
					<p>
						{order.postcode} {order.city}
					</p>
				</Detail>
				<Detail title={t.contact}>
					<p className="truncate">{order.email}</p>
					<p>{order.phone}</p>
				</Detail>
				<Detail title={t.payment}>
					<p>{m.checkout.cashOnDelivery}</p>
					<p className="text-muted-foreground">
						{m.checkout.cashOnDeliveryDetail}
					</p>
				</Detail>
			</section>

			<Button variant="outline" className="mt-8" render={<Link to="/" />}>
				{t.backToShop}
			</Button>
		</Container>
	);
}

function Detail({
	title,
	children,
}: {
	title: string;
	children: React.ReactNode;
}) {
	return (
		<div className="min-w-0">
			<h3 className="mb-2 text-muted-foreground text-xs uppercase tracking-wider">
				{title}
			</h3>
			<div className="flex flex-col gap-0.5">{children}</div>
		</div>
	);
}
