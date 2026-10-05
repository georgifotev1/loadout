import { createFileRoute, Outlet } from "@tanstack/react-router";
import { ShopLayout } from "#/components/site/shop-layout";

// The storefront pages, inside the shop header and footer.
export const Route = createFileRoute("/_shop")({
	component: () => (
		<ShopLayout>
			<Outlet />
		</ShopLayout>
	),
});
