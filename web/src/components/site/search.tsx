import { Autocomplete } from "@base-ui/react/autocomplete";
import { ArrowRight, CornerDownLeft, Search, TrendingUp } from "lucide-react";
import { useEffect, useRef } from "react";
import { Price } from "#/components/product/price";
import { ProductImage } from "#/components/product/product-image";
import { m } from "#/i18n";
import type { components } from "#/lib/api/schema.gen";
import { cn } from "#/lib/utils";

type ProductSummary = components["schemas"]["ProductSummary"];
type Category = components["schemas"]["Category"];

export type SearchItem =
	| { kind: "query"; label: string }
	| { kind: "product"; label: string; product: ProductSummary }
	| { kind: "category"; label: string; category: Category; count?: number }
	| { kind: "all"; label: string };

type Group = { label?: string; items: SearchItem[] };

// SiteSearch is the header search with instant results. Empty, it suggests popular searches
// and categories; while typing it shows matching products and categories. Enter without a
// highlighted item submits the query for the full results page. "/" or ⌘K focuses it.
export function SiteSearch({
	query,
	onQueryChange,
	products,
	categories,
	popular,
	onSelect,
	className,
}: {
	query: string;
	onQueryChange: (q: string) => void;
	products: ProductSummary[];
	categories: { category: Category; count?: number }[];
	popular: string[];
	onSelect: (item: SearchItem) => void;
	className?: string;
}) {
	const input = useRef<HTMLInputElement>(null);
	const q = query.trim();

	useEffect(() => {
		const onKey = (e: KeyboardEvent) => {
			const typing =
				e.target instanceof HTMLElement &&
				(e.target.isContentEditable ||
					/^(INPUT|TEXTAREA|SELECT)$/.test(e.target.tagName));
			if (
				(e.key === "k" && (e.metaKey || e.ctrlKey)) ||
				(e.key === "/" && !typing)
			) {
				e.preventDefault();
				input.current?.focus();
			}
		};
		window.addEventListener("keydown", onKey);
		return () => window.removeEventListener("keydown", onKey);
	}, []);

	const categoryItems: SearchItem[] = categories.map((c) => ({
		kind: "category",
		label: m.categories[c.category],
		category: c.category,
		count: c.count,
	}));

	const groups: Group[] = q
		? [
				{
					label: m.search.products,
					items: products.map(
						(p): SearchItem => ({
							kind: "product",
							label: p.name,
							product: p,
						}),
					),
				},
				{ label: m.search.categories, items: categoryItems },
				{ items: [{ kind: "all", label: m.search.viewAll(q) } as SearchItem] },
			].filter((g: Group) => g.items.length > 0)
		: [
				{
					label: m.search.popular,
					items: popular.map((label): SearchItem => ({ kind: "query", label })),
				},
				{ label: m.search.categories, items: categoryItems },
			];

	return (
		<Autocomplete.Root
			items={groups}
			value={query}
			onValueChange={(v, details) => {
				// Picking or arrowing to an item doesn't fill the input with its name: products and
				// categories navigate, and a popular search fills it in onSelect.
				if (
					details.reason === "item-press" ||
					details.reason === "list-navigation"
				)
					return;
				onQueryChange(v);
			}}
			itemToStringValue={(item: SearchItem) => item.label}
			filter={null}
			openOnInputClick
		>
			<search className={cn("relative", className)}>
				<form
					onSubmit={(e) => {
						e.preventDefault();
						if (q) onSelect({ kind: "all", label: q });
					}}
				>
					<Search className="-translate-y-1/2 pointer-events-none absolute top-1/2 left-3.5 size-4 text-subtle-foreground" />
					<Autocomplete.Input
						ref={input}
						type="search"
						aria-label={m.search.label}
						placeholder={m.nav.searchPlaceholder}
						className="h-10 w-full rounded-lg border border-transparent bg-muted pr-14 pl-10 text-sm outline-none transition-[background-color,border-color,box-shadow] duration-150 placeholder:text-subtle-foreground hover:bg-accent focus:border-border-strong focus:bg-card focus:shadow-[0_0_0_4px] focus:shadow-brand/12 pointer-coarse:text-base [&::-webkit-search-cancel-button]:hidden"
					/>
					<kbd className="-translate-y-1/2 pointer-events-none absolute top-1/2 right-3 hidden items-center rounded border bg-card px-1.5 py-0.5 font-mono text-[11px] text-subtle-foreground leading-none md:flex">
						⌘K
					</kbd>
				</form>
			</search>

			<Autocomplete.Portal>
				<Autocomplete.Positioner
					sideOffset={8}
					align="start"
					className="z-50 outline-none"
				>
					<Autocomplete.Popup className="w-[max(var(--anchor-width),34rem)] max-w-[var(--available-width)] origin-[var(--transform-origin)] overflow-hidden rounded-xl border bg-popover text-popover-foreground shadow-popover transition-[opacity,transform] duration-150 ease-out data-ending-style:scale-[0.98] data-starting-style:scale-[0.98] data-ending-style:opacity-0 data-starting-style:opacity-0">
						<Autocomplete.List className="max-h-[min(30rem,var(--available-height))] overflow-y-auto overscroll-contain p-1.5 outline-none">
							{(group: Group) => (
								<Autocomplete.Group
									key={group.label ?? "all"}
									items={group.items}
									className="pb-1.5 last:pb-0"
								>
									{group.label ? (
										<Autocomplete.GroupLabel className="px-2.5 pt-2 pb-1.5 font-medium text-[11px] text-subtle-foreground uppercase tracking-wider">
											{group.label}
										</Autocomplete.GroupLabel>
									) : (
										<div className="-mx-1.5 mb-1.5 border-t" />
									)}
									<Autocomplete.Collection>
										{(item: SearchItem) => (
											<Autocomplete.Item
												key={key(item)}
												value={item}
												onClick={() => onSelect(item)}
												className="flex cursor-pointer select-none items-center gap-3 rounded-lg px-2.5 py-2 text-sm outline-none data-highlighted:bg-muted"
											>
												<ItemBody item={item} query={q} />
											</Autocomplete.Item>
										)}
									</Autocomplete.Collection>
								</Autocomplete.Group>
							)}
						</Autocomplete.List>
						{q && products.length === 0 && categories.length === 0 ? (
							<p className="px-4 pb-4 text-muted-foreground text-sm">
								{m.search.noResults(q)}
							</p>
						) : null}
						<div className="hidden items-center gap-4 border-t bg-background px-4 py-2 text-[11px] text-subtle-foreground md:flex">
							<Hint keys={["↑", "↓"]} label={m.search.navigate} />
							<Hint keys={["↵"]} label={m.search.select} />
							<Hint keys={["esc"]} label={m.search.dismiss} />
						</div>
					</Autocomplete.Popup>
				</Autocomplete.Positioner>
			</Autocomplete.Portal>
		</Autocomplete.Root>
	);
}

function key(item: SearchItem) {
	switch (item.kind) {
		case "product":
			return `p:${item.product.slug}`;
		case "category":
			return `c:${item.category}`;
		default:
			return `${item.kind}:${item.label}`;
	}
}

function ItemBody({ item, query }: { item: SearchItem; query: string }) {
	switch (item.kind) {
		case "product": {
			const p = item.product;
			return (
				<>
					<ProductImage
						src={p.image}
						alt=""
						className="size-10 shrink-0 rounded-md border p-1"
					/>
					<div className="min-w-0 flex-1">
						<p className="truncate">
							<Highlight text={p.name} query={query} />
						</p>
						<p className="truncate text-subtle-foreground text-xs">
							{m.categories[p.category]} · {p.brand}
						</p>
					</div>
					<Price cents={p.price.amount_cents} className="text-sm" />
				</>
			);
		}
		case "category":
			return (
				<>
					<span className="flex size-6 items-center justify-center rounded-md border bg-card">
						<ArrowRight className="size-3.5 text-subtle-foreground" />
					</span>
					<span className="flex-1">{item.label}</span>
					{item.count ? (
						<span className="text-subtle-foreground text-xs tabular-nums">
							{item.count}
						</span>
					) : null}
				</>
			);
		case "query":
			return (
				<>
					<TrendingUp className="size-4 text-subtle-foreground" />
					<span className="flex-1">{item.label}</span>
				</>
			);
		case "all":
			return (
				<>
					<Search className="size-4 text-subtle-foreground" />
					<span className="flex-1 font-medium">{item.label}</span>
					<CornerDownLeft className="size-3.5 text-subtle-foreground" />
				</>
			);
	}
}

// Highlight bolds the parts of text that match the query's words.
function Highlight({ text, query }: { text: string; query: string }) {
	const words = query.split(/\s+/).filter(Boolean).map(escapeRegExp);
	if (words.length === 0) return text;
	const parts = text.split(new RegExp(`(${words.join("|")})`, "gi"));
	return parts.map((part, i) =>
		i % 2 === 1 ? (
			// biome-ignore lint/suspicious/noArrayIndexKey: parts of one static string
			<mark key={i} className="bg-transparent font-semibold text-foreground">
				{part}
			</mark>
		) : (
			part
		),
	);
}

function escapeRegExp(s: string) {
	return s.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

function Hint({ keys, label }: { keys: string[]; label: string }) {
	return (
		<span className="flex items-center gap-1.5">
			{keys.map((k) => (
				<kbd
					key={k}
					className="rounded border bg-card px-1 font-mono leading-4"
				>
					{k}
				</kbd>
			))}
			{label}
		</span>
	);
}
