import { createFileRoute, Link } from "@tanstack/react-router";
import { ArrowRight, ShoppingCart, X } from "lucide-react";
import { Quantity } from "#/components/cart/quantity";
import { Price } from "#/components/product/price";
import { ProductImage } from "#/components/product/product-image";
import { Stock } from "#/components/product/stock";
import { Container } from "#/components/site/container";
import { Button } from "#/components/ui/button";
import { m } from "#/i18n";
import { usePreviewCart } from "./-cart";
import { Totals } from "./-order";

export const Route = createFileRoute("/preview/cart")({
	component: CartPage,
});

function CartPage() {
	const cart = usePreviewCart();

	if (cart.lines.length === 0) {
		return (
			<Container className="flex flex-col items-center pt-24 text-center">
				<ShoppingCart
					className="size-10 text-subtle-foreground"
					strokeWidth={1.5}
				/>
				<h1 className="mt-4 font-semibold text-2xl tracking-[-0.02em]">
					{m.cart.empty}
				</h1>
				<Button className="mt-6" render={<Link to="/" />}>
					{m.cart.continueShopping}
				</Button>
			</Container>
		);
	}

	return (
		<Container className="pt-10">
			<h1 className="font-semibold text-3xl tracking-[-0.03em]">
				{m.cart.title}
			</h1>
			<p className="mt-1 text-muted-foreground text-sm">
				{m.cart.items(cart.count)}
			</p>

			<div className="mt-8 grid items-start gap-8 lg:grid-cols-[1fr_22rem]">
				<ul className="divide-y rounded-xl border bg-card">
					{cart.lines.map(({ product: p, qty }) => (
						<li key={p.slug} className="flex gap-4 p-4 sm:gap-5 sm:p-5">
							<Link
								to="/p/$slug"
								params={{ slug: p.slug }}
								className="shrink-0"
							>
								<ProductImage
									src={p.image}
									alt={p.name}
									className="size-20 rounded-lg border p-2 sm:size-24"
								/>
							</Link>
							<div className="flex min-w-0 flex-1 flex-col">
								<div className="flex gap-4">
									<div className="min-w-0 flex-1">
										<p className="font-medium text-[11px] text-subtle-foreground uppercase tracking-wider">
											{p.brand}
										</p>
										<Link
											to="/p/$slug"
											params={{ slug: p.slug }}
											className="mt-0.5 line-clamp-2 font-medium text-sm hover:underline"
										>
											{p.name}
										</Link>
									</div>
									<Button
										variant="ghost"
										size="icon-sm"
										aria-label={m.cart.remove}
										onClick={() => cart.remove(p.slug)}
										className="-mt-1 -mr-1 text-subtle-foreground"
									>
										<X />
									</Button>
								</div>
								<div className="mt-auto flex items-end justify-between gap-4 pt-3">
									<div className="flex items-center gap-4">
										<Quantity
											value={qty}
											onChange={(q) => cart.setQty(p.slug, q)}
										/>
										<Stock
											inStock={p.in_stock}
											className="hidden sm:inline-flex"
										/>
									</div>
									<Price
										cents={p.price.amount_cents * qty}
										className="text-base"
									/>
								</div>
							</div>
						</li>
					))}
				</ul>

				<aside className="rounded-xl border bg-card p-6 lg:sticky lg:top-[calc(var(--spacing-header)+2.5rem+1.5rem)]">
					<h2 className="font-medium">{m.cart.summary}</h2>
					<Totals className="mt-5" />
					<Button
						size="lg"
						className="mt-6 h-11 w-full"
						render={<Link to="/preview/checkout" />}
					>
						{m.cart.checkout} <ArrowRight />
					</Button>
					<Button
						variant="link"
						className="mt-2 w-full text-muted-foreground"
						render={<Link to="/" />}
					>
						{m.cart.continueShopping}
					</Button>
				</aside>
			</div>
		</Container>
	);
}
