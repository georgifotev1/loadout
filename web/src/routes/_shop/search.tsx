import { useSuspenseQuery } from "@tanstack/react-query";
import { createFileRoute } from "@tanstack/react-router";
import { Listing } from "#/components/listing/listing";
import { Container } from "#/components/site/container";
import { PageBreadcrumb } from "#/components/site/page-breadcrumb";
import { m } from "#/i18n";
import { searchQuery } from "#/lib/api/products";
import { parseListingSearch, toSearchParams } from "#/lib/listing";

// /search?q=rtx: the full results for the header search, with a category facet.
export const Route = createFileRoute("/_shop/search")({
	validateSearch: parseListingSearch,
	loaderDeps: ({ search }) => search,
	loader: ({ context, deps }) =>
		context.queryClient.query(searchQuery(toSearchParams(deps))),
	head: ({ match }) => ({
		meta: [
			{ title: m.site.title(title(match.search.q)) },
			// Result pages are endless and thin; the category pages are what should be indexed.
			{ name: "robots", content: "noindex, follow" },
		],
	}),
	component: SearchPage,
});

function title(q?: string) {
	return q ? m.listing.searchTitle(q) : m.listing.allProducts;
}

function SearchPage() {
	const search = Route.useSearch();
	const { data } = useSuspenseQuery(searchQuery(toSearchParams(search)));

	return (
		<Container className="pt-6">
			<Listing
				title={title(search.q)}
				breadcrumb={<PageBreadcrumb current={m.search.label} />}
				search={search}
				result={data}
				categoryFacet
			/>
		</Container>
	);
}
