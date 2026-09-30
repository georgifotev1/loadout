import { useQuery } from "@tanstack/react-query";
import { createFileRoute } from "@tanstack/react-router";
import { healthQuery } from "#/lib/api/health";

// Phase 0 placeholder: proves the web app → Go API → Postgres path works, both during SSR
// (the loader runs on the server for the first request) and in the browser (via the /api proxy).
// TODO(phase-4): real storefront home page; strings move into i18n files.
export const Route = createFileRoute("/")({
	// query() rejects when the API is down; swallow it so the page renders and useQuery shows the error.
	loader: ({ context }) =>
		context.queryClient.query(healthQuery).catch(() => {}),
	component: Home,
});

function Home() {
	const { data, error, isFetching, refetch } = useQuery(healthQuery);

	return (
		<main className="mx-auto max-w-xl p-8">
			<h1 className="text-4xl font-bold">Loadout</h1>
			<p className="mt-2 text-muted-foreground">API health check</p>

			<section className="mt-6 rounded-lg border p-4">
				{error ? (
					<p className="text-destructive">API unreachable: {error.message}</p>
				) : data ? (
					<dl className="grid grid-cols-[auto_1fr] gap-x-4 gap-y-1 font-mono text-sm">
						<dt>status</dt>
						<dd
							className={
								data.status === "ok" ? "text-green-600" : "text-destructive"
							}
						>
							{data.status}
						</dd>
						<dt>database</dt>
						<dd>{data.database}</dd>
						<dt>version</dt>
						<dd>{data.version}</dd>
						<dt>env</dt>
						<dd>{data.env}</dd>
						<dt>time</dt>
						<dd>{data.time}</dd>
					</dl>
				) : (
					<p>Loading…</p>
				)}
			</section>

			<button
				type="button"
				onClick={() => refetch()}
				disabled={isFetching}
				className="mt-4 rounded-md border px-3 py-1.5 text-sm disabled:opacity-50"
			>
				{isFetching ? "Checking…" : "Check again"}
			</button>
		</main>
	);
}
