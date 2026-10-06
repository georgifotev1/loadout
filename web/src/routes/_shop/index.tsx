import { useSuspenseQueries } from "@tanstack/react-query";
import { createFileRoute, Link } from "@tanstack/react-router";
import {
	ArrowRight,
	ArrowUpRight,
	Box,
	CircuitBoard,
	Cpu,
	CreditCard,
	Fan,
	Gpu,
	MemoryStick,
	PlugZap,
	RotateCcw,
	ShieldCheck,
	Truck,
} from "lucide-react";
import { CartProductCard } from "#/components/cart/cart-product-card";
import { Price } from "#/components/product/price";
import { ProductImage } from "#/components/product/product-image";
import { Container } from "#/components/site/container";
import { Button } from "#/components/ui/button";
import { m } from "#/i18n";
import type { Schemas } from "#/lib/api/client";
import { searchQuery } from "#/lib/api/products";
import type { Category } from "#/lib/categories";
import { cn } from "#/lib/utils";

type ProductSummary = Schemas["ProductSummary"];

// The bento shows the priciest product of each of these categories, and the rows below list
// a few of these. TODO: admin-curated picks instead, once products can be featured (deferred
// from Phase 6).
const featuredCategories: Category[] = ["gpu", "cpu", "peripheral.monitor"];
const rowCategories: Category[] = ["gpu", "peripheral.monitor"];

const queries = [
	...featuredCategories.map((category) =>
		searchQuery({ category, sort: "price_desc", page_size: 1 }),
	),
	...rowCategories.map((category) => searchQuery({ category, page_size: 4 })),
];

export const Route = createFileRoute("/_shop/")({
	loader: ({ context }) =>
		Promise.all(queries.map((q) => context.queryClient.query(q))),
	head: () => ({
		meta: [
			{ title: m.site.title(m.site.tagline) },
			{ name: "description", content: m.site.description },
		],
		links: [{ rel: "canonical", href: "/" }],
	}),
	component: Home,
});

const perks = [
	{ icon: Truck, title: m.perks.delivery, detail: m.perks.deliveryDetail },
	{ icon: RotateCcw, title: m.perks.returns, detail: m.perks.returnsDetail },
	{
		icon: ShieldCheck,
		title: m.perks.warranty,
		detail: m.perks.warrantyDetail,
	},
	{ icon: CreditCard, title: m.perks.payment, detail: m.perks.paymentDetail },
];

function Home() {
	const results = useSuspenseQueries({ queries });
	const [big, second, third] = results
		.slice(0, featuredCategories.length)
		.map((r) => r.data.products[0]);
	const rows = results.slice(featuredCategories.length).map((r, i) => ({
		category: rowCategories[i],
		products: r.data.products,
	}));

	return (
		<>
			<Container className="pt-10 pb-8 lg:pt-16 lg:pb-10">
				<div className="flex flex-col gap-6 lg:flex-row lg:items-end lg:justify-between">
					<div>
						<h1 className="max-w-2xl text-balance font-semibold text-4xl tracking-[-0.035em] sm:text-5xl">
							{m.home.title}
						</h1>
						<p className="mt-4 max-w-lg text-lg text-muted-foreground">
							{m.home.lead}
						</p>
					</div>
					<div className="flex shrink-0 flex-wrap gap-3">
						<Button
							size="lg"
							render={<Link to="/c/$category" params={{ category: "gpu" }} />}
						>
							{m.home.browseComponents} <ArrowRight />
						</Button>
						<Button
							size="lg"
							variant="outline"
							className="bg-card"
							render={
								<Link
									to="/c/$category"
									params={{ category: "peripheral.monitor" }}
								/>
							}
						>
							{m.nav.peripherals}
						</Button>
					</div>
				</div>
			</Container>

			<Container>
				<div className="grid grid-cols-2 gap-3 lg:grid-cols-4 lg:grid-rows-[repeat(2,16rem)]">
					{big ? (
						<BentoTile
							product={big}
							size="lg"
							className="col-span-2 h-80 sm:h-96 lg:row-span-2 lg:h-auto"
						/>
					) : null}
					{second ? (
						<BentoTile product={second} className="h-64 lg:h-auto" />
					) : null}
					{third ? (
						<BentoTile product={third} className="h-64 lg:h-auto" />
					) : null}
					<BuilderTile />
				</div>
			</Container>

			<Container className="pt-3">
				<ul className="grid grid-cols-2 gap-px overflow-hidden rounded-xl border bg-border lg:grid-cols-4">
					{perks.map((p) => (
						<li key={p.title} className="flex gap-3 bg-card p-4 sm:p-5">
							<p.icon
								className="mt-0.5 size-5 shrink-0 text-brand-ink"
								strokeWidth={1.75}
							/>
							<div>
								<p className="font-medium text-sm">{p.title}</p>
								<p className="mt-0.5 text-meta text-muted-foreground">
									{p.detail}
								</p>
							</div>
						</li>
					))}
				</ul>
			</Container>

			{rows.map((r) =>
				r.products.length > 0 ? (
					<ProductRow
						key={r.category}
						category={r.category}
						products={r.products}
					/>
				) : null,
			)}
		</>
	);
}

function BentoTile({
	product: p,
	size = "md",
	className,
}: {
	product: ProductSummary;
	size?: "md" | "lg";
	className?: string;
}) {
	return (
		<Link
			to="/p/$slug"
			params={{ slug: p.slug }}
			className={cn(
				"group relative flex flex-col overflow-hidden rounded-xl border bg-card transition-colors hover:border-border-strong",
				className,
			)}
		>
			<ProductImage
				src={p.image}
				alt={p.name}
				loading="eager"
				fetchPriority={size === "lg" ? "high" : undefined}
				className={cn(
					"aspect-auto min-h-0 flex-1 transition-transform duration-500 ease-out group-hover:scale-[1.04]",
					size === "lg" ? "p-[8%]" : "p-[10%]",
				)}
			/>
			<div className="flex items-end justify-between gap-3 px-5 pb-4 sm:px-6 sm:pb-5">
				<div className="min-w-0">
					<p className="font-medium text-[11px] text-subtle-foreground uppercase tracking-wider">
						{m.categories[p.category]}
					</p>
					<p
						className={cn(
							"mt-1 font-semibold tracking-[-0.01em]",
							size === "lg"
								? "line-clamp-2 text-xl sm:text-2xl sm:tracking-[-0.02em]"
								: "truncate text-base",
						)}
					>
						{p.name}
					</p>
					<Price
						cents={p.price.amount_cents}
						className={cn(
							"mt-0.5 block text-muted-foreground",
							size === "lg" ? "text-base" : "text-sm",
						)}
					/>
				</div>
				<span className="flex size-9 shrink-0 items-center justify-center rounded-full border bg-card text-muted-foreground transition-colors group-hover:border-foreground group-hover:bg-primary group-hover:text-primary-foreground">
					<ArrowUpRight className="size-4" />
				</span>
			</div>
		</Link>
	);
}

function BuilderTile() {
	const parts = [Cpu, Gpu, CircuitBoard, MemoryStick, PlugZap, Box, Fan];
	return (
		<Link
			to="/builder"
			className="group relative col-span-2 flex flex-col justify-between gap-6 overflow-hidden rounded-xl bg-primary p-6 text-primary-foreground sm:p-7"
		>
			<div className="flex gap-2">
				{parts.map((Icon, i) => (
					<span
						// biome-ignore lint/suspicious/noArrayIndexKey: static decorative list
						key={i}
						className="flex size-9 items-center justify-center rounded-lg border border-white/10 bg-white/5 text-white/60"
					>
						<Icon className="size-4.5" strokeWidth={1.5} />
					</span>
				))}
			</div>
			<div className="flex items-end justify-between gap-6">
				<div>
					<p className="font-semibold text-xl tracking-[-0.02em] sm:text-2xl">
						{m.home.builderTitle}
					</p>
					<p className="mt-1.5 max-w-md text-sm text-white/60">
						{m.home.builderDetail}
					</p>
				</div>
				<span className="inline-flex h-9 shrink-0 items-center gap-1.5 rounded-full bg-white px-4 font-medium text-primary text-sm transition-transform group-hover:translate-x-0.5">
					{m.home.builderStart} <ArrowRight className="size-4" />
				</span>
			</div>
		</Link>
	);
}

function ProductRow({
	category,
	products,
}: {
	category: Category;
	products: ProductSummary[];
}) {
	return (
		<Container className="pt-16 lg:pt-20">
			<div className="mb-5 flex items-end justify-between gap-4">
				<h2 className="font-semibold text-2xl tracking-[-0.02em]">
					{m.categories[category]}
				</h2>
				<Link
					to="/c/$category"
					params={{ category }}
					className="inline-flex items-center gap-1 font-medium text-muted-foreground text-sm transition-colors hover:text-foreground"
				>
					{m.home.viewAll} <ArrowRight className="size-4" />
				</Link>
			</div>
			<div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
				{products.map((p) => (
					<CartProductCard key={p.slug} product={p} />
				))}
			</div>
		</Container>
	);
}
