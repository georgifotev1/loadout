import { Price } from "#/components/product/price";
import { ProductImage } from "#/components/product/product-image";
import { m } from "#/i18n";
import { cn } from "#/lib/utils";
import { usePreviewCart } from "./-cart";

export const deliveryCents = 599;

function useSubtotal() {
	const { lines } = usePreviewCart();
	return lines.reduce((s, l) => s + l.product.price.amount_cents * l.qty, 0);
}

export function Totals({
	delivery,
	className,
}: {
	delivery?: number;
	className?: string;
}) {
	const subtotal = useSubtotal();
	return (
		<dl className={cn("flex flex-col gap-2.5 text-sm", className)}>
			<div className="flex justify-between">
				<dt className="text-muted-foreground">{m.cart.subtotal}</dt>
				<dd>
					<Price cents={subtotal} className="font-medium" />
				</dd>
			</div>
			<div className="flex justify-between">
				<dt className="text-muted-foreground">{m.cart.delivery}</dt>
				<dd className="text-muted-foreground">
					{delivery === undefined ? (
						m.cart.deliveryAtCheckout
					) : (
						<Price cents={delivery} className="font-medium text-foreground" />
					)}
				</dd>
			</div>
			<div className="mt-2 flex items-baseline justify-between border-t pt-4">
				<dt className="font-medium">{m.cart.total}</dt>
				<dd>
					<Price
						cents={subtotal + (delivery ?? 0)}
						className="text-2xl tracking-[-0.02em]"
					/>
				</dd>
			</div>
			<p className="text-right text-subtle-foreground text-xs">
				{m.cart.vatIncluded}
			</p>
		</dl>
	);
}

// CompactLines lists the cart lines in the checkout and confirmation summaries.
export function CompactLines() {
	const { lines } = usePreviewCart();
	return (
		<ul className="flex flex-col gap-4">
			{lines.map(({ product: p, qty }) => (
				<li key={p.slug} className="flex items-center gap-3">
					<div className="relative shrink-0">
						<ProductImage
							src={p.image}
							alt={p.name}
							className="size-14 rounded-md border p-1.5"
						/>
						<span className="-top-1.5 -right-1.5 absolute flex size-5 items-center justify-center rounded-full bg-primary font-medium text-[11px] text-primary-foreground tabular-nums">
							{qty}
						</span>
					</div>
					<p className="line-clamp-2 flex-1 text-meta">{p.name}</p>
					<Price
						cents={p.price.amount_cents * qty}
						className="font-medium text-sm"
					/>
				</li>
			))}
		</ul>
	);
}
