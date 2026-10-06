import { ProductCard } from "#/components/product/product-card";
import type { Schemas } from "#/lib/api/client";
import { useCart } from "./cart-provider";

// CartProductCard is the storefront's product card, whose button adds one to the cart.
export function CartProductCard({
	product,
}: {
	product: Schemas["ProductSummary"];
}) {
	const cart = useCart();
	return (
		<ProductCard
			product={product}
			onAddToCart={() => cart.add([{ slug: product.slug, quantity: 1 }])}
		/>
	);
}
