import { createFileRoute, Outlet, useLocation } from "@tanstack/react-router";
import { CheckoutHeader } from "#/components/site/checkout-header";
import { SiteFooter } from "#/components/site/footer";
import { SiteHeader } from "#/components/site/header";
import { ShopSearch } from "#/components/site/shop-search";
import { PreviewCartProvider, usePreviewCart } from "./-cart";
import { PreviewToolbar } from "./-toolbar";

// /preview: design previews on sample data for the pages that don't exist yet (checkout,
// confirmation) and the design tokens. Not linked from anywhere.
// TODO(phase-7): delete once the real cart and checkout exist.
export const Route = createFileRoute("/preview")({
	head: () => ({ meta: [{ name: "robots", content: "noindex" }] }),
	component: PreviewLayout,
});

function PreviewLayout() {
	return (
		<PreviewCartProvider>
			<Shell />
		</PreviewCartProvider>
	);
}

function Shell() {
	const cart = usePreviewCart();
	const checkout = useLocation({
		select: (l) => l.pathname.startsWith("/preview/checkout"),
	});

	return (
		<div id="top" className="flex min-h-dvh flex-col">
			{checkout ? (
				<CheckoutHeader />
			) : (
				<SiteHeader
					search={<ShopSearch />}
					cartCount={cart.count}
					onCartClick={() => {}}
				/>
			)}
			<main className="flex-1">
				<Outlet />
			</main>
			{checkout ? null : <SiteFooter />}
			<PreviewToolbar />
		</div>
	);
}
