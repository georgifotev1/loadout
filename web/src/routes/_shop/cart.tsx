import { useQueryClient } from "@tanstack/react-query";
import { createFileRoute, Link } from "@tanstack/react-router";
import { ArrowRight, ShoppingCart, TriangleAlert, X } from "lucide-react";
import { useCart } from "#/components/cart/cart-provider";
import { LineNote, LineTitle } from "#/components/cart/line";
import { Quantity } from "#/components/cart/quantity";
import { Price } from "#/components/product/price";
import { ProductImage } from "#/components/product/product-image";
import { Container } from "#/components/site/container";
import { PageSkeleton } from "#/components/site/page-skeletons";
import { StatusPage } from "#/components/site/status-page";
import { Button } from "#/components/ui/button";
import { m } from "#/i18n";
import { cartQuery, maxQuantity } from "#/lib/api/cart";
import { cn } from "#/lib/utils";

// /cart: the cart in full. Its data loads in the browser (see cartQuery), so the server renders
// the skeleton.
export const Route = createFileRoute("/_shop/cart")({
	head: () => ({
		meta: [
			{ title: m.site.title(m.cart.title) },
			{ name: "robots", content: "noindex" },
		],
	}),
	component: CartPage,
});

function CartPage() {
	const { cart, loadFailed, setQuantity, remove, busy, error } = useCart();
	const queryClient = useQueryClient();

	if (loadFailed && !cart) {
		return (
			<StatusPage title={m.error.title} detail={m.error.detail}>
				<Button
					size="lg"
					onClick={() =>
						queryClient.refetchQueries({ queryKey: cartQuery.queryKey })
					}
				>
					{m.error.retry}
				</Button>
			</StatusPage>
		);
	}
	if (!cart) return <PageSkeleton />;

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
			{error ? (
				<p
					role="alert"
					className="mt-4 flex items-center gap-2 rounded-xl border border-destructive/40 bg-destructive/3 px-4 py-3 text-destructive text-meta"
				>
					<TriangleAlert className="size-4 shrink-0" /> {error}
				</p>
			) : null}

			<div className="mt-8 grid items-start gap-8 lg:grid-cols-[1fr_22rem]">
				<ul className="divide-y rounded-xl border bg-card">
					{cart.lines.map((line) => (
						<li
							key={line.product_id}
							aria-busy={busy(line.product_id)}
							className="flex gap-4 p-4 sm:gap-5 sm:p-5"
						>
							<ProductImage
								src={line.product?.image}
								alt={line.name}
								className={cn(
									"size-20 shrink-0 rounded-lg border p-2 sm:size-24",
									!line.product && "opacity-50",
								)}
							/>
							<div className="flex min-w-0 flex-1 flex-col">
								<div className="flex gap-4">
									<div className="min-w-0 flex-1">
										{line.product ? (
											<p className="font-medium text-[11px] text-subtle-foreground uppercase tracking-wider">
												{line.product.brand}
											</p>
										) : null}
										<LineTitle
											line={line}
											className="mt-0.5 line-clamp-2 font-medium text-sm"
										/>
										<LineNote line={line} className="mt-1" />
									</div>
									<Button
										variant="ghost"
										size="icon-sm"
										aria-label={m.cart.remove}
										disabled={busy(line.product_id)}
										onClick={() => remove(line.product_id)}
										className="-mt-1 -mr-1 text-subtle-foreground"
									>
										<X />
									</Button>
								</div>
								{line.product && line.line_total ? (
									<div className="mt-auto flex items-end justify-between gap-4 pt-3">
										<Quantity
											value={line.quantity}
											max={maxQuantity}
											disabled={busy(line.product_id)}
											onChange={(q) => setQuantity(line.product_id, q)}
										/>
										<Price
											cents={line.line_total.amount_cents}
											className="text-base"
										/>
									</div>
								) : null}
							</div>
						</li>
					))}
				</ul>

				<aside className="rounded-xl border bg-card p-6 lg:sticky lg:top-[calc(var(--spacing-header)+2.5rem+1.5rem)]">
					<h2 className="font-medium">{m.cart.summary}</h2>
					<dl className="mt-5 flex flex-col gap-2.5 text-sm">
						<div className="flex justify-between">
							<dt className="text-muted-foreground">{m.cart.subtotal}</dt>
							<dd>
								<Price
									cents={cart.subtotal.amount_cents}
									className="text-2xl tracking-[-0.02em]"
								/>
							</dd>
						</div>
						<div className="flex justify-between">
							<dt className="text-muted-foreground">{m.cart.delivery}</dt>
							<dd className="text-muted-foreground">
								{m.cart.deliveryAtCheckout}
							</dd>
						</div>
						<p className="text-right text-subtle-foreground text-xs">
							{m.cart.vatIncluded}
						</p>
					</dl>
					<Button
						size="lg"
						className="mt-6 h-11 w-full"
						render={<Link to="/checkout" />}
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
