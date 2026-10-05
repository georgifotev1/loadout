import { Check, ChevronDown, SlidersHorizontal } from "lucide-react";
import {
	Select,
	SelectContent,
	SelectItem,
	SelectTrigger,
	SelectValue,
} from "#/components/ui/select";
import { m } from "#/i18n";
import type { Schemas } from "#/lib/api/client";
import {
	activeCount,
	type Facet,
	type FilterState,
	facetValues,
	type Sort,
	type Toggle,
} from "#/lib/builder-filters";
import { cn } from "#/lib/utils";

type Option = Schemas["BuilderOption"];

// PickerToolbar is the "filters" switch and the sort, above the option list.
export function PickerToolbar({
	state,
	onChange,
	sorts,
	hasFilters,
	open,
	onOpenChange,
}: {
	state: FilterState;
	onChange: (s: FilterState) => void;
	sorts: Sort[];
	hasFilters: boolean;
	open: boolean;
	onOpenChange: (open: boolean) => void;
}) {
	const active = activeCount(state);
	const items = sorts.map((s) => ({ value: s.id, label: s.label }));
	return (
		<div className="flex items-center justify-between gap-3 px-5 py-2.5">
			{hasFilters ? (
				<button
					type="button"
					aria-expanded={open}
					onClick={() => onOpenChange(!open)}
					className="-ml-2 flex h-8 items-center gap-2 rounded-md px-2 font-medium text-sm transition-colors hover:bg-muted"
				>
					<SlidersHorizontal className="size-4" />
					{m.builder.filters.title}
					{active ? (
						<span className="flex h-4.5 min-w-4.5 items-center justify-center rounded-full bg-foreground px-1 font-semibold text-[10px] text-background tabular-nums">
							{active}
						</span>
					) : null}
					<ChevronDown
						className={cn(
							"size-4 text-subtle-foreground transition-transform",
							open && "rotate-180",
						)}
					/>
				</button>
			) : (
				<span />
			)}
			<Select
				items={items}
				value={state.sort}
				onValueChange={(v) => onChange({ ...state, sort: String(v) })}
			>
				<SelectTrigger
					className="h-8 max-w-[60%] bg-card text-xs"
					aria-label={m.builder.filters.sort}
				>
					<SelectValue />
				</SelectTrigger>
				<SelectContent align="end">
					{items.map((i) => (
						<SelectItem key={i.value} value={i.value}>
							{i.label}
						</SelectItem>
					))}
				</SelectContent>
			</Select>
		</div>
	);
}

// FilterPanel lists the facets as rows of chips with counts, then the yes/no toggles.
export function FilterPanel({
	options,
	defs,
	state,
	onChange,
}: {
	options: Option[];
	defs: { facets: Facet[]; toggles: Toggle[] };
	state: FilterState;
	onChange: (s: FilterState) => void;
}) {
	const toggleValue = (f: Facet, label: string) => {
		const cur = state.facets[f.id] ?? [];
		const next = cur.includes(label)
			? cur.filter((v) => v !== label)
			: [...cur, label];
		onChange({ ...state, facets: { ...state.facets, [f.id]: next } });
	};
	const toggle = (id: string) =>
		onChange({
			...state,
			toggles: state.toggles.includes(id)
				? state.toggles.filter((t) => t !== id)
				: [...state.toggles, id],
		});

	return (
		<div className="flex flex-col gap-2.5 px-5 pb-3.5">
			{defs.facets.map((f) => (
				<div
					key={f.id}
					className="grid grid-cols-[5.5rem_1fr] items-start gap-3"
				>
					<span className="pt-1 text-subtle-foreground text-xs">{f.label}</span>
					<div className="-my-0.5 flex gap-1.5 overflow-x-auto py-0.5 scrollbar-none sm:flex-wrap sm:overflow-visible">
						{facetValues(options, defs, state, f).map((v) => {
							const on = state.facets[f.id]?.includes(v.label) ?? false;
							return (
								<Chip
									key={v.label}
									active={on}
									disabled={!on && v.count === 0}
									onClick={() => toggleValue(f, v.label)}
								>
									{v.label}
									<span
										className={cn(
											"tabular-nums",
											on ? "text-background/60" : "text-subtle-foreground",
										)}
									>
										{v.count}
									</span>
								</Chip>
							);
						})}
					</div>
				</div>
			))}
			{defs.toggles.length ? (
				<div className="flex flex-wrap gap-1.5 pt-0.5">
					{defs.toggles.map((t) => {
						const on = state.toggles.includes(t.id);
						return (
							<Chip key={t.id} active={on} onClick={() => toggle(t.id)}>
								<span
									className={cn(
										"flex size-3.5 items-center justify-center rounded-[4px] border",
										on
											? "border-background bg-background text-foreground"
											: "border-border-strong",
									)}
								>
									{on ? <Check className="size-2.5" strokeWidth={3.5} /> : null}
								</span>
								{t.label}
							</Chip>
						);
					})}
				</div>
			) : null}
			{activeCount(state) ? (
				<button
					type="button"
					onClick={() => onChange({ ...state, facets: {}, toggles: [] })}
					className="self-start text-muted-foreground text-xs underline decoration-border-strong underline-offset-4 hover:text-foreground"
				>
					{m.builder.filters.clear}
				</button>
			) : null}
		</div>
	);
}

function Chip({
	active,
	disabled,
	onClick,
	children,
}: {
	active: boolean;
	disabled?: boolean;
	onClick: () => void;
	children: React.ReactNode;
}) {
	return (
		<button
			type="button"
			aria-pressed={active}
			disabled={disabled}
			onClick={onClick}
			className={cn(
				"flex h-7 shrink-0 items-center gap-1.5 rounded-full border px-2.5 text-xs transition-colors hover:border-border-strong disabled:pointer-events-none disabled:opacity-40",
				active
					? "border-foreground bg-foreground text-background hover:border-foreground"
					: "bg-card text-foreground",
			)}
		>
			{children}
		</button>
	);
}
