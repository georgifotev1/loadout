import { Link } from "@tanstack/react-router";
import { ShoppingCart } from "lucide-react";
import { Button } from "#/components/ui/button";
import { m } from "#/i18n";
import type { components } from "#/lib/api/schema.gen";
import { cn } from "#/lib/utils";
import { Price } from "./price";
import { ProductImage } from "./product-image";
import { Stock } from "./stock";

type ProductSummary = components["schemas"]["ProductSummary"];

export function ProductCard({
	product,
	highlights,
	onAddToCart,
	className,
}: {
	product: ProductSummary;
	// Key specs shown under the name, e.g. ["12 GB", "GDDR6"].
	// TODO(phase-6): from the canonical specs, once products have them.
	highlights?: string[];
	// Without it there's no add to cart button.
	onAddToCart?: () => void;
	className?: string;
}) {
	return (
		<article
			className={cn(
				"group relative flex flex-col overflow-hidden rounded-lg border bg-card transition-colors hover:border-border-strong",
				className,
			)}
		>
			<ProductImage
				src={product.image}
				alt={product.name}
				className="transition-transform duration-300 ease-out group-hover:scale-[1.03]"
			/>
			<div className="flex flex-1 flex-col gap-1 px-4 pt-3 pb-4">
				<p className="font-medium text-[11px] text-subtle-foreground uppercase tracking-wider">
					{product.brand}
				</p>
				<h3 className="line-clamp-2 min-h-10 font-medium text-sm leading-5">
					<Link
						to="/p/$slug"
						params={{ slug: product.slug }}
						className="outline-none after:absolute after:inset-0 focus-visible:underline"
					>
						{product.name}
					</Link>
				</h3>
				{highlights?.length ? (
					<p className="truncate text-subtle-foreground text-xs">
						{highlights.join(" · ")}
					</p>
				) : null}
				<div className="mt-auto flex items-end justify-between gap-2 pt-3">
					<div className="flex flex-col gap-1">
						<Price cents={product.price.amount_cents} className="text-base" />
						<Stock inStock={product.in_stock} />
					</div>
					{onAddToCart ? (
						<Button
							variant="outline"
							size="icon-sm"
							className="relative z-10"
							disabled={!product.in_stock}
							onClick={onAddToCart}
							aria-label={m.product.addToCart}
						>
							<ShoppingCart />
						</Button>
					) : null}
				</div>
			</div>
		</article>
	);
}
