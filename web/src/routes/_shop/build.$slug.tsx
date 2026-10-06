import { useSuspenseQuery } from "@tanstack/react-query";
import { createFileRoute, Link, notFound } from "@tanstack/react-router";
import { Pencil, ShoppingCart, TriangleAlert } from "lucide-react";
import { BuildSummary } from "#/components/builder/build-summary";
import { BuildSlots } from "#/components/builder/slot-row";
import { Container } from "#/components/site/container";
import { PageBreadcrumb } from "#/components/site/page-breadcrumb";
import { PageSkeleton } from "#/components/site/page-skeletons";
import { StatusPage } from "#/components/site/status-page";
import { Button } from "#/components/ui/button";
import { formatPrice, formatShortDate, m } from "#/i18n";
import { savedBuildQuery } from "#/lib/api/builder";
import { ApiError, type Schemas } from "#/lib/api/client";
import { buildStatus, selectionOf } from "#/lib/builder";

// /build/{slug}: a saved build as its share link shows it, checked again at today's parts and
// prices. Read-only; "edit" opens it in the builder.
export const Route = createFileRoute("/_shop/build/$slug")({
	loader: ({ context, params }) =>
		context.queryClient.query(savedBuildQuery(params.slug)).catch((err) => {
			if (err instanceof ApiError && err.status === 404) throw notFound();
			throw err;
		}),
	head: () => ({
		meta: [
			{ title: m.site.title(m.builder.saved.title) },
			// Shoppers' builds are for sharing, not for search results.
			{ name: "robots", content: "noindex" },
		],
	}),
	notFoundComponent: () => (
		<StatusPage
			title={m.builder.saved.notFoundTitle}
			detail={m.builder.saved.notFoundDetail}
		>
			<Button size="lg" render={<Link to="/builder" />}>
				{m.builder.saved.startNew}
			</Button>
		</StatusPage>
	),
	pendingComponent: PageSkeleton,
	component: SavedBuildPage,
});

function SavedBuildPage() {
	const { slug } = Route.useParams();
	const { data: saved } = useSuspenseQuery(savedBuildQuery(slug));
	const { check } = saved;
	const status = buildStatus(check);

	const edit = (
		<Button
			variant="outline"
			render={<Link to="/builder" search={selectionOf(check)} />}
		>
			<Pencil /> {m.builder.saved.edit}
		</Button>
	);

	return (
		<Container className="pt-6 pb-16">
			<PageBreadcrumb current={m.builder.saved.title} />

			<div className="mt-6 flex flex-wrap items-end justify-between gap-4">
				<div>
					<h1 className="font-semibold text-3xl tracking-[-0.03em]">
						{m.builder.saved.title}
					</h1>
					<p className="mt-2 text-muted-foreground">
						{m.builder.saved.savedOn(
							formatShortDate(new Date(saved.created_at)),
						)}
					</p>
				</div>
				{edit}
			</div>

			<div className="mt-4 flex flex-col gap-2 empty:hidden">
				{saved.price_changes.length ? <PriceChanges saved={saved} /> : null}
				{check.unavailable.length ? (
					<p className="flex gap-2 rounded-xl border border-destructive/40 bg-destructive/[0.03] px-4 py-3 text-destructive text-meta">
						<TriangleAlert className="mt-0.5 size-4 shrink-0" />
						{m.builder.saved.unavailable(check.unavailable.length)}
					</p>
				) : null}
			</div>

			<div className="mt-6 grid gap-8 lg:grid-cols-[1fr_22rem] lg:gap-10">
				<BuildSlots check={check} status={status} />
				<aside className="lg:sticky lg:top-[calc(var(--spacing-header)+2.5rem+1.5rem)] lg:self-start">
					<BuildSummary check={check} status={status}>
						{/* TODO(phase-7): add the build to the cart. */}
						<Button size="lg" className="h-11 w-full" disabled>
							<ShoppingCart /> {m.builder.summary.addToCart}
						</Button>
						<p className="mt-2 text-center text-subtle-foreground text-xs">
							{m.builder.summary.cartSoon}
						</p>
					</BuildSummary>
				</aside>
			</div>
		</Container>
	);
}

function PriceChanges({ saved }: { saved: Schemas["SavedBuild"] }) {
	const name = (slug: string) =>
		saved.check.parts.find((p) => p.product.slug === slug)?.product.name ??
		slug;
	return (
		<div className="rounded-xl border border-warning/60 bg-warning/[0.06] px-4 py-3">
			<p className="flex items-center gap-2 font-medium text-sm">
				<TriangleAlert className="size-4 text-warning" />
				{m.builder.saved.pricesChanged(saved.price_changes.length)}
			</p>
			<ul className="mt-2 flex flex-col gap-1 pl-6 text-meta">
				{saved.price_changes.map((c) => (
					<li key={c.slug} className="flex flex-wrap gap-x-2">
						<span className="line-clamp-1 min-w-0 flex-1">{name(c.slug)}</span>
						<span className="text-muted-foreground tabular-nums">
							{m.builder.saved.wasNow(
								formatPrice(c.saved.amount_cents),
								formatPrice(c.current.amount_cents),
							)}
						</span>
					</li>
				))}
			</ul>
			<p className="mt-2 pl-6 text-subtle-foreground text-xs">
				{m.builder.saved.savedTotal(
					formatPrice(saved.saved_total.amount_cents),
				)}
			</p>
		</div>
	);
}
