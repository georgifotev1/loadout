import { createFileRoute, Link } from "@tanstack/react-router";
import { RotateCcw, ShieldCheck, ShoppingCart, Truck } from "lucide-react";
import { useState } from "react";
import { Quantity } from "#/components/cart/quantity";
import { Price } from "#/components/product/price";
import { ProductImage } from "#/components/product/product-image";
import { Stock } from "#/components/product/stock";
import { Container } from "#/components/site/container";
import {
	Breadcrumb,
	BreadcrumbItem,
	BreadcrumbLink,
	BreadcrumbList,
	BreadcrumbPage,
	BreadcrumbSeparator,
} from "#/components/ui/breadcrumb";
import { Button } from "#/components/ui/button";
import { formatShortDate, m } from "#/i18n";
import { cn } from "#/lib/utils";
import { PreviewProductCard, usePreviewCart } from "./-cart";
import { bySlug, deliveryDate, inCategory, keySpecs, products } from "./-data";

export const Route = createFileRoute("/preview/product")({
	validateSearch: (s: Record<string, unknown>): { slug?: string } => ({
		slug: typeof s.slug === "string" ? s.slug : undefined,
	}),
	component: ProductPage,
});

function ProductPage() {
	const { slug } = Route.useSearch();
	const p = bySlug(slug ?? "asr-vc-b580-cl-12g") ?? products[0];
	const [image, setImage] = useState(0);
	const [qty, setQty] = useState(1);
	const cart = usePreviewCart();
	const addToCart = () => cart.add(p.slug, qty);
	const specs = Object.entries(p.attributes).filter(([, v]) => v && v !== "-");
	const related = inCategory(p.category)
		.filter((r) => r.slug !== p.slug)
		.slice(0, 4);

	return (
		<Container className="pt-6 pb-20 lg:pb-0">
			<Breadcrumb>
				<BreadcrumbList>
					<BreadcrumbItem>
						<BreadcrumbLink render={<Link to="/preview/home" />}>
							{m.product.home}
						</BreadcrumbLink>
					</BreadcrumbItem>
					<BreadcrumbSeparator />
					<BreadcrumbItem>
						<BreadcrumbLink
							render={
								<Link
									to="/preview/category"
									search={{ category: p.category }}
								/>
							}
						>
							{m.categories[p.category]}
						</BreadcrumbLink>
					</BreadcrumbItem>
					<BreadcrumbSeparator />
					<BreadcrumbItem className="min-w-0">
						<BreadcrumbPage className="truncate">{p.brand}</BreadcrumbPage>
					</BreadcrumbItem>
				</BreadcrumbList>
			</Breadcrumb>

			<div className="mt-6 grid gap-8 lg:grid-cols-[1.15fr_1fr] lg:gap-12">
				<div className="flex flex-col gap-3">
					<div className="overflow-hidden rounded-xl border bg-card">
						<ProductImage
							src={p.images[image]}
							alt={p.name}
							loading="eager"
							className="p-[10%]"
						/>
					</div>
					{p.images.length > 1 ? (
						<div className="flex gap-2">
							{p.images.map((src, i) => (
								<button
									key={src}
									type="button"
									onClick={() => setImage(i)}
									className={cn(
										"size-20 overflow-hidden rounded-lg border bg-card",
										i === image && "border-foreground",
									)}
								>
									<ProductImage src={src} alt="" />
								</button>
							))}
						</div>
					) : null}
				</div>

				<div className="lg:sticky lg:top-[calc(var(--spacing-header)+2.5rem+1.5rem)] lg:self-start">
					<p className="font-medium text-subtle-foreground text-xs uppercase tracking-wider">
						{p.brand}
					</p>
					<h1 className="mt-2 text-balance font-semibold text-2xl tracking-[-0.02em] sm:text-3xl sm:tracking-[-0.025em]">
						{p.name}
					</h1>
					<div className="mt-3 flex flex-wrap items-center gap-x-4 gap-y-1 text-meta">
						<Stock inStock={p.in_stock} className="text-meta" />
						<span className="text-subtle-foreground">
							{m.product.sku}{" "}
							<span className="font-mono text-muted-foreground">{p.sku}</span>
						</span>
					</div>

					<div className="mt-8 flex items-baseline gap-3">
						<Price
							cents={p.price.amount_cents}
							className="text-4xl tracking-[-0.03em]"
						/>
						<span className="text-subtle-foreground text-xs">
							{m.product.vatIncluded}
						</span>
					</div>

					<div className="mt-6 flex gap-3">
						<Quantity value={qty} onChange={setQty} size="lg" />
						<Button
							size="lg"
							className="h-11 flex-1"
							disabled={!p.in_stock}
							onClick={addToCart}
						>
							<ShoppingCart /> {m.product.addToCart}
						</Button>
					</div>

					<ul className="mt-6 divide-y rounded-lg border bg-card text-sm">
						{[
							{
								icon: Truck,
								text: m.product.deliveryBy(formatShortDate(deliveryDate())),
							},
							{ icon: RotateCcw, text: m.product.freeReturns },
							{ icon: ShieldCheck, text: m.perks.warranty },
						].map((r) => (
							<li key={r.text} className="flex items-center gap-3 px-4 py-3">
								<r.icon
									className="size-4 text-subtle-foreground"
									strokeWidth={1.75}
								/>
								{r.text}
							</li>
						))}
					</ul>

					{keySpecs(p).length > 0 ? (
						<dl
							className={cn(
								"mt-6 grid gap-px overflow-hidden rounded-lg border bg-border",
								keySpecs(p).length === 3 ? "grid-cols-3" : "grid-cols-2",
							)}
						>
							{keySpecs(p).map(([k, v]) => (
								<div key={k} className="bg-card px-4 py-3">
									<dt className="text-subtle-foreground text-xs">{k}</dt>
									<dd className="mt-0.5 truncate font-medium text-sm">{v}</dd>
								</div>
							))}
						</dl>
					) : null}
				</div>
			</div>

			{specs.length > 0 ? (
				<section className="mt-16 lg:mt-24">
					<h2 className="font-semibold text-2xl tracking-[-0.02em]">
						{m.product.specs}
					</h2>
					<dl className="mt-5 grid overflow-hidden rounded-xl border bg-card md:grid-cols-2">
						{specs.map(([k, v], i) => (
							<div
								key={k}
								className={cn(
									"grid grid-cols-[minmax(0,2fr)_minmax(0,3fr)] gap-4 border-t px-5 py-3 text-sm",
									i === 0 && "border-t-0",
									i === 1 && "md:border-t-0",
									i % 2 === 1 && "md:border-l",
								)}
							>
								<dt className="text-muted-foreground">{k}</dt>
								<dd className="font-medium">{v}</dd>
							</div>
						))}
					</dl>
				</section>
			) : null}

			{related.length > 0 ? (
				<section className="mt-16 lg:mt-24">
					<h2 className="mb-5 font-semibold text-2xl tracking-[-0.02em]">
						{m.product.related}
					</h2>
					<div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
						{related.map((r) => (
							<PreviewProductCard key={r.slug} product={r} />
						))}
					</div>
				</section>
			) : null}

			{/* Phones: the price and add to cart stay in reach while reading specs. */}
			<div className="fixed inset-x-0 bottom-0 z-30 flex items-center gap-4 border-t bg-card/95 px-4 pt-3 pb-[calc(0.75rem+env(safe-area-inset-bottom,0px))] backdrop-blur-md lg:hidden">
				<div className="min-w-0 flex-1">
					<Price cents={p.price.amount_cents} className="block text-lg" />
					<Stock inStock={p.in_stock} />
				</div>
				<Button
					size="lg"
					className="h-11"
					disabled={!p.in_stock}
					onClick={addToCart}
				>
					<ShoppingCart /> {m.product.addToCart}
				</Button>
			</div>
		</Container>
	);
}
