import { ShoppingCart } from "lucide-react";
import { useCart } from "#/components/cart/cart-provider";
import { Button } from "#/components/ui/button";
import { m } from "#/i18n";
import { type BuildCheck, canOrder, cartLinesOf } from "#/lib/builder";

// AddBuildToCart puts every part of a build in the cart, once the API says it can be ordered,
// and asks for it to be assembled when the shopper chose that.
export function AddBuildToCart({
	check,
	assembly,
}: {
	check: BuildCheck;
	assembly: boolean;
}) {
	const cart = useCart();
	const orderable = canOrder(check);
	return (
		<>
			<Button
				size="lg"
				className="h-11 w-full"
				disabled={!orderable || cart.adding}
				onClick={() =>
					cart.add(cartLinesOf(check), {
						assembly: assembly && check.assembly.offered,
					})
				}
			>
				<ShoppingCart />{" "}
				{cart.adding ? m.product.adding : m.builder.summary.addToCart}
			</Button>
			{orderable ? null : (
				<p className="mt-2 text-center text-subtle-foreground text-xs">
					{check.assembly.offered && check.out_of_stock.length
						? m.builder.summary.replaceOutOfStock
						: m.builder.summary.fixFirst}
				</p>
			)}
		</>
	);
}
