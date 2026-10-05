import { createFileRoute, Outlet, useLocation } from "@tanstack/react-router";
import { CheckoutHeader } from "#/components/site/checkout-header";
import { SiteFooter } from "#/components/site/footer";
import { SiteHeader } from "#/components/site/header";
import { PreviewCartProvider, usePreviewCart } from "./-cart";
import { PreviewSearch } from "./-search";
import { PreviewToolbar } from "./-toolbar";

// /preview: design previews of the storefront on sample data, for choosing tokens and layout.
// Not linked from anywhere; delete once the real pages exist.
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
					search={<PreviewSearch />}
					cartCount={cart.count}
					onCartClick={cart.openDrawer}
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
