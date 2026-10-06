import {
	createFileRoute,
	notFound,
	Outlet,
	redirect,
	rootRouteId,
} from "@tanstack/react-router";
import { AdminLayout } from "#/components/admin/admin-layout";
import { AdminNotFound } from "#/components/admin/admin-status";
import { ShopLayout } from "#/components/site/shop-layout";
import { PageError } from "#/components/site/status-page";
import { m } from "#/i18n";
import { meQuery, signedIn } from "#/lib/api/auth";
import { sidebarOpen } from "#/lib/sidebar-state";

// /admin: the back office. beforeLoad only spares the user an admin page that can't load: the Go
// API checks the session, role and second factor on every /api/admin call (requireAdmin).
// The admin's pages are split into their own chunks, so shoppers never download them.
export const Route = createFileRoute("/admin")({
	beforeLoad: async ({ context, location }) => {
		const me = await context.queryClient.query(meQuery);
		if (!signedIn(me)) {
			throw redirect({ to: "/login", search: { redirect: location.href } });
		}
		// Not a page a customer can use; no need to tell them it's there. The root's not-found
		// page is the shop's, where this route's own would show the admin's.
		if (me.role !== "admin") throw notFound({ routeId: rootRouteId });
		return { me, sidebarOpen: sidebarOpen() };
	},
	head: () => ({
		meta: [
			{ title: m.site.title(m.admin.title) },
			{ name: "robots", content: "noindex" },
		],
	}),
	// The session check failed (the API is down): there's no admin layout to show it in.
	errorComponent: (props) => (
		<ShopLayout>
			<PageError {...props} />
		</ShopLayout>
	),
	// An unknown /admin URL.
	notFoundComponent: AdminNotFound,
	component: AdminShell,
});

function AdminShell() {
	const { me, sidebarOpen } = Route.useRouteContext();
	return (
		<AdminLayout me={me} sidebarOpen={sidebarOpen}>
			<Outlet />
		</AdminLayout>
	);
}
