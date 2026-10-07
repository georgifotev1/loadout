import { createFileRoute, Outlet } from "@tanstack/react-router";
import { CheckoutHeader } from "#/components/site/checkout-header";

// Checkout pages: only the checkout header, so nothing pulls the shopper away mid-order.
export const Route = createFileRoute("/_checkout")({
	component: () => (
		<div id="top" className="flex min-h-dvh flex-col">
			<CheckoutHeader />
			<main className="flex-1 pb-16">
				<Outlet />
			</main>
		</div>
	),
});
