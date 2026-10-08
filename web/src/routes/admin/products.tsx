import { useSuspenseQuery } from "@tanstack/react-query";
import { createFileRoute, Link, useNavigate } from "@tanstack/react-router";
import { AdminPage, StatusDot } from "#/components/admin/admin-page";
import { AdminError, AdminNotFound } from "#/components/admin/admin-status";
import { adminColumns, DataTable } from "#/components/admin/data-table";
import { SearchInput } from "#/components/admin/search-input";
import { Pagination } from "#/components/site/pagination";
import {
	Select,
	SelectContent,
	SelectItem,
	SelectTrigger,
	SelectValue,
} from "#/components/ui/select";
import { formatPercent, formatPrice, m } from "#/i18n";
import {
	type ProductsSearch,
	parseProductsSearch,
	productsPageSize,
} from "#/lib/admin-products";
import { type AdminProduct, adminProductsQuery } from "#/lib/api/admin-catalog";
import { categories } from "#/lib/categories";
import { cn } from "#/lib/utils";

const queryParams = (s: ProductsSearch) => ({
	...s,
	page_size: productsPageSize,
});

// /admin/products: every product with what the shop doesn't show: dealer price, margin, the
// markup rule behind the price, and products the supplier dropped.
export const Route = createFileRoute("/admin/products")({
	validateSearch: parseProductsSearch,
	loaderDeps: ({ search }) => search,
	loader: ({ context, deps }) =>
		context.queryClient.query(adminProductsQuery(queryParams(deps))),
	head: () => ({ meta: [{ title: m.site.title(m.admin.products.title) }] }),
	errorComponent: AdminError,
	notFoundComponent: AdminNotFound,
	component: ProductsPage,
});

const t = m.admin.products;

const money =
	(pick: (p: NonNullable<AdminProduct["price"]>) => number) =>
	({ row }: { row: { original: AdminProduct } }) =>
		row.original.price ? formatPrice(pick(row.original.price)) : "—";

const col = adminColumns<AdminProduct>();
const columns = col.columns([
	col.accessor("name", {
		header: t.product,
		cell: (c) => <ProductCell product={c.row.original} />,
		meta: { className: "w-full min-w-72 whitespace-normal" },
	}),
	col.accessor("category", {
		header: t.category,
		cell: (c) => m.categories[c.getValue()],
		meta: { className: "text-muted-foreground" },
	}),
	col.display({
		id: "dealer",
		header: t.dealer,
		cell: money((p) => p.dealer_cents),
		meta: { className: "text-right tabular-nums text-muted-foreground" },
	}),
	col.display({
		id: "retail",
		header: t.retail,
		cell: money((p) => p.retail_cents),
		meta: { className: "text-right tabular-nums font-medium" },
	}),
	col.display({
		id: "margin",
		header: t.margin,
		cell: (c) => {
			const p = c.row.original.price;
			if (!p) return "—";
			return (
				<>
					{formatPrice(p.margin_cents)}
					<span className="ml-1.5 text-subtle-foreground">
						{formatPercent(p.margin_bp)}
					</span>
				</>
			);
		},
		meta: { className: "text-right tabular-nums" },
	}),
	col.display({
		id: "rule",
		header: t.rule,
		cell: (c) => <RuleCell product={c.row.original} />,
	}),
	col.accessor("stock_qty", {
		header: t.stock,
		cell: (c) => (
			<span className={cn(c.getValue() === 0 && "text-destructive")}>
				{c.getValue()}
			</span>
		),
		meta: { className: "text-right tabular-nums" },
	}),
	col.accessor("specs_status", {
		header: t.specs,
		cell: (c) => <SpecsCell product={c.row.original} />,
	}),
	col.accessor("active", {
		header: t.status,
		cell: (c) =>
			c.getValue() ? (
				<StatusDot tone="success">{t.active}</StatusDot>
			) : (
				<StatusDot tone="muted">{t.inactive}</StatusDot>
			),
	}),
]);

function ProductCell({ product: p }: { product: AdminProduct }) {
	// Only active, priced products have a shop page.
	const name =
		p.active && p.price ? (
			<Link
				to="/p/$slug"
				params={{ slug: p.slug }}
				target="_blank"
				aria-label={t.viewInShop(p.name)}
				className="font-medium hover:underline"
			>
				{p.name}
			</Link>
		) : (
			<span className="font-medium">{p.name}</span>
		);
	return (
		<div className="flex flex-col gap-0.5">
			{name}
			<span className="text-subtle-foreground text-xs">
				{p.brand} · {p.sku}
			</span>
		</div>
	);
}

function RuleCell({ product: p }: { product: AdminProduct }) {
	if (!p.price)
		return <span className="text-muted-foreground">{t.unpriced}</span>;
	if (!p.rule) {
		return <span className="text-muted-foreground">{t.ruleDeleted}</span>;
	}
	return (
		<Link to="/admin/markup-rules" className="hover:underline">
			{m.admin.markupRules.scopes[p.rule.scope]}
		</Link>
	);
}

function SpecsCell({ product: p }: { product: AdminProduct }) {
	switch (p.specs_status) {
		case "approved":
			return <StatusDot tone="success">{t.specsApproved}</StatusDot>;
		case "incomplete":
			return (
				<Link
					to="/admin/specs/$productId"
					params={{ productId: p.id }}
					search={{ category: undefined }}
					className="hover:underline"
				>
					<StatusDot tone="warning">{t.specsIncomplete}</StatusDot>
				</Link>
			);
		default:
			return <span className="text-muted-foreground">—</span>;
	}
}

const categoryItems = [
	{ value: "", label: t.allCategories },
	...categories.map((c) => ({ value: c, label: m.categories[c] })),
];

const statusItems = [
	{ value: "", label: t.allStatuses },
	{ value: "active", label: t.statusActive },
	{ value: "inactive", label: t.statusInactive },
];

function ProductsPage() {
	const search = Route.useSearch();
	const navigate = useNavigate({ from: Route.fullPath });
	const { data } = useSuspenseQuery(adminProductsQuery(queryParams(search)));

	// A filter change starts again from page 1.
	const filter = (patch: Record<string, unknown>) =>
		navigate({
			search: (prev) =>
				parseProductsSearch({ ...prev, ...patch, page: undefined }),
		});

	return (
		<AdminPage title={t.title} lead={t.lead}>
			<div className="mb-3 flex flex-wrap items-center gap-2">
				<SearchInput
					value={search.q ?? ""}
					onChange={(q) => filter({ q })}
					label={t.search}
				/>
				<Select
					items={categoryItems}
					value={search.category ?? ""}
					onValueChange={(category) => filter({ category })}
				>
					<SelectTrigger className="w-52 bg-card" aria-label={t.category}>
						<SelectValue />
					</SelectTrigger>
					<SelectContent>
						{categoryItems.map((c) => (
							<SelectItem key={c.value} value={c.value}>
								{c.label}
							</SelectItem>
						))}
					</SelectContent>
				</Select>
				<Select
					items={statusItems}
					value={search.status ?? ""}
					onValueChange={(status) => filter({ status })}
				>
					<SelectTrigger className="w-48 bg-card" aria-label={t.status}>
						<SelectValue />
					</SelectTrigger>
					<SelectContent>
						{statusItems.map((s) => (
							<SelectItem key={s.value} value={s.value}>
								{s.label}
							</SelectItem>
						))}
					</SelectContent>
				</Select>
				<p className="ml-auto text-muted-foreground text-sm">
					{t.count(data.total)}
				</p>
			</div>
			<DataTable
				columns={columns}
				data={data.items}
				getRowId={(p) => p.id}
				empty={t.empty}
			/>
			<p className="mt-3 max-w-3xl text-muted-foreground text-xs">{t.hint}</p>
			<Pagination
				page={data.page}
				last={Math.ceil(data.total / productsPageSize)}
				className="mt-6"
			/>
		</AdminPage>
	);
}
