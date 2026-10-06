import {
	createFileRoute,
	notFound,
	Outlet,
	redirect,
} from "@tanstack/react-router";
import { AdminLayout } from "#/components/admin/admin-layout";
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
		// Not a page a customer can use; no need to tell them it's there.
		if (me.role !== "admin") throw notFound();
		return { me, sidebarOpen: sidebarOpen() };
	},
	head: () => ({
		meta: [
			{ title: m.site.title(m.admin.title) },
			{ name: "robots", content: "noindex" },
		],
	}),
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
