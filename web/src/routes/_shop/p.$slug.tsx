import { useQuery, useSuspenseQuery } from "@tanstack/react-query";
import { createFileRoute, notFound } from "@tanstack/react-router";
import { RotateCcw, ShieldCheck, ShoppingCart, Truck } from "lucide-react";
import { useState } from "react";
import { CartProductCard } from "#/components/cart/cart-product-card";
import { useCart } from "#/components/cart/cart-provider";
import { Quantity } from "#/components/cart/quantity";
import { Price } from "#/components/product/price";
import { ProductImage } from "#/components/product/product-image";
import { Stock } from "#/components/product/stock";
import { Container } from "#/components/site/container";
import { PageBreadcrumb } from "#/components/site/page-breadcrumb";
import { ProductSkeleton } from "#/components/site/page-skeletons";
import { Button } from "#/components/ui/button";
import { formatPrice, m } from "#/i18n";
import { maxQuantity } from "#/lib/api/cart";
import { ApiError, type Schemas } from "#/lib/api/client";
import { productQuery, searchQuery } from "#/lib/api/products";
import { cn } from "#/lib/utils";

type ProductDetail = Schemas["ProductDetail"];

const relatedQuery = (p: ProductDetail) =>
	searchQuery({ category: p.category, page_size: 5 });

export const Route = createFileRoute("/_shop/p/$slug")({
	loader: async ({ context, params }) => {
		const product = await context.queryClient
			.query(productQuery(params.slug))
			.catch((err) => {
				if (err instanceof ApiError && err.status === 404) throw notFound();
				throw err;
			});
		// Related products are a nice-to-have: the page renders without them.
		await context.queryClient.query(relatedQuery(product)).catch(() => {});
		return product;
	},
	head: ({ loaderData: p }) =>
		p
			? {
					meta: [
						{ title: m.site.title(p.name) },
						{
							name: "description",
							content: m.product.metaDescription(
								p.name,
								formatPrice(p.price.amount_cents),
								p.in_stock,
							),
						},
						{ property: "og:type", content: "product" },
						{ property: "og:title", content: p.name },
						...(p.images[0]
							? [{ property: "og:image", content: p.images[0] }]
							: []),
						{ "script:ld+json": productJsonLd(p) },
					],
					links: [{ rel: "canonical", href: `/p/${p.slug}` }],
				}
			: {},
	// The HTML has no per-shopper data (the cart loads in the browser), so shared caches (the
	// CDN) may keep it for a minute and serve it stale while refetching. Prices change on the
	// supplier sync, not by the second.
	headers: () => ({
		"Cache-Control": "public, max-age=60, stale-while-revalidate=300",
	}),
	pendingComponent: ProductSkeleton,
	component: ProductPage,
});

// productJsonLd is the schema.org Product search engines read for rich results: price,
// currency and availability.
function productJsonLd(p: ProductDetail) {
	return {
		"@context": "https://schema.org",
		"@type": "Product",
		name: p.name,
		sku: p.sku,
		brand: { "@type": "Brand", name: p.brand },
		image: p.images,
		offers: {
			"@type": "Offer",
			// schema.org wants a decimal string; the API's cents are exact, so this is too.
			price: (p.price.amount_cents / 100).toFixed(2),
			priceCurrency: p.price.currency,
			availability: p.in_stock
				? "https://schema.org/InStock"
				: "https://schema.org/OutOfStock",
		},
	};
}

function ProductPage() {
	const { slug } = Route.useParams();
	const { data: p } = useSuspenseQuery(productQuery(slug));
	const [image, setImage] = useState(0);
	const [qty, setQty] = useState(1);
	const cart = useCart();
	const specs = Object.entries(p.attributes).filter(([, v]) => v && v !== "-");

	return (
		<Container className="pt-6">
			<PageBreadcrumb category={p.category} current={p.brand} />

			<div className="mt-6 grid gap-8 lg:grid-cols-[1.15fr_1fr] lg:gap-12">
				<div className="flex flex-col gap-3">
					<div className="overflow-hidden rounded-xl border bg-card">
						<ProductImage
							src={p.images[image]}
							alt={p.name}
							loading="eager"
							fetchPriority="high"
							className="p-[10%]"
						/>
					</div>
					{p.images.length > 1 ? (
						<div className="flex gap-2 overflow-x-auto">
							{p.images.map((src, i) => (
								<button
									key={src}
									type="button"
									onClick={() => setImage(i)}
									aria-label={`${p.name} ${i + 1}`}
									aria-pressed={i === image}
									className={cn(
										"size-20 shrink-0 overflow-hidden rounded-lg border bg-card",
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
					<h1 className="mt-2 text-balance font-semibold text-2xl tracking-[-0.02em] sm:text-3xl sm:tracking-tight">
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
						<Quantity
							size="lg"
							value={qty}
							max={maxQuantity}
							disabled={!p.in_stock}
							onChange={setQty}
						/>
						<Button
							size="lg"
							className="h-11 flex-1 sm:flex-none"
							disabled={!p.in_stock || cart.adding}
							onClick={() => cart.add([{ slug: p.slug, quantity: qty }])}
						>
							<ShoppingCart />{" "}
							{cart.adding ? m.product.adding : m.product.addToCart}
						</Button>
					</div>

					<ul className="mt-6 divide-y rounded-lg border bg-card text-sm">
						{[
							{ icon: Truck, text: m.product.deliveryEstimate },
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

			<Related product={p} />
		</Container>
	);
}

function Related({ product }: { product: ProductDetail }) {
	// Not suspense: if they failed to load, the rest of the page still shows.
	const { data } = useQuery(relatedQuery(product));
	const related = (data?.products ?? [])
		.filter((r) => r.slug !== product.slug)
		.slice(0, 4);
	if (related.length === 0) return null;
	return (
		<section className="mt-16 lg:mt-24">
			<h2 className="mb-5 font-semibold text-2xl tracking-[-0.02em]">
				{m.product.related}
			</h2>
			<div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
				{related.map((r) => (
					<CartProductCard key={r.slug} product={r} />
				))}
			</div>
		</section>
	);
}
