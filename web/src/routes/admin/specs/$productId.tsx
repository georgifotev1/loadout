import { useSuspenseQuery } from "@tanstack/react-query";
import { createFileRoute, notFound } from "@tanstack/react-router";
import { SpecsEditor } from "#/components/admin/specs-editor";
import { m } from "#/i18n";
import { parseSpecsSearch } from "#/lib/admin-specs";
import { productSpecsQuery } from "#/lib/api/admin-specs";
import { ApiError } from "#/lib/api/client";

// /admin/specs/:productId: fill in or correct one builder part's canonical specs.
export const Route = createFileRoute("/admin/specs/$productId")({
	validateSearch: (s) => ({ category: parseSpecsSearch(s).category }),
	loader: async ({ context, params }) => {
		try {
			return await context.queryClient.query(
				productSpecsQuery(params.productId),
			);
		} catch (err) {
			// 404: a peripheral or unknown product; 422: not a product ID at all.
			if (
				err instanceof ApiError &&
				(err.status === 404 || err.status === 422)
			) {
				throw notFound();
			}
			throw err;
		}
	},
	head: ({ loaderData }) => ({
		meta: [
			{ title: m.site.title(loaderData?.product_name ?? m.admin.specs.title) },
		],
	}),
	component: SpecsPage,
});

function SpecsPage() {
	const { productId } = Route.useParams();
	const search = Route.useSearch();
	const { data } = useSuspenseQuery(productSpecsQuery(productId));
	// A new product starts with fresh drafts.
	return <SpecsEditor key={productId} specs={data} search={search} />;
}
