import { useSuspenseQuery } from "@tanstack/react-query";
import { createFileRoute, Link, type LinkProps } from "@tanstack/react-router";
import { ArrowRight } from "lucide-react";
import { AdminPage } from "#/components/admin/admin-page";
import { AdminError, AdminNotFound } from "#/components/admin/admin-status";
import { SyncStatus } from "#/components/admin/sync-status";
import { formatRelativeTime, m } from "#/i18n";
import { overviewQuery } from "#/lib/api/admin";
import { cn } from "#/lib/utils";

// /admin: the dashboard. One card per thing that can need an admin, each linking to its page.
// TODO(phase-7): orders by state and open admin tasks.
export const Route = createFileRoute("/admin/")({
	loader: ({ context }) => context.queryClient.query(overviewQuery),
	head: () => ({ meta: [{ title: m.site.title(m.admin.dashboard.title) }] }),
	errorComponent: AdminError,
	notFoundComponent: AdminNotFound,
	component: Dashboard,
});

function Dashboard() {
	const { data } = useSuspenseQuery(overviewQuery);
	const t = m.admin.dashboard;

	return (
		<AdminPage title={t.title} lead={t.lead}>
			<div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
				<Card
					label={t.specs}
					to="/admin/specs"
					attention={data.specs_incomplete > 0}
					value={data.specs_incomplete}
					note={t.specsNote(data.specs_incomplete)}
				/>
				<Card
					label={t.lastSync}
					to="/admin/sync"
					attention={data.last_sync?.status === "failed"}
					value={data.last_sync ? <SyncStatus run={data.last_sync} /> : "—"}
					note={
						data.last_sync ? (
							<span suppressHydrationWarning>
								{formatRelativeTime(new Date(data.last_sync.started_at))}
							</span>
						) : (
							t.neverSynced
						)
					}
				/>
				<Card
					label={t.jobs}
					to="/admin/jobs"
					attention={data.discarded_jobs > 0}
					value={data.discarded_jobs}
					note={t.jobsNote(data.discarded_jobs)}
				/>
				<Card
					label={t.products}
					value={data.products.active}
					note={t.productsNote(data.products.inactive)}
				/>
			</div>
		</AdminPage>
	);
}

function Card({
	label,
	value,
	note,
	to,
	attention = false,
}: {
	label: string;
	value: React.ReactNode;
	note: React.ReactNode;
	to?: LinkProps["to"];
	attention?: boolean;
}) {
	const body = (
		<>
			<p className="flex items-center justify-between gap-2 text-muted-foreground text-sm">
				{label}
				{to ? (
					<ArrowRight
						aria-hidden
						className="size-4 text-subtle-foreground transition-transform group-hover:translate-x-0.5"
					/>
				) : null}
			</p>
			<p className="mt-3 font-semibold text-2xl tabular-nums tracking-[-0.02em]">
				{value}
			</p>
			<p className="mt-1 text-muted-foreground text-sm">{note}</p>
		</>
	);
	const className = cn(
		"group block rounded-lg border bg-card p-5",
		attention && "border-l-4 border-l-warning",
		to && "transition-colors hover:border-border-strong",
	);
	return to ? (
		<Link to={to} className={className}>
			{body}
		</Link>
	) : (
		<div className={className}>{body}</div>
	);
}
