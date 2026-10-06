import { queryOptions } from "@tanstack/react-query";
import { api, expectNoContent, type Schemas, unwrap } from "./client";
import type { operations } from "./schema.gen";

export type SyncRun = Schemas["SyncRun"];
export type DiscardedJob = Schemas["DiscardedJob"];
export type SyncRunsParams = NonNullable<
	operations["listSyncRuns"]["parameters"]["query"]
>;

export const overviewQuery = queryOptions({
	queryKey: ["admin", "overview"],
	queryFn: async ({ signal }) =>
		unwrap(await api.GET("/admin/overview", { signal })),
});

export const syncRunsQuery = (params: SyncRunsParams) =>
	queryOptions({
		queryKey: ["admin", "sync-runs", params],
		queryFn: async ({ signal }) =>
			unwrap(
				await api.GET("/admin/sync-runs", {
					params: { query: params },
					signal,
				}),
			),
	});

export const discardedJobsQuery = queryOptions({
	queryKey: ["admin", "jobs", "discarded"],
	queryFn: async ({ signal }) =>
		unwrap(
			await api.GET("/admin/jobs/discarded", {
				params: { query: { limit: 100 } },
				signal,
			}),
		),
});

/** Queues a supplier sync, or reports the one already waiting or running. */
export async function triggerSync() {
	return unwrap(await api.POST("/admin/sync"));
}

export async function retryJob(id: number) {
	expectNoContent(
		await api.POST("/admin/jobs/discarded/{job_id}/retry", {
			params: { path: { job_id: id } },
		}),
	);
}

export async function deleteJob(id: number) {
	expectNoContent(
		await api.DELETE("/admin/jobs/discarded/{job_id}", {
			params: { path: { job_id: id } },
		}),
	);
}
