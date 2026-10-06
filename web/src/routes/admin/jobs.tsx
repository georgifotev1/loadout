import {
	useMutation,
	useQueryClient,
	useSuspenseQuery,
} from "@tanstack/react-query";
import { createFileRoute } from "@tanstack/react-router";
import { RotateCcw, Trash2 } from "lucide-react";
import { useState } from "react";
import { AdminPage } from "#/components/admin/admin-page";
import { AdminError, AdminNotFound } from "#/components/admin/admin-status";
import { adminColumns, DataTable } from "#/components/admin/data-table";
import {
	AlertDialog,
	AlertDialogAction,
	AlertDialogCancel,
	AlertDialogContent,
	AlertDialogDescription,
	AlertDialogFooter,
	AlertDialogHeader,
	AlertDialogTitle,
	AlertDialogTrigger,
} from "#/components/ui/alert-dialog";
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
import {
	type DiscardedJob,
	deleteJob,
	discardedJobsQuery,
	retryJob,
} from "#/lib/api/admin";
import { ApiError } from "#/lib/api/client";

// /admin/jobs: background jobs that failed every attempt (River's dead-letter queue), to run
// again or delete.
export const Route = createFileRoute("/admin/jobs")({
	loader: ({ context }) => context.queryClient.query(discardedJobsQuery),
	head: () => ({ meta: [{ title: m.site.title(m.admin.jobs.title) }] }),
	errorComponent: AdminError,
	notFoundComponent: AdminNotFound,
	component: JobsPage,
});

const t = m.admin.jobs;

const kindLabel = (kind: string) => t.kinds[kind] ?? kind;
const lastError = (j: DiscardedJob) => j.errors.at(-1)?.error ?? "";

const col = adminColumns<DiscardedJob>();
const columns = col.columns([
	col.accessor("kind", {
		header: t.job,
		cell: (c) => (
			<>
				<span className="font-medium">{kindLabel(c.getValue())}</span>
				<span className="ml-2 text-subtle-foreground tabular-nums">
					#{c.row.original.id}
				</span>
			</>
		),
	}),
	col.display({
		id: "attempts",
		header: t.attempts,
		cell: (c) => `${c.row.original.attempt} / ${c.row.original.max_attempts}`,
		meta: { className: "text-muted-foreground tabular-nums" },
	}),
	col.display({
		id: "error",
		header: t.lastError,
		cell: (c) => <ErrorCell job={c.row.original} />,
		meta: { className: "w-full min-w-72 whitespace-normal" },
	}),
	col.accessor("finalized_at", {
		header: t.finished,
		cell: (c) => {
			const v = c.getValue();
			return v ? formatDateTime(new Date(v)) : "—";
		},
		meta: { className: "text-muted-foreground tabular-nums" },
	}),
	col.display({
		id: "actions",
		header: () => <span className="sr-only">{t.actions}</span>,
		cell: (c) => <JobActions job={c.row.original} />,
	}),
]);

function JobsPage() {
	const { data } = useSuspenseQuery(discardedJobsQuery);

	return (
		<AdminPage title={t.title} lead={t.lead}>
			<p className="mb-3 text-muted-foreground text-sm">
				{t.count(data.items.length, data.total)}
			</p>
			<DataTable
				columns={columns}
				data={data.items}
				getRowId={(j) => String(j.id)}
				empty={t.empty}
			/>
		</AdminPage>
	);
}

// ErrorCell shows the start of the last error; "details" opens the job's args and every
// attempt's error in full.
function ErrorCell({ job }: { job: DiscardedJob }) {
	return (
		<div className="flex items-start gap-3">
			<span className="line-clamp-2 min-w-0 flex-1 break-all font-mono text-destructive text-xs">
				{lastError(job)}
			</span>
			<Dialog>
				<DialogTrigger render={<Button variant="link" size="xs" />}>
					{t.details}
				</DialogTrigger>
				<DialogContent className="sm:max-w-2xl">
					<DialogHeader>
						<DialogTitle>
							{kindLabel(job.kind)}{" "}
							<span className="font-normal text-subtle-foreground">
								#{job.id}
							</span>
						</DialogTitle>
						<DialogDescription>
							{job.queue} · {formatDateTime(new Date(job.created_at))}
						</DialogDescription>
					</DialogHeader>
					<section>
						<h3 className="mb-1.5 font-medium">{t.args}</h3>
						<pre className="overflow-x-auto rounded-md bg-muted p-3 font-mono text-xs">
							{JSON.stringify(job.args, null, 2)}
						</pre>
					</section>
					<section>
						<h3 className="mb-1.5 font-medium">{t.errors}</h3>
						<ol className="flex flex-col gap-3">
							{job.errors.map((e) => (
								<li key={`${e.attempt}-${e.at}`}>
									<p className="text-muted-foreground text-xs">
										{t.attempt(e.attempt)} · {formatDateTime(new Date(e.at))}
									</p>
									<pre className="mt-1 overflow-x-auto whitespace-pre-wrap break-all rounded-md bg-muted p-3 font-mono text-xs">
										{e.error}
									</pre>
								</li>
							))}
						</ol>
					</section>
				</DialogContent>
			</Dialog>
		</div>
	);
}

function JobActions({ job }: { job: DiscardedJob }) {
	const queryClient = useQueryClient();
	const onSuccess = () => {
		queryClient.invalidateQueries({ queryKey: ["admin", "jobs"] });
		queryClient.invalidateQueries({ queryKey: ["admin", "overview"] });
		queryClient.invalidateQueries({ queryKey: ["admin", "sync-runs"] });
	};
	const retry = useMutation({ mutationFn: () => retryJob(job.id), onSuccess });
	const remove = useMutation({
		mutationFn: () => deleteJob(job.id),
		onSuccess,
	});
	const error = retry.error ?? remove.error;
	const [confirming, setConfirming] = useState(false);

	return (
		<div className="flex items-center justify-end gap-1">
			{error ? (
				<span role="alert" className="mr-1 max-w-48 text-destructive text-xs">
					{error instanceof ApiError && error.code === "already_queued"
						? t.alreadyQueued
						: t.failed}
				</span>
			) : null}
			<Button
				variant="outline"
				size="sm"
				disabled={retry.isPending || remove.isPending}
				onClick={() => retry.mutate()}
			>
				<RotateCcw />
				{t.retry}
			</Button>
			<AlertDialog open={confirming} onOpenChange={setConfirming}>
				<AlertDialogTrigger
					render={
						<Button
							variant="ghost"
							size="icon-sm"
							aria-label={t.delete}
							title={t.delete}
							disabled={retry.isPending || remove.isPending}
						/>
					}
				>
					<Trash2 />
				</AlertDialogTrigger>
				<AlertDialogContent>
					<AlertDialogHeader>
						<AlertDialogTitle>{t.deleteTitle}</AlertDialogTitle>
						<AlertDialogDescription>
							{kindLabel(job.kind)} #{job.id}. {t.deleteBody}
						</AlertDialogDescription>
					</AlertDialogHeader>
					<AlertDialogFooter>
						<AlertDialogCancel>{t.cancel}</AlertDialogCancel>
						<AlertDialogAction
							variant="destructive"
							onClick={() => {
								setConfirming(false);
								remove.mutate();
							}}
						>
							{t.delete}
						</AlertDialogAction>
					</AlertDialogFooter>
				</AlertDialogContent>
			</AlertDialog>
		</div>
	);
}
