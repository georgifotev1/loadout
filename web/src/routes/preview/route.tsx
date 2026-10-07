import { createFileRoute, Outlet } from "@tanstack/react-router";
import { SiteFooter } from "#/components/site/footer";
import { SiteHeader } from "#/components/site/header";
import { ShopSearch } from "#/components/site/shop-search";
import { PreviewCartProvider, usePreviewCart } from "./-cart";

// /preview: the design tokens on sample data. Not linked from anywhere.
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
	return (
		<div id="top" className="flex min-h-dvh flex-col">
			<SiteHeader
				search={<ShopSearch />}
				cartCount={cart.count}
				onCartClick={() => {}}
			/>
			<main className="flex-1">
				<Outlet />
			</main>
			<SiteFooter />
		</div>
	);
}
