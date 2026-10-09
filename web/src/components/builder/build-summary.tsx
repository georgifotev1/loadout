import { CircleCheck } from "lucide-react";
import { AssemblyOption } from "#/components/order/assembly-option";
import { Price } from "#/components/product/price";
import { m } from "#/i18n";
import {
	type BuildCheck,
	type BuildStatus,
	buildHighlights,
	type Finding,
	findingText,
	hasPowerParts,
	type Slot,
} from "#/lib/builder";
import { cn } from "#/lib/utils";
import { SeverityIcon } from "./finding-note";
import { findingKey, scrollToSlot } from "./slot-row";

// BuildSummary is the build's total, compatibility, specs and power at a glance, the offer to
// assemble it, and the actions (`children`) at the bottom. assembly is the shopper's choice; the
// total includes it while the API offers assembly.
export function BuildSummary({
	check,
	status,
	assembly,
	onAssemblyChange,
	children,
}: {
	check: BuildCheck;
	status: BuildStatus;
	assembly: boolean;
	onAssemblyChange: (on: boolean) => void;
	children?: React.ReactNode;
}) {
	const assembled = assembly && check.assembly.offered;
	return (
		<div className="overflow-hidden rounded-xl border bg-card">
			<div className="p-5">
				<p className="text-muted-foreground text-sm">
					{m.builder.summary.total}
				</p>
				<Price
					cents={
						assembled
							? check.assembly.total.amount_cents
							: check.total.amount_cents
					}
					className="mt-1 block text-3xl tracking-[-0.03em]"
				/>
				<p className="mt-1 text-subtle-foreground text-xs">
					{m.builder.summary.vatIncluded} ·{" "}
					{m.builder.summary.parts(check.parts.length)}
					{assembled ? ` · ${m.builder.summary.withAssembly}` : null}
				</p>
			</div>

			<div className="border-t p-5">
				<Progress status={status} />
				<Compatibility check={check} status={status} />
			</div>

			<Highlights check={check} />

			{hasPowerParts(check) ? (
				<div className="border-t p-5">
					<PowerMeter check={check} />
				</div>
			) : null}

			{check.assembly.offered ? (
				<div className="border-t p-5">
					<AssemblyOption
						id="build-assembly"
						offered
						requested={assembly}
						priceCents={check.assembly.price.amount_cents}
						onChange={onAssemblyChange}
					/>
				</div>
			) : null}

			{children ? <div className="border-t p-5">{children}</div> : null}
		</div>
	);
}

// Highlights is the build's spec sheet: what it has, in a shopper's terms.
function Highlights({ check }: { check: BuildCheck }) {
	const lines = buildHighlights(check);
	if (!lines.length) return null;
	return (
		<div className="border-t p-5">
			<p className="text-muted-foreground text-sm">
				{m.builder.summary.config}
			</p>
			<dl className="mt-3 grid grid-cols-[auto_1fr] gap-x-4 gap-y-2 text-sm">
				{lines.map((l) => (
					<div key={l.slot} className="contents">
						<dt className="text-subtle-foreground">{m.builder.slot[l.slot]}</dt>
						<dd
							className="min-w-0 truncate text-right font-medium"
							title={l.value}
						>
							{l.value}
						</dd>
					</div>
				))}
			</dl>
		</div>
	);
}

function Progress({ status }: { status: BuildStatus }) {
	const total = status.required.length;
	const done = total - status.missing.length;
	return (
		<div>
			<div className="flex items-baseline justify-between gap-3 text-meta">
				<span className="text-muted-foreground">
					{m.builder.progress(done, total)}
				</span>
			</div>
			<div className="mt-2 flex gap-1" aria-hidden>
				{status.required.map((s) => (
					<span
						key={s}
						className={cn(
							"h-1.5 flex-1 rounded-full",
							status.missing.includes(s) ? "bg-muted" : "bg-foreground",
						)}
					/>
				))}
			</div>
		</div>
	);
}

function Compatibility({
	check,
	status,
}: {
	check: BuildCheck;
	status: BuildStatus;
}) {
	const empty = check.parts.length === 0 && check.unavailable.length === 0;
	if (empty) {
		return (
			<p className="mt-4 text-meta text-muted-foreground">
				{m.builder.summary.empty}
			</p>
		);
	}
	const unavailable = check.unavailable.length;
	const errors = status.errors + unavailable;
	const outOfStock = check.out_of_stock.length;
	return (
		<div className="mt-4 flex flex-col gap-3">
			{outOfStock ? (
				<p className="flex items-center gap-2 font-medium text-destructive text-sm">
					<SeverityIcon severity="error" />
					{m.builder.summary.outOfStock(outOfStock)}
				</p>
			) : null}
			{errors === 0 ? (
				<p className="flex items-center gap-2 font-medium text-sm text-success">
					<CircleCheck className="size-4" />
					{status.missing.length
						? m.builder.summary.compatibleSoFar
						: m.builder.summary.compatible}
				</p>
			) : (
				<p className="flex items-center gap-2 font-medium text-destructive text-sm">
					<SeverityIcon severity="error" />
					{m.builder.summary.errors(errors)}
				</p>
			)}
			{status.problems.length || unavailable || outOfStock ? (
				<ul className="flex flex-col gap-1">
					{check.out_of_stock.map((u) => (
						<ProblemLink
							key={`stock:${u.slot}:${u.slug}`}
							slot={u.slot}
							severity="error"
							text={m.builder.summary.outOfStockItem(m.builder.slot[u.slot])}
						/>
					))}
					{check.unavailable.map((u) => (
						<ProblemLink
							key={`${u.slot}:${u.slug}`}
							slot={u.slot}
							severity="error"
							text={`${m.builder.slot[u.slot]}: ${m.builder.unavailable.toLowerCase()}`}
						/>
					))}
					{status.problems.map((f: Finding) => (
						<ProblemLink
							key={findingKey(f)}
							slot={f.slots[0] as Slot}
							severity={f.severity}
							text={findingText(f)}
						/>
					))}
				</ul>
			) : null}
		</div>
	);
}

function ProblemLink({
	slot,
	severity,
	text,
}: {
	slot: Slot;
	severity: Finding["severity"];
	text: string;
}) {
	return (
		<li>
			<button
				type="button"
				onClick={() => scrollToSlot(slot)}
				className="-mx-2 flex w-[calc(100%+1rem)] gap-2 rounded-md px-2 py-1.5 text-left text-xs transition-colors hover:bg-muted"
			>
				<SeverityIcon severity={severity} className="mt-px size-3.5" />
				<span className="text-muted-foreground">{text}</span>
			</button>
		</li>
	);
}

// PowerMeter shows the estimated peak draw against the PSU, or the PSU to pick before there is
// one. Above 80% the bar turns amber and above 100% red, as the PSU_WATTAGE rule does.
function PowerMeter({ check }: { check: BuildCheck }) {
	const psu = check.parts.find((p) => p.slot === "psu");
	const psuW =
		typeof psu?.specs.wattage_w === "number" ? psu.specs.wattage_w : 0;
	const est = check.estimated_power_w;
	const scale = psuW || check.recommended_psu_w;
	const load = scale ? est / scale : 0;
	const color = !psuW
		? "bg-foreground/70"
		: load > 1
			? "bg-destructive"
			: load > 0.8
				? "bg-warning"
				: "bg-success";
	return (
		<div>
			<div className="flex items-baseline justify-between gap-3">
				<span className="text-muted-foreground text-sm">
					{m.builder.summary.power}
				</span>
				<span className="font-medium text-sm tabular-nums">
					~{est} W
					{psuW ? (
						<span className="font-normal text-subtle-foreground">
							{" "}
							{m.builder.summary.ofPsu(psuW)}
						</span>
					) : null}
				</span>
			</div>
			<div
				className="mt-2 h-1.5 overflow-hidden rounded-full bg-muted"
				// The numbers are in the text above; the bar is only a picture of them.
				aria-hidden
			>
				<div
					className={cn("h-full rounded-full transition-[width]", color)}
					style={{ width: `${Math.min(100, load * 100)}%` }}
				/>
			</div>
			<p className="mt-2 text-subtle-foreground text-xs">
				{m.builder.summary.estimate(est)}.{" "}
				{m.builder.summary.recommendedPsu(check.recommended_psu_w)}
			</p>
		</div>
	);
}
