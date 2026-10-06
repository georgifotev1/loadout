import {
	useMutation,
	useQueryClient,
	useSuspenseQuery,
} from "@tanstack/react-query";
import { createFileRoute } from "@tanstack/react-router";
import { RefreshCw } from "lucide-react";
import { AdminPage } from "#/components/admin/admin-page";
import { adminColumns, DataTable } from "#/components/admin/data-table";
import { SyncStatus } from "#/components/admin/sync-status";
import { Pagination } from "#/components/site/pagination";
import { Button } from "#/components/ui/button";
import {
	Dialog,
	DialogContent,
	DialogDescription,
	DialogHeader,
	DialogTitle,
	DialogTrigger,
} from "#/components/ui/dialog";
import { formatDateTime, m } from "#/i18n";
import { type SyncRun, syncRunsQuery, triggerSync } from "#/lib/api/admin";
import { wholeNumber } from "#/lib/listing";
import { cn } from "#/lib/utils";

const pageSize = 25;
// How often the list refreshes while a sync is waiting or running.
const pollMs = 3000;

const runsParams = (page?: number) => ({ page, page_size: pageSize });

// /admin/sync: the supplier sync history and "sync now".
export const Route = createFileRoute("/admin/sync")({
	validateSearch: (s): { page?: number } => ({ page: wholeNumber(s.page, 2) }),
	loaderDeps: ({ search }) => search,
	loader: ({ context, deps }) =>
		context.queryClient.query(syncRunsQuery(runsParams(deps.page))),
	head: () => ({ meta: [{ title: m.site.title(m.admin.sync.title) }] }),
	component: SyncPage,
});

const t = m.admin.sync;

const col = adminColumns<SyncRun>();
const columns = col.columns([
	col.accessor("started_at", {
		header: t.started,
		cell: (c) => formatDateTime(new Date(c.getValue())),
		meta: { className: "tabular-nums" },
	}),
	col.display({
		id: "duration",
		header: t.duration,
		cell: (c) => {
			const run = c.row.original;
			if (!run.finished_at) return "—";
			const ms = Date.parse(run.finished_at) - Date.parse(run.started_at);
			return m.admin.duration(ms / 1000);
		},
		meta: { className: "text-muted-foreground tabular-nums" },
	}),
	col.accessor("status", {
		header: t.status,
		cell: (c) => {
			const run = c.row.original;
			return (
				<div className="flex items-center gap-3">
					<SyncStatus run={run} />
					{run.error_message ? (
						<ErrorDetails run={run} message={run.error_message} />
					) : null}
				</div>
			);
		},
		// The spare width goes here, so the counts line up on the right.
		meta: { className: "w-full" },
	}),
	col.accessor("fetched", {
		header: t.fetched,
		meta: { className: "text-right tabular-nums" },
	}),
	col.accessor("changed", {
		header: t.changed,
		meta: { className: "text-right tabular-nums" },
	}),
	col.accessor("errors", {
		header: t.skipped,
		cell: (c) => (
			<span className={cn(c.getValue() > 0 && "font-medium text-destructive")}>
				{c.getValue()}
			</span>
		),
		meta: { className: "text-right tabular-nums" },
	}),
]);

function SyncPage() {
	const { page = 1 } = Route.useSearch();
	const queryClient = useQueryClient();
	const { data } = useSuspenseQuery({
		...syncRunsQuery(runsParams(page)),
		refetchInterval: (q) => (inProgress(q.state.data) ? pollMs : false),
	});
	const busy = inProgress(data);

	const sync = useMutation({
		mutationFn: triggerSync,
		onSettled: () => {
			queryClient.invalidateQueries({ queryKey: ["admin", "sync-runs"] });
			queryClient.invalidateQueries({ queryKey: ["admin", "overview"] });
		},
	});

	return (
		<AdminPage
			title={t.title}
			lead={t.lead}
			actions={
				<Button onClick={() => sync.mutate()} disabled={busy || sync.isPending}>
					<RefreshCw className={cn(busy && "motion-safe:animate-spin")} />
					{busy ? t.inProgress : t.now}
				</Button>
			}
		>
			<div className="mb-3 flex flex-wrap items-baseline justify-between gap-x-4 gap-y-1 text-sm">
				<p className="text-muted-foreground">{t.count(data.total)}</p>
				<output className={cn(sync.isError && "text-destructive")}>
					{syncMessage(sync)}
				</output>
			</div>
			<DataTable
				columns={columns}
				data={data.items}
				getRowId={(r) => String(r.id)}
				empty={t.empty}
			/>
			<p className="mt-3 max-w-3xl text-muted-foreground text-xs">{t.hint}</p>
			<Pagination
				page={data.page}
				last={Math.ceil(data.total / pageSize)}
				className="mt-6"
			/>
		</AdminPage>
	);
}

// ErrorDetails opens why a run failed. The message is for developers, so it stays out of the
// table.
function ErrorDetails({ run, message }: { run: SyncRun; message: string }) {
	return (
		<Dialog>
			<DialogTrigger render={<Button variant="link" size="xs" />}>
				{t.details}
			</DialogTrigger>
			<DialogContent className="sm:max-w-2xl">
				<DialogHeader>
					<DialogTitle>{t.errorTitle}</DialogTitle>
					<DialogDescription>
						{formatDateTime(new Date(run.started_at))}
					</DialogDescription>
				</DialogHeader>
				<pre className="overflow-x-auto whitespace-pre-wrap break-all rounded-md bg-muted p-3 font-mono text-xs">
					{message}
				</pre>
			</DialogContent>
		</Dialog>
	);
}

function syncMessage(sync: {
	isError: boolean;
	data?: { already_queued: boolean };
}) {
	if (sync.isError) return t.failed;
	if (sync.data) return sync.data.already_queued ? t.alreadyQueued : t.queued;
	return null;
}

function inProgress(data?: { queued: boolean; items: SyncRun[] }) {
	return !!data && (data.queued || data.items[0]?.status === "running");
}
