import { Link, useNavigate } from "@tanstack/react-router";
import { ChevronLeft, ChevronRight, SlidersHorizontal, X } from "lucide-react";
import { useState } from "react";
import { ProductCard } from "#/components/product/product-card";
import { Button } from "#/components/ui/button";
import { Checkbox } from "#/components/ui/checkbox";
import {
	Drawer,
	DrawerClose,
	DrawerContent,
	DrawerPopup,
	DrawerTitle,
} from "#/components/ui/drawer";
import { Label } from "#/components/ui/label";
import {
	Select,
	SelectContent,
	SelectItem,
	SelectTrigger,
	SelectValue,
} from "#/components/ui/select";
import { formatPrice, m } from "#/i18n";
import type { Schemas } from "#/lib/api/client";
import { type ListingSearch, pageNumbers, pageSize } from "#/lib/listing";
import { cn } from "#/lib/utils";

type SearchResult = Schemas["SearchResult"];
type PriceBucket = Schemas["PriceBucket"];

const sortOptions = [
	{ value: "relevance", label: m.listing.sortRelevance },
	{ value: "price_asc", label: m.listing.sortPriceAsc },
	{ value: "price_desc", label: m.listing.sortPriceDesc },
];

// Listing is the body of the category and search pages: the facets, sort, product grid and
// pagination over one GET /api/search result. All its state is in the URL (ListingSearch),
// so changing a filter is a navigation and the page stays shareable.
export function Listing({
	title,
	breadcrumb,
	search,
	result,
	categoryFacet = false,
}: {
	title: string;
	breadcrumb: React.ReactNode;
	search: ListingSearch;
	result: SearchResult;
	// Show the category facet: on the search page, not on a category page.
	categoryFacet?: boolean;
}) {
	const navigate = useNavigate();
	const [filtersOpen, setFiltersOpen] = useState(false);
	const { facets } = result;

	// Any filter change starts over at page 1.
	const update = (change: Partial<ListingSearch>) =>
		navigate({
			to: ".",
			search: (prev: ListingSearch) => ({
				...prev,
				...change,
				page: undefined,
			}),
			resetScroll: false,
		});

	const brands = search.brand ?? [];
	const toggleBrand = (b: string) =>
		update({
			brand: brands.includes(b)
				? brands.filter((x) => x !== b)
				: [...brands, b],
		});
	const priceSelected = (b: PriceBucket) =>
		search.price_min === b.min_cents && search.price_max === b.max_cents;
	const togglePrice = (b: PriceBucket) =>
		update(
			priceSelected(b)
				? { price_min: undefined, price_max: undefined }
				: { price_min: b.min_cents, price_max: b.max_cents },
		);

	const chips = [
		...(categoryFacet && search.category
			? [
					{
						label: m.categories[search.category],
						remove: () => update({ category: undefined }),
					},
				]
			: []),
		...brands.map((b) => ({ label: b, remove: () => toggleBrand(b) })),
		...(search.price_min !== undefined || search.price_max !== undefined
			? [
					{
						label: priceLabel(search.price_min ?? 0, search.price_max),
						remove: () =>
							update({ price_min: undefined, price_max: undefined }),
					},
				]
			: []),
	];
	const clearAll = () =>
		update({
			category: categoryFacet ? undefined : search.category,
			brand: undefined,
			price_min: undefined,
			price_max: undefined,
		});

	// Rendered in the sidebar on desktop and in the filter sheet on phones.
	const facetGroups = (prefix: string) => (
		<>
			{categoryFacet && facets.categories.length > 0 ? (
				<FacetGroup title={m.listing.category}>
					{facets.categories.map((c) => (
						<FacetCheckbox
							key={c.value}
							id={`${prefix}-category-${c.value}`}
							label={m.categories[c.value]}
							count={c.count}
							checked={search.category === c.value}
							onChange={() =>
								update({
									category: search.category === c.value ? undefined : c.value,
								})
							}
						/>
					))}
				</FacetGroup>
			) : null}
			{facets.brands.length > 0 ? (
				<FacetGroup title={m.listing.brand}>
					{facets.brands.map((b) => (
						<FacetCheckbox
							key={b.value}
							id={`${prefix}-brand-${b.value}`}
							label={b.value}
							count={b.count}
							checked={brands.includes(b.value)}
							onChange={() => toggleBrand(b.value)}
						/>
					))}
				</FacetGroup>
			) : null}
			{facets.prices.length > 0 ? (
				<FacetGroup title={m.listing.price}>
					{facets.prices.map((b) => (
						<FacetCheckbox
							key={b.min_cents}
							id={`${prefix}-price-${b.min_cents}`}
							label={priceLabel(b.min_cents, b.max_cents)}
							count={b.count}
							checked={priceSelected(b)}
							onChange={() => togglePrice(b)}
						/>
					))}
				</FacetGroup>
			) : null}
		</>
	);

	return (
		<>
			{breadcrumb}

			<div className="mt-4 flex flex-wrap items-end justify-between gap-4 border-b pb-6">
				<div className="min-w-0">
					<h1 className="text-balance font-semibold text-3xl tracking-[-0.03em]">
						{title}
					</h1>
					<p className="mt-1 text-muted-foreground text-sm tabular-nums">
						{m.listing.results(result.total)}
					</p>
				</div>
				<div className="flex items-center gap-2">
					<Button
						variant="outline"
						className="bg-card lg:hidden"
						onClick={() => setFiltersOpen(true)}
					>
						<SlidersHorizontal /> {m.listing.filters}
					</Button>
					<Select
						items={sortOptions}
						value={search.sort ?? "relevance"}
						onValueChange={(v) =>
							update({
								sort: v === "price_asc" || v === "price_desc" ? v : undefined,
							})
						}
					>
						<SelectTrigger
							className="w-48 bg-card sm:w-56"
							aria-label={m.listing.sort}
						>
							<SelectValue />
						</SelectTrigger>
						<SelectContent align="end">
							{sortOptions.map((o) => (
								<SelectItem key={o.value} value={o.value}>
									{o.label}
								</SelectItem>
							))}
						</SelectContent>
					</Select>
				</div>
			</div>

			<div className="grid gap-8 pt-6 lg:grid-cols-[15rem_1fr]">
				<aside className="hidden lg:block">
					<div className="sticky top-[calc(var(--spacing-header)+2.5rem+1.5rem)] flex flex-col gap-8">
						{facetGroups("side")}
					</div>
				</aside>

				<div>
					{chips.length > 0 ? (
						<div className="mb-4 flex flex-wrap items-center gap-2">
							{chips.map((c) => (
								<ActiveChip key={c.label} label={c.label} onRemove={c.remove} />
							))}
							<button
								type="button"
								onClick={clearAll}
								className="px-2 text-muted-foreground text-sm hover:text-foreground"
							>
								{m.listing.clearFilters}
							</button>
						</div>
					) : null}

					{result.products.length > 0 ? (
						<div className="grid grid-cols-2 gap-3 md:grid-cols-3">
							{result.products.map((p) => (
								<ProductCard key={p.slug} product={p} />
							))}
						</div>
					) : (
						<p className="rounded-lg border bg-card px-5 py-10 text-center text-muted-foreground text-sm">
							{search.q && chips.length === 0
								? m.listing.emptySearch(search.q)
								: m.listing.empty}
						</p>
					)}

					<Pagination
						page={result.page}
						last={Math.ceil(result.total / pageSize)}
					/>
				</div>
			</div>

			<Drawer side="bottom" open={filtersOpen} onOpenChange={setFiltersOpen}>
				<DrawerPopup side="bottom">
					<DrawerContent className="flex min-h-0 flex-1 flex-col">
						<DrawerTitle className="px-5 pt-2 pb-3 font-medium">
							{m.listing.filters}
						</DrawerTitle>
						<div className="flex min-h-0 flex-1 flex-col gap-8 overflow-y-auto overscroll-contain border-t px-5 py-5">
							{facetGroups("sheet")}
						</div>
						<div className="border-t px-5 pt-4">
							<DrawerClose
								render={<Button size="lg" className="h-11 w-full" />}
							>
								{m.listing.results(result.total)}
							</DrawerClose>
						</div>
					</DrawerContent>
				</DrawerPopup>
			</Drawer>
		</>
	);
}

function priceLabel(min: number, max?: number) {
	return max === undefined
		? m.listing.priceFrom(formatPrice(min))
		: `${formatPrice(min)} – ${formatPrice(max)}`;
}

// Pagination links to the other pages, so crawlers can follow them too.
function Pagination({ page, last }: { page: number; last: number }) {
	if (last <= 1) return null;
	const to = (n: number) => (prev: ListingSearch) => ({
		...prev,
		page: n === 1 ? undefined : n,
	});
	return (
		<nav
			aria-label={m.listing.pages}
			className="mt-10 flex items-center justify-center gap-1"
		>
			<Button
				variant="ghost"
				size="sm"
				disabled={page <= 1}
				render={page > 1 ? <Link to="." search={to(page - 1)} /> : undefined}
			>
				<ChevronLeft />
				<span className="hidden sm:inline">{m.listing.previous}</span>
			</Button>
			{pageNumbers(page, last).map((n, i) =>
				n === null ? (
					<span
						// biome-ignore lint/suspicious/noArrayIndexKey: a gap has no identity
						key={`gap-${i}`}
						className="px-1 text-subtle-foreground"
					>
						…
					</span>
				) : (
					<Button
						key={n}
						variant={n === page ? "outline" : "ghost"}
						size="icon-sm"
						className={cn("tabular-nums", n === page && "bg-card")}
						aria-label={m.listing.page(n)}
						aria-current={n === page ? "page" : undefined}
						render={<Link to="." search={to(n)} />}
					>
						{n}
					</Button>
				),
			)}
			<Button
				variant="ghost"
				size="sm"
				disabled={page >= last}
				render={page < last ? <Link to="." search={to(page + 1)} /> : undefined}
			>
				<span className="hidden sm:inline">{m.listing.next}</span>
				<ChevronRight />
			</Button>
		</nav>
	);
}

function FacetGroup({
	title,
	children,
}: {
	title: string;
	children: React.ReactNode;
}) {
	return (
		<fieldset>
			<legend className="mb-3 font-medium text-sm">{title}</legend>
			<div className="flex flex-col gap-2.5">{children}</div>
		</fieldset>
	);
}

function FacetCheckbox({
	id,
	label,
	count,
	checked,
	onChange,
}: {
	id: string;
	label: string;
	count: number;
	checked: boolean;
	onChange: () => void;
}) {
	return (
		<div className="flex items-center gap-2.5">
			<Checkbox id={id} checked={checked} onCheckedChange={onChange} />
			<Label
				htmlFor={id}
				className="flex-1 cursor-pointer font-normal text-muted-foreground text-sm"
			>
				{label}
			</Label>
			<span className="text-subtle-foreground text-xs tabular-nums">
				{count}
			</span>
		</div>
	);
}

function ActiveChip({
	label,
	onRemove,
}: {
	label: string;
	onRemove: () => void;
}) {
	return (
		<button
			type="button"
			onClick={onRemove}
			aria-label={m.listing.removeFilter(label)}
			className="inline-flex h-7 items-center gap-1.5 rounded-full border border-brand bg-brand/10 px-3 font-medium text-brand-ink text-sm transition-colors hover:bg-brand/15"
		>
			{label} <X className="size-3.5" />
		</button>
	);
}
