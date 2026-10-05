import { createFileRoute, Link } from "@tanstack/react-router";
import { ChevronLeft, ChevronRight, SlidersHorizontal, X } from "lucide-react";
import { useState } from "react";
import { Container } from "#/components/site/container";
import {
	Breadcrumb,
	BreadcrumbItem,
	BreadcrumbLink,
	BreadcrumbList,
	BreadcrumbPage,
	BreadcrumbSeparator,
} from "#/components/ui/breadcrumb";
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
import type { components } from "#/lib/api/schema.gen";
import { cn } from "#/lib/utils";
import { PreviewProductCard } from "./-cart";
import { inCategory } from "./-data";

type Category = components["schemas"]["Category"];

export const Route = createFileRoute("/preview/category")({
	validateSearch: (s: Record<string, unknown>): { category: Category } => ({
		category: (typeof s.category === "string" ? s.category : "gpu") as Category,
	}),
	component: CategoryPage,
});

const buckets = [
	{ min: 0, max: 10000 },
	{ min: 10000, max: 25000 },
	{ min: 25000, max: 50000 },
	{ min: 50000, max: 100000 },
	{ min: 100000 },
];

const sortOptions = [
	{ value: "relevance", label: m.listing.sortRelevance },
	{ value: "price_asc", label: m.listing.sortPriceAsc },
	{ value: "price_desc", label: m.listing.sortPriceDesc },
];

function CategoryPage() {
	const { category } = Route.useSearch();
	const all = inCategory(category);
	const [brands, setBrands] = useState<string[]>([]);
	const [inStockOnly, setInStockOnly] = useState(false);
	const [sort, setSort] = useState("relevance");
	const [filtersOpen, setFiltersOpen] = useState(false);

	const brandCounts = Object.entries(
		all.reduce<Record<string, number>>((acc, p) => {
			acc[p.brand] = (acc[p.brand] ?? 0) + 1;
			return acc;
		}, {}),
	).sort((a, b) => b[1] - a[1]);

	const priceCounts = buckets
		.map((b) => ({
			...b,
			count: all.filter(
				(p) =>
					p.price.amount_cents >= b.min &&
					(b.max === undefined || p.price.amount_cents < b.max),
			).length,
		}))
		.filter((b) => b.count > 0);

	let shown = all.filter(
		(p) =>
			(brands.length === 0 || brands.includes(p.brand)) &&
			(!inStockOnly || p.in_stock),
	);
	if (sort === "price_asc")
		shown = [...shown].sort(
			(a, b) => a.price.amount_cents - b.price.amount_cents,
		);
	if (sort === "price_desc")
		shown = [...shown].sort(
			(a, b) => b.price.amount_cents - a.price.amount_cents,
		);

	const toggleBrand = (b: string) =>
		setBrands((cur) =>
			cur.includes(b) ? cur.filter((x) => x !== b) : [...cur, b],
		);

	// Rendered in the sidebar on desktop and in the filter sheet on phones.
	const facets = (prefix: string) => (
		<>
			<FacetGroup title={m.listing.availability}>
				<FacetCheckbox
					id={`${prefix}-stock`}
					label={m.listing.inStockOnly}
					count={all.filter((p) => p.in_stock).length}
					checked={inStockOnly}
					onChange={() => setInStockOnly((v) => !v)}
				/>
			</FacetGroup>
			<FacetGroup title={m.listing.brand}>
				{brandCounts.map(([b, n]) => (
					<FacetCheckbox
						key={b}
						id={`${prefix}-brand-${b}`}
						label={b}
						count={n}
						checked={brands.includes(b)}
						onChange={() => toggleBrand(b)}
					/>
				))}
			</FacetGroup>
			<FacetGroup title={m.listing.price}>
				{priceCounts.map((b) => (
					<FacetCheckbox
						key={b.min}
						id={`${prefix}-price-${b.min}`}
						label={
							b.max === undefined
								? m.listing.priceFrom(formatPrice(b.min))
								: `${formatPrice(b.min)} – ${formatPrice(b.max)}`
						}
						count={b.count}
						checked={false}
						onChange={() => {}}
					/>
				))}
			</FacetGroup>
		</>
	);

	return (
		<Container className="pt-6">
			<Breadcrumb>
				<BreadcrumbList>
					<BreadcrumbItem>
						<BreadcrumbLink render={<Link to="/preview/home" />}>
							{m.product.home}
						</BreadcrumbLink>
					</BreadcrumbItem>
					<BreadcrumbSeparator />
					<BreadcrumbItem>
						<BreadcrumbPage>{m.categories[category]}</BreadcrumbPage>
					</BreadcrumbItem>
				</BreadcrumbList>
			</Breadcrumb>

			<div className="mt-4 flex flex-wrap items-end justify-between gap-4 border-b pb-6">
				<div>
					<h1 className="font-semibold text-3xl tracking-[-0.03em]">
						{m.categories[category]}
					</h1>
					<p className="mt-1 text-muted-foreground text-sm tabular-nums">
						{m.listing.results(shown.length)}
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
						value={sort}
						onValueChange={(v) => v && setSort(v)}
					>
						<SelectTrigger className="w-56 bg-card" aria-label={m.listing.sort}>
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
						{facets("side")}
					</div>
				</aside>

				<div>
					{brands.length > 0 || inStockOnly ? (
						<div className="mb-4 flex flex-wrap items-center gap-2">
							{inStockOnly ? (
								<ActiveChip
									label={m.listing.inStockOnly}
									onRemove={() => setInStockOnly(false)}
								/>
							) : null}
							{brands.map((b) => (
								<ActiveChip key={b} label={b} onRemove={() => toggleBrand(b)} />
							))}
							<button
								type="button"
								onClick={() => {
									setBrands([]);
									setInStockOnly(false);
								}}
								className="px-2 text-muted-foreground text-sm hover:text-foreground"
							>
								{m.listing.clearFilters}
							</button>
						</div>
					) : null}

					<div className="grid grid-cols-2 gap-3 md:grid-cols-3">
						{shown.map((p) => (
							<PreviewProductCard key={p.slug} product={p} />
						))}
					</div>

					<nav className="mt-10 flex items-center justify-center gap-1">
						<Button variant="ghost" size="sm" disabled>
							<ChevronLeft /> {m.listing.previous}
						</Button>
						{[1, 2, 3].map((n) => (
							<Button
								key={n}
								variant={n === 1 ? "outline" : "ghost"}
								size="icon-sm"
								className={cn("tabular-nums", n === 1 && "bg-card")}
							>
								{n}
							</Button>
						))}
						<Button variant="ghost" size="sm">
							{m.listing.next} <ChevronRight />
						</Button>
					</nav>
				</div>
			</div>

			<Drawer side="bottom" open={filtersOpen} onOpenChange={setFiltersOpen}>
				<DrawerPopup side="bottom">
					<DrawerContent className="flex min-h-0 flex-1 flex-col">
						<DrawerTitle className="px-5 pt-2 pb-3 font-medium">
							{m.listing.filters}
						</DrawerTitle>
						<div className="flex min-h-0 flex-1 flex-col gap-8 overflow-y-auto overscroll-contain border-t px-5 py-5">
							{facets("sheet")}
						</div>
						<div className="border-t px-5 pt-4">
							<DrawerClose
								render={<Button size="lg" className="h-11 w-full" />}
							>
								{m.listing.results(shown.length)}
							</DrawerClose>
						</div>
					</DrawerContent>
				</DrawerPopup>
			</Drawer>
		</Container>
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
			className="inline-flex h-7 items-center gap-1.5 rounded-full border border-brand bg-brand/10 px-3 font-medium text-brand-ink text-sm transition-colors hover:bg-brand/15"
		>
			{label} <X className="size-3.5" />
		</button>
	);
}
