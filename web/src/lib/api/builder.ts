import { queryOptions } from "@tanstack/react-query";
import type { BuildSearch, Slot } from "#/lib/builder";
import { api, unwrap } from "./client";

export const buildCheckQuery = (build: BuildSearch) =>
	queryOptions({
		queryKey: ["builder", "check", build],
		queryFn: async ({ signal }) =>
			unwrap(
				await api.GET("/builder/check", { params: { query: build }, signal }),
			),
	});

// The options replace the slot's parts, so the slot's own picks are left out of the key: picking
// another CPU doesn't refetch the CPU list.
export const builderOptionsQuery = (slot: Slot, rest: BuildSearch) =>
	queryOptions({
		queryKey: ["builder", "options", slot, rest],
		queryFn: async ({ signal }) =>
			unwrap(
				await api.GET("/builder/options", {
					params: { query: { slot, ...rest } },
					signal,
				}),
			),
	});

export const savedBuildQuery = (slug: string) =>
	queryOptions({
		queryKey: ["build", slug],
		queryFn: async ({ signal }) =>
			unwrap(
				await api.GET("/builds/{slug}", { params: { path: { slug } }, signal }),
			),
	});

export async function saveBuild(build: BuildSearch) {
	return unwrap(await api.POST("/builds", { body: build }));
}
