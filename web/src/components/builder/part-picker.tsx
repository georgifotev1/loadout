import { useQuery } from "@tanstack/react-query";
import { Check, X } from "lucide-react";
import { useMemo, useState } from "react";
import { Price } from "#/components/product/price";
import { ProductImage } from "#/components/product/product-image";
import { Button } from "#/components/ui/button";
import {
	Drawer,
	DrawerClose,
	DrawerContent,
	DrawerDescription,
	DrawerPopup,
	DrawerTitle,
} from "#/components/ui/drawer";
import { useMediaQuery } from "#/hooks/use-media-query";
import { formatPrice, m } from "#/i18n";
import { builderOptionsQuery } from "#/lib/api/builder";
import type { Schemas } from "#/lib/api/client";
import {
	type BuildSearch,
	keySpecs,
	recommendedPSU,
	type Slot,
	without,
} from "#/lib/builder";
import {
	applyFilters,
	commonToggles,
	emptyFilters,
	type FilterState,
	narrowing,
	pickerFilters,
	sortOptions,
} from "#/lib/builder-filters";
import { cn } from "#/lib/utils";
import { FindingNote } from "./finding-note";
import { FilterPanel, PickerToolbar } from "./picker-filters";
import { findingKey, SpecChip } from "./slot-row";

type Option = Schemas["BuilderOption"];

export type PickerTarget = {
	slot: Slot;
	// The storage drive being swapped; absent when adding one.
	replacing?: string;
	// The slugs to mark as picked, and the price the deltas are relative to.
	current: string[];
	currentCents?: number;
};

// PartPicker lists the parts that fit a slot: a wide sidebar on desktop and a bottom sheet on
// phones, so the build stays in view behind it.
export function PartPicker({
	open,
	onOpenChange,
	target,
	build,
	recommendedPsuW,
	onPick,
}: {
	open: boolean;
	onOpenChange: (open: boolean) => void;
	target?: PickerTarget;
	build: BuildSearch;
	recommendedPsuW: number;
	onPick: (slug: string) => void;
}) {
	const desktop = useMediaQuery("(min-width: 768px)");
	const side = desktop ? "right" : "bottom";
	return (
		<Drawer side={side} open={open} onOpenChange={onOpenChange}>
			<DrawerPopup
				side={side}
				className={cn(
					side === "right" && "w-152",
					side === "bottom" && "h-[calc(88dvh+3rem)]",
				)}
			>
				<DrawerContent className="flex min-h-0 flex-1 flex-col">
					{target ? (
						// Keyed so the filters reset when another slot opens.
						<PickerBody
							key={`${target.slot}:${target.replacing ?? ""}`}
							target={target}
							build={build}
							recommendedPsuW={recommendedPsuW}
							onPick={onPick}
						/>
					) : null}
				</DrawerContent>
			</DrawerPopup>
		</Drawer>
	);
}

function PickerBody({
	target,
	build,
	recommendedPsuW,
	onPick,
}: {
	target: PickerTarget;
	build: BuildSearch;
	recommendedPsuW: number;
	onPick: (slug: string) => void;
}) {
	const { slot } = target;
	const q = useQuery(builderOptionsQuery(slot, without(build, slot)));
	const desktop = useMediaQuery("(min-width: 768px)");
	const [filters, setFilters] = useState<FilterState>(emptyFilters);
	// Open on desktop, where there's room; a tap away on phones, to keep the list in view.
	const [filtersOpen, setFiltersOpen] = useState(desktop);

	const options = q.data?.options ?? [];
	const defs = useMemo(() => {
		const all = pickerFilters(slot, recommendedPsuW);
		return { ...all, toggles: [...all.toggles, ...commonToggles()] };
	}, [slot, recommendedPsuW]);
	const shownDefs = useMemo(() => narrowing(options, defs), [options, defs]);
	const recommended =
		slot === "psu" ? recommendedPSU(options, recommendedPsuW) : undefined;
	const shown = useMemo(() => {
		const list = sortOptions(
			applyFilters(options, shownDefs, filters),
			defs.sorts.find((s) => s.id === filters.sort),
		);
		// The current pick goes first, so "keep what I have" is always one glance away.
		return [
			...list.filter((o) => target.current.includes(o.product.slug)),
			...list.filter((o) => !target.current.includes(o.product.slug)),
		];
	}, [options, shownDefs, defs.sorts, filters, target.current]);
	const hasFilters = shownDefs.facets.length + shownDefs.toggles.length > 0;

	return (
		<>
			<div className="flex items-start justify-between gap-4 border-b px-5 pt-4 pb-3.5">
				<div>
					<DrawerTitle className="font-semibold text-lg tracking-[-0.01em]">
						{m.builder.pickerTitle[slot]}
					</DrawerTitle>
					<DrawerDescription className="mt-0.5 text-meta text-subtle-foreground">
						{!q.data
							? "\u00a0"
							: shown.length < options.length
								? m.builder.picker.countFiltered(shown.length, options.length)
								: m.builder.picker.count(options.length)}
					</DrawerDescription>
				</div>
				<DrawerClose
					render={
						<Button
							variant="ghost"
							size="icon-sm"
							aria-label={m.nav.close}
							className="-mr-1.5 text-muted-foreground"
						/>
					}
				>
					<X />
				</DrawerClose>
			</div>

			{options.length > 1 ? (
				<div className="border-b">
					<PickerToolbar
						state={filters}
						onChange={setFilters}
						sorts={defs.sorts}
						hasFilters={hasFilters}
						open={filtersOpen}
						onOpenChange={setFiltersOpen}
					/>
					{hasFilters && filtersOpen ? (
						<FilterPanel
							options={options}
							defs={shownDefs}
							state={filters}
							onChange={setFilters}
						/>
					) : null}
				</div>
			) : null}

			<div className="min-h-0 flex-1 overflow-y-auto overscroll-contain">
				{q.isPending ? (
					<OptionSkeletons />
				) : q.isError ? (
					<div className="flex flex-col items-start gap-3 p-5">
						<p className="text-muted-foreground text-sm">
							{m.builder.picker.error}
						</p>
						<Button variant="outline" size="sm" onClick={() => q.refetch()}>
							{m.builder.picker.retry}
						</Button>
					</div>
				) : options.length === 0 ? (
					<p className="p-5 text-muted-foreground text-sm">
						{m.builder.picker.empty}
					</p>
				) : shown.length === 0 ? (
					<div className="flex flex-col items-start gap-3 p-5">
						<p className="text-muted-foreground text-sm">
							{m.builder.picker.emptyFiltered}
						</p>
						<Button
							variant="outline"
							size="sm"
							onClick={() =>
								setFilters({ ...filters, facets: {}, toggles: [] })
							}
						>
							{m.builder.filters.clear}
						</Button>
					</div>
				) : (
					<ul className="divide-y">
						{shown.map((o) => (
							<OptionRow
								key={o.product.slug}
								slot={slot}
								option={o}
								current={target.current.includes(o.product.slug)}
								recommended={o.product.slug === recommended}
								currentCents={target.currentCents}
								onPick={onPick}
							/>
						))}
					</ul>
				)}
			</div>
		</>
	);
}

function OptionRow({
	slot,
	option: o,
	current,
	recommended,
	currentCents,
	onPick,
}: {
	slot: Slot;
	option: Option;
	current: boolean;
	recommended: boolean;
	currentCents?: number;
	onPick: (slug: string) => void;
}) {
	const p = o.product;
	const delta =
		currentCents !== undefined && !current
			? p.price.amount_cents - currentCents
			: undefined;
	return (
		<li>
			<button
				type="button"
				onClick={() => onPick(p.slug)}
				aria-current={current || undefined}
				className={cn(
					"grid w-full grid-cols-[3.5rem_1fr_auto] items-start gap-x-4 px-5 py-4 text-left transition-colors hover:bg-muted/60 focus-visible:bg-muted/60 focus-visible:outline-none",
					current && "bg-muted/40",
				)}
			>
				<ProductImage
					src={p.image}
					alt=""
					className="size-14 rounded-lg border p-1"
				/>
				<div className="min-w-0">
					<div className="flex flex-wrap items-center gap-1.5">
						<span className="font-medium text-[11px] text-subtle-foreground uppercase tracking-wider">
							{p.brand}
						</span>
						{recommended ? (
							<span className="rounded-full bg-brand/10 px-1.5 py-px font-medium text-[11px] text-brand-ink">
								{m.builder.picker.recommended}
							</span>
						) : null}
						{current ? (
							<span className="rounded-full bg-foreground px-1.5 py-px font-medium text-[11px] text-background">
								{m.builder.picker.current}
							</span>
						) : null}
					</div>
					<p className="mt-0.5 line-clamp-2 text-meta">{p.name}</p>
					<div className="mt-1.5 flex flex-wrap items-center gap-1.5">
						{keySpecs(slot, o.specs).map((s) => (
							<SpecChip key={s}>{s}</SpecChip>
						))}
					</div>
					{o.findings.length ? (
						<div className="mt-2 flex flex-col gap-1">
							{o.findings.map((f) => (
								<FindingNote
									key={findingKey(f)}
									finding={f}
									className="text-xs"
								/>
							))}
						</div>
					) : null}
				</div>
				<div className="flex flex-col items-end gap-1">
					<Price cents={p.price.amount_cents} className="text-sm" />
					{delta !== undefined && delta !== 0 ? (
						<span className="text-subtle-foreground text-xs tabular-nums">
							{delta > 0 ? "+" : "−"}
							{formatPrice(Math.abs(delta))}
						</span>
					) : null}
					<span
						className={cn(
							"mt-2 inline-flex h-7 items-center gap-1 rounded-md px-2.5 font-medium text-xs",
							current
								? "text-muted-foreground"
								: "border bg-card text-foreground",
						)}
					>
						{current ? (
							<>
								<Check className="size-3.5" /> {m.builder.chosen}
							</>
						) : (
							m.builder.choose
						)}
					</span>
				</div>
			</button>
		</li>
	);
}

function OptionSkeletons() {
	return (
		<ul aria-hidden className="divide-y">
			{[0, 1, 2, 3, 4].map((i) => (
				<li key={i} className="flex gap-4 px-5 py-4">
					<div className="size-14 shrink-0 animate-pulse rounded-lg bg-muted" />
					<div className="flex flex-1 flex-col gap-2 pt-1">
						<div className="h-3 w-16 animate-pulse rounded bg-muted" />
						<div className="h-3.5 w-4/5 animate-pulse rounded bg-muted" />
						<div className="h-3.5 w-2/5 animate-pulse rounded bg-muted" />
					</div>
					<div className="h-4 w-16 animate-pulse rounded bg-muted" />
				</li>
			))}
		</ul>
	);
}
