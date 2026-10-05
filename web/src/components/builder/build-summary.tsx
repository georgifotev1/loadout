import { CircleCheck } from "lucide-react";
import { Price } from "#/components/product/price";
import { m } from "#/i18n";
import {
	type BuildCheck,
	type BuildStatus,
	type Finding,
	findingText,
	hasPowerParts,
	type Slot,
} from "#/lib/builder";
import { cn } from "#/lib/utils";
import { SeverityIcon } from "./finding-note";
import { findingKey, scrollToSlot } from "./slot-row";

// BuildSummary is the build's total, compatibility and power at a glance, with the actions
// (`children`) at the bottom.
export function BuildSummary({
	check,
	status,
	children,
}: {
	check: BuildCheck;
	status: BuildStatus;
	children?: React.ReactNode;
}) {
	return (
		<div className="overflow-hidden rounded-xl border bg-card">
			<div className="p-5">
				<p className="text-muted-foreground text-sm">
					{m.builder.summary.total}
				</p>
				<Price
					cents={check.total.amount_cents}
					className="mt-1 block text-3xl tracking-[-0.03em]"
				/>
				<p className="mt-1 text-subtle-foreground text-xs">
					{m.builder.summary.vatIncluded} ·{" "}
					{m.builder.summary.parts(check.parts.length)}
				</p>
			</div>

			<div className="border-t p-5">
				<Progress status={status} />
				<Compatibility check={check} status={status} />
			</div>

			{hasPowerParts(check) ? (
				<div className="border-t p-5">
					<PowerMeter check={check} />
				</div>
			) : null}

			{children ? <div className="border-t p-5">{children}</div> : null}
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
	return (
		<div className="mt-4 flex flex-col gap-3">
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
			{status.problems.length || unavailable ? (
				<ul className="flex flex-col gap-1">
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
