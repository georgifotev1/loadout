import { useSuspenseQuery } from "@tanstack/react-query";
import { createFileRoute, Link, useNavigate } from "@tanstack/react-router";
import { AdminPage } from "#/components/admin/admin-page";
import { adminColumns, DataTable } from "#/components/admin/data-table";
import { Pagination } from "#/components/site/pagination";
import { Badge } from "#/components/ui/badge";
import {
	Select,
	SelectContent,
	SelectItem,
	SelectTrigger,
	SelectValue,
} from "#/components/ui/select";
import { formatShortDate, m } from "#/i18n";
import {
	fieldLabel,
	parseSpecsSearch,
	reviewPageSize,
	type SpecsSearch,
} from "#/lib/admin-specs";
import { reviewQueueQuery } from "#/lib/api/admin-specs";
import type { Schemas } from "#/lib/api/client";
import { componentCategories } from "#/lib/categories";

type ReviewItem = Schemas["ReviewItem"];

const queueParams = (s: SpecsSearch) => ({
	category: s.category,
	page: s.page,
	page_size: reviewPageSize,
});

// /admin/specs: builder parts whose specs are incomplete, waiting longest first. Filling one
// in takes it out of the builder's "not offered" list.
export const Route = createFileRoute("/admin/specs/")({
	validateSearch: parseSpecsSearch,
	loaderDeps: ({ search }) => search,
	loader: ({ context, deps }) =>
		context.queryClient.query(reviewQueueQuery(queueParams(deps))),
	head: () => ({ meta: [{ title: m.site.title(m.admin.specs.title) }] }),
	component: ReviewQueuePage,
});

const col = adminColumns<ReviewItem>();
const columns = col.columns([
	col.accessor("product_name", {
		header: m.admin.specs.product,
		cell: (c) => <ProductLink item={c.row.original} />,
		meta: { className: "w-full min-w-72 whitespace-normal" },
	}),
	col.accessor("category", {
		header: m.admin.specs.category,
		cell: (c) => m.categories[c.getValue()],
		meta: { className: "text-muted-foreground" },
	}),
	col.accessor("missing", {
		header: m.admin.specs.missing,
		cell: (c) => (
			<div className="flex min-w-56 flex-wrap gap-1">
				{c.getValue().map((f) => (
					<Badge key={f} variant="secondary" className="font-normal">
						{fieldLabel(m.admin.specs.field, f)}
					</Badge>
				))}
			</div>
		),
		meta: { className: "whitespace-normal" },
	}),
	col.accessor("updated_at", {
		header: m.admin.specs.waiting,
		cell: (c) => formatShortDate(new Date(c.getValue())),
		meta: { className: "text-muted-foreground tabular-nums" },
	}),
]);

function ProductLink({ item }: { item: ReviewItem }) {
	const search = Route.useSearch();
	return (
		<Link
			to="/admin/specs/$productId"
			params={{ productId: item.product_id }}
			search={{ category: search.category }}
			className="font-medium hover:underline"
		>
			{item.product_name}
		</Link>
	);
}

const categoryItems = [
	{ value: "", label: m.admin.specs.allCategories },
	...componentCategories.map((c) => ({ value: c, label: m.categories[c] })),
];

function ReviewQueuePage() {
	const search = Route.useSearch();
	const navigate = useNavigate({ from: Route.fullPath });
	const { data } = useSuspenseQuery(reviewQueueQuery(queueParams(search)));

	return (
		<AdminPage
			title={m.admin.specs.title}
			lead={m.admin.specs.lead}
			actions={
				<Select
					items={categoryItems}
					value={search.category ?? ""}
					onValueChange={(v) =>
						navigate({
							search: {
								category: parseSpecsSearch({ category: v }).category,
							},
						})
					}
				>
					<SelectTrigger
						className="w-56 bg-card"
						aria-label={m.admin.specs.category}
					>
						<SelectValue />
					</SelectTrigger>
					<SelectContent align="end">
						{categoryItems.map((c) => (
							<SelectItem key={c.value} value={c.value}>
								{c.label}
							</SelectItem>
						))}
					</SelectContent>
				</Select>
			}
		>
			<p className="mb-3 text-muted-foreground text-sm">
				{m.admin.specs.count(data.total)}
			</p>
			<DataTable
				columns={columns}
				data={data.items}
				getRowId={(it) => it.product_id}
				empty={m.admin.specs.empty}
			/>
			<Pagination
				page={data.page}
				last={Math.ceil(data.total / reviewPageSize)}
				className="mt-6"
			/>
		</AdminPage>
	);
}
