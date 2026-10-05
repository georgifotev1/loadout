import { SiteFooter } from "./footer";
import { SiteHeader } from "./header";
import { ShopSearch } from "./shop-search";

// ShopLayout is the storefront chrome around every shop page and the not-found page.
// TODO(phase-7): pass the cart count and drawer to SiteHeader once there's a cart.
export function ShopLayout({ children }: { children: React.ReactNode }) {
	return (
		<div id="top" className="flex min-h-dvh flex-col">
			<SiteHeader search={<ShopSearch />} />
			<main className="flex-1">{children}</main>
			<SiteFooter />
		</div>
	);
}
