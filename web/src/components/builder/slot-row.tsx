import { Link } from "@tanstack/react-router";
import {
	Box,
	Check,
	CircuitBoard,
	Cpu,
	Fan,
	Gpu,
	HardDrive,
	MemoryStick,
	PlugZap,
	Plus,
	X,
} from "lucide-react";
import { Quantity } from "#/components/cart/quantity";
import { Price } from "#/components/product/price";
import { ProductImage } from "#/components/product/product-image";
import { Stock } from "#/components/product/stock";
import { Button, buttonVariants } from "#/components/ui/button";
import { m } from "#/i18n";
import {
	type BuildCheck,
	type BuildPart,
	type BuildStatus,
	type Finding,
	groupSlugs,
	hasPowerParts,
	isMultiSlot,
	keySpecs,
	maxParts,
	ownFindings,
	relatedSlots,
	type Severity,
	type Slot,
	slotOrder,
	worstSeverity,
} from "#/lib/builder";
import { cn } from "#/lib/utils";
import { FindingNote, SeverityIcon } from "./finding-note";

export const slotIcons: Record<Slot, typeof Cpu> = {
	cpu: Cpu,
	motherboard: CircuitBoard,
	ram: MemoryStick,
	cooler: Fan,
	gpu: Gpu,
	storage: HardDrive,
	psu: PlugZap,
	case: Box,
};

export type SlotActions = {
	// Opens the picker for slot; replacing is the storage drive being swapped.
	onChoose: (slot: Slot, replacing?: string) => void;
	onRemove: (slot: Slot, slug: string) => void;
	onCount: (slot: "ram" | "storage", slug: string, n: number) => void;
	// Starts loading slot's options before the picker opens.
	onPrefetch: (slot: Slot) => void;
};

// slotElementId is the row's id, so the summary can scroll to the part a problem is about.
export const slotElementId = (slot: Slot) => `slot-${slot}`;

// scrollToSlot brings a slot's row into view, e.g. from a problem in the summary.
export function scrollToSlot(slot: Slot) {
	document
		.getElementById(slotElementId(slot))
		?.scrollIntoView({ behavior: "smooth", block: "start" });
}

// BuildSlots lists every slot of the build. Without actions it's read-only (a saved build).
export function BuildSlots({
	check,
	status,
	actions,
}: {
	check: BuildCheck;
	status: BuildStatus;
	actions?: SlotActions;
}) {
	return (
		<ol className="divide-y border-y">
			{slotOrder.map((slot) => {
				const parts = check.parts.filter((p) => p.slot === slot);
				const unavailable = check.unavailable.filter((u) => u.slot === slot);
				// A read-only build skips the optional slots nobody picked.
				if (
					!actions &&
					!parts.length &&
					!unavailable.length &&
					!status.missing.includes(slot)
				) {
					return null;
				}
				return (
					<SlotRow
						key={slot}
						slot={slot}
						check={check}
						parts={parts}
						unavailable={unavailable.map((u) => u.slug)}
						status={status}
						actions={actions}
					/>
				);
			})}
		</ol>
	);
}

function SlotRow({
	slot,
	check,
	parts,
	unavailable,
	status,
	actions,
}: {
	slot: Slot;
	check: BuildCheck;
	parts: BuildPart[];
	unavailable: string[];
	status: BuildStatus;
	actions?: SlotActions;
}) {
	const required = status.required.includes(slot);
	const severity =
		unavailable.length > 0 ? "error" : worstSeverity(status.problems, slot);
	const filled = parts.length > 0 || unavailable.length > 0;
	const groups = groupSlugs(parts.map((p) => p.product.slug)).map((g) => ({
		...g,
		part: parts.find((p) => p.product.slug === g.slug) as BuildPart,
	}));
	const findings = ownFindings(status.problems, slot);
	const related = relatedSlots(status.problems, slot);
	const multi = isMultiSlot(slot);
	const total = multi ? parts.length + unavailable.length : 0;

	return (
		<li
			id={slotElementId(slot)}
			className="grid scroll-mt-[calc(var(--spacing-header)+2.5rem+1rem)] gap-x-6 gap-y-3 py-5 md:grid-cols-[10.5rem_1fr]"
		>
			<SlotLabel
				slot={slot}
				required={required}
				filled={parts.length > 0}
				next={status.next === slot && !!actions}
				severity={severity}
			/>

			<div className="flex min-w-0 flex-col gap-2">
				{groups.map(({ slug, count, part }) => (
					<PartLine
						key={slug}
						slot={slot}
						part={part}
						count={count}
						severity={severity}
						maxCount={multi ? count + maxParts[slot] - total : undefined}
						actions={actions}
					/>
				))}
				{unavailable.map((slug) => (
					<UnavailableLine
						key={slug}
						slot={slot}
						slug={slug}
						actions={actions}
					/>
				))}
				{!filled ? (
					<EmptySlot
						slot={slot}
						hint={hint(slot, check, status)}
						next={status.next === slot}
						actions={actions}
					/>
				) : null}
				{actions && slot === "storage" && filled && total < maxParts.storage ? (
					<button
						type="button"
						onClick={() => actions.onChoose("storage")}
						onPointerEnter={() => actions.onPrefetch("storage")}
						className="flex h-10 items-center gap-2 self-start rounded-md px-2 font-medium text-meta text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
					>
						<Plus className="size-4" /> {m.builder.addDrive}
					</button>
				) : null}
				{findings.map((f) => (
					<FindingNote key={findingKey(f)} finding={f} className="px-1" />
				))}
				{related.length ? (
					<p className="flex flex-wrap items-center gap-x-3 gap-y-1 px-1 text-meta text-muted-foreground">
						{related.map((r) => (
							<button
								key={r}
								type="button"
								onClick={() => scrollToSlot(r)}
								className="underline decoration-border-strong underline-offset-4 hover:text-foreground hover:decoration-foreground"
							>
								{m.builder.seeSlot(m.builder.slot[r])}
							</button>
						))}
					</p>
				) : null}
			</div>
		</li>
	);
}

export function findingKey(f: Finding): string {
	return `${f.rule}:${f.slots.join(",")}:${JSON.stringify(f.params)}`;
}

function SlotLabel({
	slot,
	required,
	filled,
	next,
	severity,
}: {
	slot: Slot;
	required: boolean;
	filled: boolean;
	next: boolean;
	severity?: Severity;
}) {
	const Icon = slotIcons[slot];
	const problem = severity === "error" || severity === "warning";
	return (
		<div className="flex items-center gap-3 md:items-start md:pt-2">
			<span
				className={cn(
					"relative flex size-10 shrink-0 items-center justify-center rounded-lg border bg-card text-muted-foreground",
					next && "border-brand text-brand-ink ring-3 ring-brand/15",
					filled && !problem && "border-foreground/80 text-foreground",
					severity === "error" && "border-destructive/60 text-destructive",
					severity === "warning" && "border-warning text-foreground",
				)}
			>
				<Icon className="size-4.5" strokeWidth={1.75} />
				{filled ? (
					<span className="-right-1.5 -top-1.5 absolute flex size-4.5 items-center justify-center rounded-full bg-card">
						{problem ? (
							<SeverityIcon severity={severity} className="size-4.5" />
						) : (
							<span className="flex size-4 items-center justify-center rounded-full bg-success text-white">
								<Check className="size-2.5" strokeWidth={3.5} />
							</span>
						)}
					</span>
				) : null}
			</span>
			<div className="min-w-0">
				<p className="font-medium text-sm">{m.builder.slot[slot]}</p>
				<p
					className={cn(
						"text-xs",
						next ? "text-brand-ink" : "text-subtle-foreground",
					)}
				>
					{required ? m.builder.required : m.builder.optional}
				</p>
			</div>
		</div>
	);
}

function hint(slot: Slot, check: BuildCheck, status: BuildStatus): string {
	const h = m.builder.hint;
	const cpu = check.parts.find((p) => p.slot === "cpu")?.specs;
	switch (slot) {
		case "cooler":
			if (status.required.includes("cooler")) return h.coolerRequired;
			return cpu?.cooler_included === true ? h.coolerBoxed : h.cooler;
		case "gpu":
			if (status.required.includes("gpu")) return h.gpuRequired;
			return cpu?.igpu === true ? h.gpuOptional : h.gpu;
		case "psu":
			return hasPowerParts(check)
				? h.psuRecommended(check.recommended_psu_w)
				: h.psu;
		default:
			return h[slot];
	}
}

function EmptySlot({
	slot,
	hint,
	next,
	actions,
}: {
	slot: Slot;
	hint: string;
	next: boolean;
	actions?: SlotActions;
}) {
	if (!actions) {
		return (
			<p className="rounded-xl border border-dashed px-4 py-3.5 text-meta text-subtle-foreground">
				{m.builder.findings.partRequired(m.builder.slot[slot])}
			</p>
		);
	}
	return (
		<button
			type="button"
			onClick={() => actions.onChoose(slot)}
			onPointerEnter={() => actions.onPrefetch(slot)}
			onFocus={() => actions.onPrefetch(slot)}
			className={cn(
				"group flex items-center justify-between gap-4 rounded-xl border border-dashed px-4 py-3 text-left transition-colors hover:border-border-strong hover:bg-card",
				next && "border-brand/50 bg-brand/3 hover:border-brand",
			)}
		>
			<span className="text-meta text-muted-foreground">{hint}</span>
			<span
				className={buttonVariants({
					variant: next ? "default" : "outline",
					size: "sm",
					className: "shrink-0",
				})}
			>
				<Plus /> {m.builder.choose}
			</span>
		</button>
	);
}

function PartLine({
	slot,
	part,
	count,
	severity,
	maxCount,
	actions,
}: {
	slot: Slot;
	part: BuildPart;
	count: number;
	severity?: Severity;
	maxCount?: number;
	actions?: SlotActions;
}) {
	const p = part.product;
	const specs = keySpecs(slot, part.specs);
	return (
		<div
			className={cn(
				"grid grid-cols-[3.5rem_1fr] items-center gap-x-4 gap-y-3 rounded-xl border bg-card p-3 sm:grid-cols-[4rem_1fr_auto]",
				severity === "error" && "border-destructive/40",
				severity === "warning" && "border-warning/60",
			)}
		>
			<ProductImage
				src={p.image}
				alt=""
				className="size-14 rounded-lg border p-1 sm:size-16"
			/>
			<div className="min-w-0">
				<Link
					to="/p/$slug"
					params={{ slug: p.slug }}
					className="line-clamp-2 text-meta hover:underline"
				>
					{p.name}
				</Link>
				<div className="mt-1.5 flex flex-wrap items-center gap-1.5">
					{specs.map((s) => (
						<SpecChip key={s}>{s}</SpecChip>
					))}
					{!p.in_stock ? <Stock inStock={false} className="ml-1" /> : null}
				</div>
			</div>
			<div className="col-span-2 flex items-center justify-between gap-3 sm:col-span-1 sm:flex-col sm:items-end">
				<Price cents={p.price.amount_cents * count} className="text-sm" />
				{actions ? (
					<div className="flex items-center gap-1.5">
						{isMultiSlot(slot) ? (
							<Quantity
								value={count}
								max={maxCount}
								onChange={(n) => actions.onCount(slot, p.slug, n)}
							/>
						) : null}
						<Button
							variant="outline"
							size="sm"
							onClick={() =>
								actions.onChoose(slot, slot === "storage" ? p.slug : undefined)
							}
							onPointerEnter={() => actions.onPrefetch(slot)}
						>
							{m.builder.change}
						</Button>
						<Button
							variant="ghost"
							size="icon-sm"
							aria-label={`${m.builder.remove}: ${p.name}`}
							onClick={() => actions.onRemove(slot, p.slug)}
							className="text-subtle-foreground"
						>
							<X />
						</Button>
					</div>
				) : count > 1 ? (
					<span className="text-subtle-foreground text-xs tabular-nums">
						{count} ×{" "}
						<Price cents={p.price.amount_cents} className="font-normal" />
					</span>
				) : null}
			</div>
		</div>
	);
}

export function SpecChip({ children }: { children: React.ReactNode }) {
	return (
		<span className="inline-flex h-5 items-center rounded bg-muted px-1.5 text-[11px] text-muted-foreground tabular-nums">
			{children}
		</span>
	);
}

function UnavailableLine({
	slot,
	slug,
	actions,
}: {
	slot: Slot;
	slug: string;
	actions?: SlotActions;
}) {
	return (
		<div className="flex flex-wrap items-center justify-between gap-3 rounded-xl border border-destructive/40 bg-destructive/3 p-3 pl-4">
			<div className="min-w-0">
				<p className="font-medium text-destructive text-meta">
					{m.builder.unavailable}
				</p>
				<p className="truncate text-subtle-foreground text-xs">
					{m.builder.unavailableDetail}{" "}
					<span className="font-mono">{slug}</span>
				</p>
			</div>
			{actions ? (
				<div className="flex items-center gap-1.5">
					<Button
						size="sm"
						onClick={() =>
							actions.onChoose(slot, slot === "storage" ? slug : undefined)
						}
					>
						{m.builder.chooseReplacement}
					</Button>
					<Button
						variant="ghost"
						size="icon-sm"
						aria-label={m.builder.remove}
						onClick={() => actions.onRemove(slot, slug)}
						className="text-subtle-foreground"
					>
						<X />
					</Button>
				</div>
			) : null}
		</div>
	);
}
