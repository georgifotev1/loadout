import { Link } from "@tanstack/react-router";
import {
	ArrowRight,
	Check,
	ShoppingCart,
	TriangleAlert,
	X,
} from "lucide-react";
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
import { maxQuantity } from "#/lib/api/cart";
import { cn } from "#/lib/utils";
import { useCart } from "./cart-provider";
import { LineNote, LineTitle } from "./line";
import { Quantity } from "./quantity";

// What the drawer says on top: the slugs just added, or why adding failed.
export type CartNotice = { added: string[] } | { error: string };

// CartDrawer is the mini cart: a sidebar on desktop and a swipeable bottom sheet on phones.
// It opens from the header and after "add to cart", so shoppers never leave the page they are on.
export function CartDrawer({
	open,
	onOpenChange,
	notice,
}: {
	open: boolean;
	onOpenChange: (open: boolean) => void;
	notice?: CartNotice;
}) {
	const desktop = useMediaQuery("(min-width: 768px)");
	const side = desktop ? "right" : "bottom";
	const { cart, setQuantity, remove, busy, error } = useCart();
	const lines = cart?.lines ?? [];
	const added = notice && "added" in notice ? notice.added : [];
	const close = () => onOpenChange(false);

	return (
		<Drawer side={side} open={open} onOpenChange={onOpenChange}>
			<DrawerPopup side={side}>
				<DrawerContent className="flex min-h-0 flex-1 flex-col">
					<div className="flex items-center justify-between gap-4 border-b px-5 py-4">
						<DrawerTitle className="font-medium">
							{m.cart.title}{" "}
							<span className="text-subtle-foreground tabular-nums">
								({cart?.count ?? 0})
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

					{notice && "error" in notice ? (
						<Banner error>{notice.error}</Banner>
					) : added.length ? (
						<Banner>{m.product.added}</Banner>
					) : null}
					{error ? <Banner error>{error}</Banner> : null}

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
							{lines.map((line) => (
								<li
									key={line.product_id}
									aria-busy={busy(line.product_id)}
									className={cn(
										"flex gap-4 px-5 py-4 transition-colors",
										added.includes(line.slug) && "bg-muted/60",
									)}
								>
									<ProductImage
										src={line.product?.image}
										alt={line.name}
										className={cn(
											"size-18 shrink-0 rounded-lg border p-1.5",
											!line.product && "opacity-50",
										)}
									/>
									<div className="flex min-w-0 flex-1 flex-col">
										<div className="flex gap-2">
											<LineTitle
												line={line}
												onClick={close}
												className="line-clamp-2 flex-1 text-meta"
											/>
											<button
												type="button"
												aria-label={m.cart.remove}
												disabled={busy(line.product_id)}
												onClick={() => remove(line.product_id)}
												className="-mt-0.5 flex size-6 shrink-0 items-center justify-center rounded text-subtle-foreground hover:bg-muted hover:text-foreground"
											>
												<X className="size-3.5" />
											</button>
										</div>
										<LineNote line={line} className="mt-1" />
										{line.product && line.line_total ? (
											<div className="mt-auto flex items-end justify-between pt-2">
												<Quantity
													value={line.quantity}
													max={maxQuantity}
													disabled={busy(line.product_id)}
													onChange={(q) => setQuantity(line.product_id, q)}
												/>
												<Price
													cents={line.line_total.amount_cents}
													className="text-sm"
												/>
											</div>
										) : null}
									</div>
								</li>
							))}
						</ul>
					)}

					{cart && lines.length > 0 ? (
						<div className="border-t bg-card px-5 pt-4 pb-5">
							<div className="flex items-baseline justify-between">
								<span className="text-muted-foreground text-sm">
									{m.cart.subtotal}
								</span>
								<Price cents={cart.subtotal.amount_cents} className="text-lg" />
							</div>
							<p className="mt-0.5 text-subtle-foreground text-xs">
								{m.cart.vatIncluded} · {m.cart.delivery}:{" "}
								{m.cart.deliveryAtCheckout.toLowerCase()}
							</p>
							<Button
								size="lg"
								className="mt-4 h-11 w-full"
								render={<Link to="/cart" onClick={close} />}
							>
								{m.cart.viewCart} <ArrowRight />
							</Button>
						</div>
					) : null}
				</DrawerContent>
			</DrawerPopup>
		</Drawer>
	);
}

function Banner({
	error,
	children,
}: {
	error?: boolean;
	children: React.ReactNode;
}) {
	return (
		<p
			role={error ? "alert" : "status"}
			className={cn(
				"flex items-center gap-2 border-b px-5 py-2.5 text-meta",
				error
					? "bg-destructive/[0.04] text-destructive"
					: "bg-success/8 text-success",
			)}
		>
			{error ? (
				<TriangleAlert className="size-4 shrink-0" />
			) : (
				<Check className="size-4 shrink-0" />
			)}
			{children}
		</p>
	);
}
