import { Link } from "@tanstack/react-router";
import { m } from "#/i18n";
import type { CartLine } from "#/lib/api/cart";
import { cn } from "#/lib/utils";

// LineTitle is a cart line's product name: a link while the product is for sale.
export function LineTitle({
	line,
	onClick,
	className,
}: {
	line: CartLine;
	onClick?: () => void;
	className?: string;
}) {
	if (!line.product) {
		return (
			<span className={cn("text-muted-foreground", className)}>
				{line.name}
			</span>
		);
	}
	return (
		<Link
			to="/p/$slug"
			params={{ slug: line.slug }}
			onClick={onClick}
			className={cn("hover:underline", className)}
		>
			{line.name}
		</Link>
	);
}

// LineNote says why a line can't be ordered: the product is gone or out of stock.
export function LineNote({
	line,
	className,
}: {
	line: CartLine;
	className?: string;
}) {
	const note = !line.product
		? m.cart.unavailable
		: !line.product.in_stock
			? m.cart.outOfStock
			: null;
	if (!note) return null;
	return <p className={cn("text-destructive text-xs", className)}>{note}</p>;
}
