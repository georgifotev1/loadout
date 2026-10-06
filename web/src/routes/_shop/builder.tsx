import { useQueryClient, useSuspenseQuery } from "@tanstack/react-query";
import { createFileRoute, redirect } from "@tanstack/react-router";
import { RotateCcw, Share2 } from "lucide-react";
import { useEffect, useState } from "react";
import { AddBuildToCart } from "#/components/builder/add-to-cart";
import { BuildSummary } from "#/components/builder/build-summary";
import { SeverityIcon } from "#/components/builder/finding-note";
import {
	PartPicker,
	type PickerTarget,
} from "#/components/builder/part-picker";
import { ShareBar, useShareBuild } from "#/components/builder/share-build";
import {
	BuildSlots,
	type SlotActions,
	scrollToSlot,
} from "#/components/builder/slot-row";
import { Price } from "#/components/product/price";
import { Container } from "#/components/site/container";
import { PageBreadcrumb } from "#/components/site/page-breadcrumb";
import { Button } from "#/components/ui/button";
import { useRefreshing } from "#/hooks/use-refreshing";
import { m } from "#/i18n";
import { buildCheckQuery, builderOptionsQuery } from "#/lib/api/builder";
import { ApiError } from "#/lib/api/client";
import {
	type BuildSearch,
	buildStatus,
	canOrder,
	hasPowerParts,
	isEmptyBuild,
	parseBuildSearch,
	pick,
	remove,
	type Slot,
	setCount,
	slotOrder,
	slugsIn,
	without,
} from "#/lib/builder";
import { cn } from "#/lib/utils";

// /builder?cpu=…&ram=[…]: the PC builder. The build lives in the URL, so a reload or a pasted
// link shows the same build; "share" saves it under a short /build/{slug} link.
export const Route = createFileRoute("/_shop/builder")({
	validateSearch: parseBuildSearch,
	loaderDeps: ({ search }) => search,
	loader: async ({ context, deps }) => {
		await context.queryClient.query(buildCheckQuery(deps)).catch((err) => {
			// A slug from the wrong category (a hand-edited link): drop that slot, keep the rest.
			if (err instanceof ApiError && err.status === 422) {
				const bad = new Set(err.fields.map((f) => f.field));
				let search = deps;
				for (const s of slotOrder) if (bad.has(s)) search = without(search, s);
				throw redirect({ to: "/builder", search, replace: true });
			}
			throw err;
		});
	},
	head: () => ({
		meta: [
			{ title: m.site.title(m.builder.title) },
			{ name: "description", content: m.builder.metaDescription },
		],
		links: [{ rel: "canonical", href: "/builder" }],
	}),
	component: BuilderPage,
});

function BuilderPage() {
	const build = Route.useSearch();
	const navigate = Route.useNavigate();
	const queryClient = useQueryClient();
	const { data: check } = useSuspenseQuery(buildCheckQuery(build));
	const status = buildStatus(check);
	const loading = useRefreshing();
	const share = useShareBuild(build);

	const [target, setTarget] = useState<PickerTarget>();
	const [pickerOpen, setPickerOpen] = useState(false);
	const [cleared, setCleared] = useState<BuildSearch>();

	// Each pick replaces the history entry: Back leaves the builder rather than stepping
	// through every part tried.
	const go = (next: BuildSearch) => {
		setCleared(undefined);
		navigate({ search: next, replace: true, resetScroll: false });
	};

	// A prefetch is a hint: if it fails, the picker loads (and shows the error) when it opens.
	const prefetch = (slot: Slot) => {
		queryClient
			.query(builderOptionsQuery(slot, without(build, slot)))
			.catch(() => {});
	};

	// Load the next step's options ahead, so its picker opens full.
	useEffect(() => {
		if (status.next) prefetch(status.next);
	});

	const actions: SlotActions = {
		onChoose: (slot, replacing) => {
			const current = replacing
				? [replacing]
				: slot === "storage"
					? []
					: slugsIn(build, slot);
			const part = check.parts.find(
				(p) => p.slot === slot && current.includes(p.product.slug),
			);
			setTarget({
				slot,
				replacing,
				current,
				currentCents: part?.product.price.amount_cents,
			});
			setPickerOpen(true);
		},
		onRemove: (slot, slug) => go(remove(build, slot, slug)),
		onCount: (slot, slug, n) => go(setCount(build, slot, slug, n)),
		onPrefetch: prefetch,
	};

	const onPick = (slug: string) => {
		if (!target) return;
		setPickerOpen(false);
		go(pick(build, target.slot, slug, target.replacing));
	};

	const empty = isEmptyBuild(build);
	const orderable = canOrder(check, status);

	return (
		<Container className="pt-6 pb-28 lg:pb-0">
			<PageBreadcrumb current={m.builder.title} />

			<div className="mt-6 flex flex-wrap items-end justify-between gap-4">
				<div className="max-w-2xl">
					<h1 className="font-semibold text-3xl tracking-[-0.03em]">
						{m.builder.title}
					</h1>
					<p className="mt-2 text-muted-foreground">{m.builder.lead}</p>
				</div>
				<div className="flex items-center gap-2">
					{!empty ? (
						<Button
							variant="ghost"
							onClick={() => {
								go({});
								setCleared(build);
							}}
							className="text-muted-foreground"
						>
							{m.builder.clear}
						</Button>
					) : null}
					<Button
						variant="outline"
						onClick={share.share}
						disabled={empty || share.pending}
					>
						<Share2 />
						{share.pending ? m.builder.sharing : m.builder.share}
					</Button>
				</div>
			</div>

			<div className="mt-4 flex flex-col gap-2 empty:hidden">
				<ShareBar state={share} />
				{cleared ? (
					<div className="flex items-center justify-between gap-3 rounded-xl border bg-card px-4 py-2.5 text-meta text-muted-foreground">
						{m.builder.clearedNote}
						<Button variant="outline" size="sm" onClick={() => go(cleared)}>
							<RotateCcw /> {m.builder.undo}
						</Button>
					</div>
				) : null}
			</div>

			<div className="mt-6 grid gap-8 lg:grid-cols-[1fr_22rem] lg:gap-10">
				<div
					aria-busy={loading}
					className={cn(
						"transition-opacity",
						loading && "opacity-60 delay-150",
					)}
				>
					<BuildSlots check={check} status={status} actions={actions} />
				</div>

				<aside
					id="summary"
					className="scroll-mt-[calc(var(--spacing-header)+2.5rem+1rem)] lg:sticky lg:top-[calc(var(--spacing-header)+2.5rem+1.5rem)] lg:self-start"
				>
					<BuildSummary check={check} status={status}>
						<AddBuildToCart check={check} orderable={orderable} />
					</BuildSummary>
				</aside>
			</div>

			<MobileSummaryBar
				totalCents={check.total.amount_cents}
				missing={status.missing.length}
				errors={status.errors + check.unavailable.length}
				warnings={status.warnings}
				onNext={
					status.next ? () => scrollToSlot(status.next as Slot) : undefined
				}
			/>

			<PartPicker
				open={pickerOpen}
				onOpenChange={setPickerOpen}
				target={target}
				build={build}
				recommendedPsuW={hasPowerParts(check) ? check.recommended_psu_w : 0}
				onPick={onPick}
			/>
		</Container>
	);
}

// MobileSummaryBar keeps the total and status in view on phones, where the summary is below
// the parts. It hides while the summary itself is on screen.
function MobileSummaryBar({
	totalCents,
	missing,
	errors,
	warnings,
	onNext,
}: {
	totalCents: number;
	missing: number;
	errors: number;
	warnings: number;
	onNext?: () => void;
}) {
	const [summaryVisible, setSummaryVisible] = useState(false);
	useEffect(() => {
		const el = document.getElementById("summary");
		if (!el) return;
		const io = new IntersectionObserver(([e]) =>
			setSummaryVisible(e?.isIntersecting ?? false),
		);
		io.observe(el);
		return () => io.disconnect();
	}, []);

	return (
		<div
			className={cn(
				"fixed inset-x-0 bottom-0 z-30 border-t bg-card/95 pb-[env(safe-area-inset-bottom,0px)] backdrop-blur-md transition-transform duration-300 lg:hidden",
				summaryVisible && "translate-y-full",
			)}
		>
			<div className="mx-auto flex h-16 max-w-page items-center gap-4 px-4">
				<div className="min-w-0 flex-1">
					<Price cents={totalCents} className="text-lg" />
					<p className="flex items-center gap-1.5 truncate text-xs text-muted-foreground">
						{errors ? (
							<>
								<SeverityIcon severity="error" className="size-3.5" />
								{m.builder.summary.errors(errors)}
							</>
						) : warnings ? (
							<>
								<SeverityIcon severity="warning" className="size-3.5" />
								{m.builder.summary.warnings(warnings)}
							</>
						) : missing ? (
							m.builder.summary.missing(missing)
						) : (
							m.builder.summary.compatible
						)}
					</p>
				</div>
				{onNext ? (
					<Button variant="outline" onClick={onNext}>
						{m.builder.next}
					</Button>
				) : null}
				<Button
					onClick={() =>
						document
							.getElementById("summary")
							?.scrollIntoView({ behavior: "smooth", block: "start" })
					}
				>
					{m.builder.summary.view}
				</Button>
			</div>
		</div>
	);
}
