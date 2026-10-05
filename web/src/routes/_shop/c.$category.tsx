import { useSuspenseQuery } from "@tanstack/react-query";
import { createFileRoute, notFound } from "@tanstack/react-router";
import { Listing } from "#/components/listing/listing";
import { Container } from "#/components/site/container";
import { PageBreadcrumb } from "#/components/site/page-breadcrumb";
import { m } from "#/i18n";
import { searchQuery } from "#/lib/api/products";
import { isCategory } from "#/lib/categories";
import { parseListingSearch, toSearchParams } from "#/lib/listing";

// /c/gpu: one category, with brand and price facets. The category is in the path, so it's
// not a filter here.
export const Route = createFileRoute("/_shop/c/$category")({
	validateSearch: (s) => {
		const { q: _, category: __, ...rest } = parseListingSearch(s);
		return rest;
	},
	beforeLoad: ({ params }) => {
		if (!isCategory(params.category)) throw notFound();
		return { category: params.category };
	},
	loaderDeps: ({ search }) => search,
	loader: ({ context, deps }) =>
		context.queryClient.query(
			searchQuery(toSearchParams({ ...deps, category: context.category })),
		),
	head: ({ params }) => {
		if (!isCategory(params.category)) return {};
		const name = m.categories[params.category];
		return {
			meta: [
				{ title: m.site.title(name) },
				{ name: "description", content: `${name}. ${m.site.description}` },
			],
			// Filtered and sorted variants are the same page to search engines.
			links: [{ rel: "canonical", href: `/c/${params.category}` }],
		};
	},
	component: CategoryPage,
});

function CategoryPage() {
	const { category } = Route.useRouteContext();
	const search = Route.useSearch();
	const { data } = useSuspenseQuery(
		searchQuery(toSearchParams({ ...search, category })),
	);

	return (
		<Container className="pt-6">
			<Listing
				title={m.categories[category]}
				breadcrumb={<PageBreadcrumb current={m.categories[category]} />}
				search={{ ...search, category }}
				result={data}
			/>
		</Container>
	);
}
