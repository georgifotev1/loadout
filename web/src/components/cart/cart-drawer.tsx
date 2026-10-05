import { Link } from "@tanstack/react-router";
import { ArrowRight, Check, ShoppingCart, X } from "lucide-react";
import { Price } from "#/components/product/price";
import { ProductImage } from "#/components/product/product-image";
import { Button } from "#/components/ui/button";
import {
	Drawer,
	DrawerClose,
	DrawerContent,
	DrawerPopup,
	DrawerTitle,
} from "#/components/ui/drawer";
import { useMediaQuery } from "#/hooks/use-media-query";
import { m } from "#/i18n";
import type { components } from "#/lib/api/schema.gen";
import { cn } from "#/lib/utils";
import { Quantity } from "./quantity";

type ProductSummary = components["schemas"]["ProductSummary"];

export type CartLine = { product: ProductSummary; qty: number };

// CartDrawer is the mini cart: a sidebar on desktop and a swipeable bottom sheet on phones.
// It opens from the header and after "add to cart", so shoppers never leave the page they are on.
// TODO(phase-4): point the links at the real cart and checkout routes.
export function CartDrawer({
	open,
	onOpenChange,
	lines,
	justAdded,
	onQtyChange,
	onRemove,
}: {
	open: boolean;
	onOpenChange: (open: boolean) => void;
	lines: CartLine[];
	justAdded?: string;
	onQtyChange: (slug: string, qty: number) => void;
	onRemove: (slug: string) => void;
}) {
	const desktop = useMediaQuery("(min-width: 768px)");
	const side = desktop ? "right" : "bottom";
	const count = lines.reduce((s, l) => s + l.qty, 0);
	const subtotal = lines.reduce(
		(s, l) => s + l.product.price.amount_cents * l.qty,
		0,
	);

	return (
		<Drawer side={side} open={open} onOpenChange={onOpenChange}>
			<DrawerPopup side={side}>
				<DrawerContent className="flex min-h-0 flex-1 flex-col">
					<div className="flex items-center justify-between gap-4 border-b px-5 py-4">
						<DrawerTitle className="font-medium">
							{m.cart.title}{" "}
							<span className="text-subtle-foreground tabular-nums">
								({count})
							</span>
						</DrawerTitle>
						<DrawerClose
							render={
								<Button
									variant="ghost"
									size="icon-sm"
									aria-label={m.nav.close}
									className="-mr-1.5 text-muted-foreground"
								/>
							}
						>
							<X />
						</DrawerClose>
					</div>

					{justAdded ? (
						<p className="flex items-center gap-2 border-b bg-success/8 px-5 py-2.5 text-meta text-success">
							<Check className="size-4" /> {m.product.added}
						</p>
					) : null}

					{lines.length === 0 ? (
						<div className="flex flex-1 flex-col items-center justify-center gap-3 p-10 text-center">
							<ShoppingCart
								className="size-8 text-subtle-foreground"
								strokeWidth={1.5}
							/>
							<p className="text-muted-foreground text-sm">{m.cart.empty}</p>
						</div>
					) : (
						<ul className="min-h-0 flex-1 divide-y overflow-y-auto overscroll-contain">
							{lines.map(({ product: p, qty }) => (
								<li
									key={p.slug}
									className={cn(
										"flex gap-4 px-5 py-4 transition-colors",
										p.slug === justAdded && "bg-muted/60",
									)}
								>
									<ProductImage
										src={p.image}
										alt={p.name}
										className="size-18 shrink-0 rounded-lg border p-1.5"
									/>
									<div className="flex min-w-0 flex-1 flex-col">
										<div className="flex gap-2">
											<Link
												to="/p/$slug"
												params={{ slug: p.slug }}
												onClick={() => onOpenChange(false)}
												className="line-clamp-2 flex-1 text-meta hover:underline"
											>
												{p.name}
											</Link>
											<button
												type="button"
												aria-label={m.cart.remove}
												onClick={() => onRemove(p.slug)}
												className="-mt-0.5 flex size-6 shrink-0 items-center justify-center rounded text-subtle-foreground hover:bg-muted hover:text-foreground"
											>
												<X className="size-3.5" />
											</button>
										</div>
										<div className="mt-auto flex items-end justify-between pt-2">
											<Quantity
												value={qty}
												onChange={(q) => onQtyChange(p.slug, q)}
											/>
											<Price
												cents={p.price.amount_cents * qty}
												className="text-sm"
											/>
										</div>
									</div>
								</li>
							))}
						</ul>
					)}

					{lines.length > 0 ? (
						<div className="border-t bg-card px-5 pt-4 pb-5">
							<div className="flex items-baseline justify-between">
								<span className="text-muted-foreground text-sm">
									{m.cart.subtotal}
								</span>
								<Price cents={subtotal} className="text-lg" />
							</div>
							<p className="mt-0.5 text-subtle-foreground text-xs">
								{m.cart.vatIncluded} · {m.cart.delivery}:{" "}
								{m.cart.deliveryAtCheckout.toLowerCase()}
							</p>
							<Button
								size="lg"
								className="mt-4 h-11 w-full"
								render={
									<Link
										to="/preview/checkout"
										onClick={() => onOpenChange(false)}
									/>
								}
							>
								{m.cart.checkout} <ArrowRight />
							</Button>
							<Button
								variant="ghost"
								className="mt-1 w-full text-muted-foreground"
								render={
									<Link
										to="/preview/cart"
										onClick={() => onOpenChange(false)}
									/>
								}
							>
								{m.cart.viewCart}
							</Button>
						</div>
					) : null}
				</DrawerContent>
			</DrawerPopup>
		</Drawer>
	);
}
