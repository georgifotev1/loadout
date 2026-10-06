import { CartProvider, useCart } from "#/components/cart/cart-provider";
import { SiteFooter } from "./footer";
import { SiteHeader } from "./header";
import { ShopSearch } from "./shop-search";

// ShopLayout is the storefront chrome around every shop page and the not-found page.
export function ShopLayout({ children }: { children: React.ReactNode }) {
	return (
		<CartProvider>
			<div id="top" className="flex min-h-dvh flex-col">
				<Header />
				<main className="flex-1">{children}</main>
				<SiteFooter />
			</div>
		</CartProvider>
	);
}

function Header() {
	const { cart, openDrawer } = useCart();
	return (
		<SiteHeader
			search={<ShopSearch />}
			cartCount={cart?.count ?? 0}
			onCartClick={openDrawer}
		/>
	);
}
