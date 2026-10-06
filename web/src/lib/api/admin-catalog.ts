import { queryOptions } from "@tanstack/react-query";
import { api, expectNoContent, type Schemas, unwrap } from "./client";
import type { operations } from "./schema.gen";

export type AdminProduct = Schemas["AdminProduct"];
export type AdminProductsParams = NonNullable<
	operations["listAdminProducts"]["parameters"]["query"]
>;
export type MarkupRule = Schemas["ListedMarkupRule"];
export type MarkupRuleInput = Schemas["MarkupRuleInput"];
export type RuleScope = Schemas["RuleScope"];
export type Rounding = Schemas["Rounding"];

export const adminProductsQuery = (params: AdminProductsParams) =>
	queryOptions({
		queryKey: ["admin", "products", params],
		queryFn: async ({ signal }) =>
			unwrap(
				await api.GET("/admin/products", { params: { query: params }, signal }),
			),
	});

export const markupRulesQuery = queryOptions({
	queryKey: ["admin", "markup-rules"],
	queryFn: async ({ signal }) =>
		unwrap(await api.GET("/admin/markup-rules", { signal })),
});

export async function createMarkupRule(body: MarkupRuleInput) {
	return unwrap(await api.POST("/admin/markup-rules", { body }));
}

/** Fails with `edit_conflict` when the rule changed since `version`. */
export async function updateMarkupRule(
	id: number,
	body: MarkupRuleInput & { version: number },
) {
	return unwrap(
		await api.PUT("/admin/markup-rules/{rule_id}", {
			params: { path: { rule_id: id } },
			body,
		}),
	);
}

export async function deleteMarkupRule(id: number) {
	expectNoContent(
		await api.DELETE("/admin/markup-rules/{rule_id}", {
			params: { path: { rule_id: id } },
		}),
	);
}
