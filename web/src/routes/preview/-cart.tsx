import { createContext, useContext, useState } from "react";
import { ProductCard } from "#/components/product/product-card";
import type { Schemas } from "#/lib/api/client";
import {
	bySlug,
	cartLines,
	highlights,
	type PreviewProduct,
	summary,
} from "./-data";

// In-memory cart for the token specimen's product cards.

type CartLine = { product: Schemas["ProductSummary"]; qty: number };

type PreviewCart = {
	lines: CartLine[];
	count: number;
	add: (slug: string, qty?: number) => void;
};

const Ctx = createContext<PreviewCart | null>(null);

export function usePreviewCart() {
	const cart = useContext(Ctx);
	if (!cart) throw new Error("usePreviewCart outside PreviewCartProvider");
	return cart;
}

export function PreviewCartProvider({
	children,
}: {
	children: React.ReactNode;
}) {
	const [lines, setLines] = useState<CartLine[]>(() =>
		cartLines.map((l) => ({ product: summary(l.product), qty: l.qty })),
	);

	const cart: PreviewCart = {
		lines,
		count: lines.reduce((s, l) => s + l.qty, 0),
		add: (slug, qty = 1) => {
			const p = bySlug(slug);
			if (!p) return;
			setLines((cur) =>
				cur.some((l) => l.product.slug === slug)
					? cur.map((l) =>
							l.product.slug === slug ? { ...l, qty: l.qty + qty } : l,
						)
					: [{ product: summary(p), qty }, ...cur],
			);
		},
	};

	return <Ctx.Provider value={cart}>{children}</Ctx.Provider>;
}

// PreviewProductCard is ProductCard wired to the preview cart and sample highlights.
export function PreviewProductCard({ product }: { product: PreviewProduct }) {
	const cart = usePreviewCart();
	return (
		<ProductCard
			product={summary(product)}
			highlights={highlights(product)}
			onAddToCart={() => cart.add(product.slug)}
		/>
	);
}
