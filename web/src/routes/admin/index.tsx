import { createFileRoute } from "@tanstack/react-router";
import { m } from "#/i18n";

// /admin: the dashboard. TODO(phase-6): sync status, orders by state, discarded jobs and admin
// tasks (step 5).
export const Route = createFileRoute("/admin/")({
	head: () => ({ meta: [{ title: m.site.title(m.admin.dashboard.title) }] }),
	component: Dashboard,
});

function Dashboard() {
	return (
		<>
			<h1 className="font-semibold text-2xl tracking-[-0.02em]">
				{m.admin.dashboard.title}
			</h1>
			<p className="mt-2 text-muted-foreground text-sm">
				{m.admin.dashboard.lead}
			</p>
		</>
	);
}
