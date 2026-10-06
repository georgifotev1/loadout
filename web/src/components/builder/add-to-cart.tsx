import { ShoppingCart } from "lucide-react";
import { useCart } from "#/components/cart/cart-provider";
import { Button } from "#/components/ui/button";
import { m } from "#/i18n";
import { type BuildCheck, cartLinesOf } from "#/lib/builder";

// AddBuildToCart puts every part of a build in the cart, once the build can be ordered.
export function AddBuildToCart({
	check,
	orderable,
}: {
	check: BuildCheck;
	orderable: boolean;
}) {
	const cart = useCart();
	return (
		<>
			<Button
				size="lg"
				className="h-11 w-full"
				disabled={!orderable || cart.adding}
				onClick={() => cart.add(cartLinesOf(check))}
			>
				<ShoppingCart />{" "}
				{cart.adding ? m.product.adding : m.builder.summary.addToCart}
			</Button>
			{orderable ? null : (
				<p className="mt-2 text-center text-subtle-foreground text-xs">
					{m.builder.summary.fixFirst}
				</p>
			)}
		</>
	);
}
