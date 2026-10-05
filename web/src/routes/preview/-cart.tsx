import { createContext, useContext, useState } from "react";
import { CartDrawer, type CartLine } from "#/components/cart/cart-drawer";
import { ProductCard } from "#/components/product/product-card";
import {
	bySlug,
	cartLines,
	highlights,
	type PreviewProduct,
	summary,
} from "./-data";

// In-memory cart for the previews, so add to cart, the drawer and the cart page agree.
// The real cart lives on the server (Phase 5).

type PreviewCart = {
	lines: CartLine[];
	count: number;
	add: (slug: string, qty?: number) => void;
	setQty: (slug: string, qty: number) => void;
	remove: (slug: string) => void;
	openDrawer: () => void;
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
	const [open, setOpen] = useState(false);
	const [justAdded, setJustAdded] = useState<string>();

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
			setJustAdded(slug);
			setOpen(true);
		},
		setQty: (slug, qty) =>
			setLines((cur) =>
				cur.map((l) => (l.product.slug === slug ? { ...l, qty } : l)),
			),
		remove: (slug) =>
			setLines((cur) => cur.filter((l) => l.product.slug !== slug)),
		openDrawer: () => {
			setJustAdded(undefined);
			setOpen(true);
		},
	};

	return (
		<Ctx.Provider value={cart}>
			{children}
			<CartDrawer
				open={open}
				onOpenChange={setOpen}
				lines={lines}
				justAdded={justAdded}
				onQtyChange={cart.setQty}
				onRemove={cart.remove}
			/>
		</Ctx.Provider>
	);
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
