import {
	useMutation,
	useQueryClient,
	useSuspenseQuery,
} from "@tanstack/react-query";
import { createFileRoute } from "@tanstack/react-router";
import { Pencil, Plus, RefreshCw, Trash2 } from "lucide-react";
import { useId, useState } from "react";
import { AdminPage } from "#/components/admin/admin-page";
import { AdminError, AdminNotFound } from "#/components/admin/admin-status";
import { adminColumns, DataTable } from "#/components/admin/data-table";
import {
	AlertDialog,
	AlertDialogAction,
	AlertDialogCancel,
	AlertDialogContent,
	AlertDialogDescription,
	AlertDialogFooter,
	AlertDialogHeader,
	AlertDialogTitle,
	AlertDialogTrigger,
} from "#/components/ui/alert-dialog";
import { Button } from "#/components/ui/button";
import {
	Dialog,
	DialogContent,
	DialogDescription,
	DialogFooter,
	DialogHeader,
	DialogTitle,
} from "#/components/ui/dialog";
import { Input } from "#/components/ui/input";
import { Label } from "#/components/ui/label";
import {
	Select,
	SelectContent,
	SelectItem,
	SelectTrigger,
	SelectValue,
} from "#/components/ui/select";
import {
	Tooltip,
	TooltipContent,
	TooltipTrigger,
} from "#/components/ui/tooltip";
import { formatPercent, formatPrice, formatShortDate, m } from "#/i18n";
import { triggerSync } from "#/lib/api/admin";
import {
	createMarkupRule,
	deleteMarkupRule,
	type MarkupRule,
	markupRulesQuery,
	type Rounding,
	type RuleScope,
	updateMarkupRule,
} from "#/lib/api/admin-catalog";
import { ApiError } from "#/lib/api/client";
import { type Category, categories } from "#/lib/categories";
import {
	type DraftErrors,
	draftOf,
	newRuleDraft,
	type RuleDraft,
	toRuleInput,
} from "#/lib/markup-rules";
import { cn } from "#/lib/utils";

// /admin/markup-rules: the rules that turn dealer prices into retail prices. Changes apply at
// the next sync, which the page offers to start.
export const Route = createFileRoute("/admin/markup-rules")({
	loader: ({ context }) => context.queryClient.query(markupRulesQuery),
	head: () => ({ meta: [{ title: m.site.title(m.admin.markupRules.title) }] }),
	errorComponent: AdminError,
	notFoundComponent: AdminNotFound,
	component: MarkupRulesPage,
});

const t = m.admin.markupRules;

function target(r: MarkupRule): string {
	switch (r.scope) {
		case "default":
			return t.everything;
		case "category":
			return r.category ? m.categories[r.category] : "";
		case "brand_category":
			return t.brandTarget(
				r.brand ?? "",
				r.category ? m.categories[r.category] : "",
			);
		case "sku":
			return t.skuTarget(r.sku ?? "");
	}
}

const col = adminColumns<MarkupRule>();
const columns = col.columns([
	col.display({
		id: "target",
		header: t.appliesTo,
		cell: (c) => (
			<div className="flex flex-col gap-0.5">
				<span className="font-medium">{target(c.row.original)}</span>
				<span className="text-subtle-foreground text-xs">
					{t.scopes[c.row.original.scope]}
				</span>
			</div>
		),
		meta: { className: "w-full min-w-64 whitespace-normal" },
	}),
	col.accessor("markup_bp", {
		header: t.markup,
		cell: (c) => formatPercent(c.getValue()),
		meta: { className: "text-right tabular-nums font-medium" },
	}),
	col.accessor("min_margin_cents", {
		header: t.minMargin,
		cell: (c) => formatPrice(c.getValue()),
		meta: { className: "text-right tabular-nums" },
	}),
	col.accessor("rounding", {
		header: t.rounding,
		cell: (c) => t.roundings[c.getValue()],
		meta: { className: "text-muted-foreground" },
	}),
	col.accessor("products", {
		header: t.products,
		meta: { className: "text-right tabular-nums" },
	}),
	col.accessor("updated_at", {
		header: t.updated,
		cell: (c) => formatShortDate(new Date(c.getValue())),
		meta: { className: "text-muted-foreground tabular-nums" },
	}),
	col.display({
		id: "actions",
		header: () => <span className="sr-only">{t.actions}</span>,
		cell: (c) => <RuleActions rule={c.row.original} />,
	}),
]);

function MarkupRulesPage() {
	const { data } = useSuspenseQuery(markupRulesQuery);
	const [adding, setAdding] = useState(false);

	return (
		<AdminPage
			title={t.title}
			lead={t.lead}
			actions={
				<>
					<SyncNow />
					<Button onClick={() => setAdding(true)}>
						<Plus />
						{t.add}
					</Button>
				</>
			}
		>
			<p className="mb-3 text-muted-foreground text-sm">
				{t.count(data.items.length)}
			</p>
			<DataTable
				columns={columns}
				data={data.items}
				getRowId={(r) => String(r.id)}
				empty={null}
			/>
			<p className="mt-3 max-w-3xl text-muted-foreground text-xs">
				{t.nextSync}
			</p>
			<RuleDialog open={adding} onOpenChange={setAdding} />
		</AdminPage>
	);
}

// SyncNow queues a supplier sync, so rule changes reach prices without waiting for the next
// scheduled one.
function SyncNow() {
	const queryClient = useQueryClient();
	const sync = useMutation({
		mutationFn: triggerSync,
		onSettled: () => {
			queryClient.invalidateQueries({ queryKey: ["admin", "sync-runs"] });
			queryClient.invalidateQueries({ queryKey: ["admin", "overview"] });
		},
	});
	const message = sync.isError
		? t.syncFailed
		: sync.data
			? sync.data.already_queued
				? t.syncAlreadyQueued
				: t.syncQueued
			: null;

	return (
		<>
			<output className={cn("text-sm", sync.isError && "text-destructive")}>
				{message}
			</output>
			<Button
				variant="outline"
				onClick={() => sync.mutate()}
				disabled={sync.isPending}
			>
				<RefreshCw />
				{t.syncNow}
			</Button>
		</>
	);
}

function RuleActions({ rule }: { rule: MarkupRule }) {
	const [editing, setEditing] = useState(false);
	return (
		<div className="flex items-center justify-end gap-1">
			<Button
				variant="ghost"
				size="icon-sm"
				aria-label={t.edit}
				title={t.edit}
				onClick={() => setEditing(true)}
			>
				<Pencil />
			</Button>
			{rule.scope === "default" ? (
				<Tooltip>
					{/* aria-disabled, not disabled: a disabled button gets no hover or focus for the tooltip. */}
					<TooltipTrigger
						render={
							<Button
								variant="ghost"
								size="icon-sm"
								aria-label={t.delete}
								aria-disabled
								className="cursor-not-allowed opacity-50"
							/>
						}
					>
						<Trash2 />
					</TooltipTrigger>
					<TooltipContent className="max-w-64">
						{t.defaultLocked}
					</TooltipContent>
				</Tooltip>
			) : (
				<DeleteRule rule={rule} />
			)}
			{/* Mounted only while open, so it starts from the rule as it is now. */}
			{editing ? (
				<RuleDialog rule={rule} open onOpenChange={setEditing} />
			) : null}
		</div>
	);
}

function DeleteRule({ rule }: { rule: MarkupRule }) {
	const queryClient = useQueryClient();
	const [confirming, setConfirming] = useState(false);
	const remove = useMutation({
		mutationFn: () => deleteMarkupRule(rule.id),
		onSuccess: () => {
			setConfirming(false);
			queryClient.invalidateQueries({ queryKey: ["admin", "markup-rules"] });
			queryClient.invalidateQueries({ queryKey: ["admin", "products"] });
		},
	});

	return (
		<AlertDialog open={confirming} onOpenChange={setConfirming}>
			<AlertDialogTrigger
				render={
					<Button
						variant="ghost"
						size="icon-sm"
						aria-label={t.delete}
						title={t.delete}
					/>
				}
			>
				<Trash2 />
			</AlertDialogTrigger>
			<AlertDialogContent>
				<AlertDialogHeader>
					<AlertDialogTitle>{t.deleteTitle}</AlertDialogTitle>
					<AlertDialogDescription>
						{target(rule)}. {t.deleteBody(rule.products)}
					</AlertDialogDescription>
				</AlertDialogHeader>
				{remove.isError ? (
					<p role="alert" className="text-destructive text-sm">
						{t.deleteFailed}
					</p>
				) : null}
				<AlertDialogFooter>
					<AlertDialogCancel>{t.cancel}</AlertDialogCancel>
					<AlertDialogAction
						variant="destructive"
						disabled={remove.isPending}
						onClick={() => remove.mutate()}
					>
						{t.delete}
					</AlertDialogAction>
				</AlertDialogFooter>
			</AlertDialogContent>
		</AlertDialog>
	);
}

const f = t.form;

const scopeItems = (["category", "brand_category", "sku"] as const).map(
	(s) => ({ value: s, label: t.scopes[s] }),
);
const categoryItems = categories.map((c) => ({
	value: c,
	label: m.categories[c],
}));
const roundingItems = (["90", "99", "none"] as const).map((r) => ({
	value: r,
	label: t.roundings[r],
}));

type FieldName =
	| "scope"
	| "category"
	| "brand"
	| "sku"
	| "markup_bp"
	| "min_margin_cents"
	| "rounding";

// RuleDialog adds a rule, or edits one when `rule` is given. The default rule's scope is fixed.
function RuleDialog({
	rule,
	open,
	onOpenChange,
}: {
	rule?: MarkupRule;
	open: boolean;
	onOpenChange: (open: boolean) => void;
}) {
	const queryClient = useQueryClient();
	const id = useId();
	const [draft, setDraft] = useState<RuleDraft>(
		rule ? draftOf(rule) : newRuleDraft,
	);
	const [draftErrors, setDraftErrors] = useState<DraftErrors>({});

	const save = useMutation({
		mutationFn: (input: Parameters<typeof createMarkupRule>[0]) =>
			rule
				? updateMarkupRule(rule.id, { ...input, version: rule.version })
				: createMarkupRule(input),
		onSuccess: () => {
			queryClient.invalidateQueries({ queryKey: ["admin", "markup-rules"] });
			close();
		},
		onError: (err) => {
			if (err instanceof ApiError && err.code === "edit_conflict") {
				queryClient.invalidateQueries({ queryKey: ["admin", "markup-rules"] });
			}
		},
	});

	function close() {
		onOpenChange(false);
		// The add dialog stays mounted: start the next one empty.
		if (!rule) {
			setDraft(newRuleDraft);
			setDraftErrors({});
			save.reset();
		}
	}

	const set = <K extends keyof RuleDraft>(key: K, value: RuleDraft[K]) =>
		setDraft((d) => ({ ...d, [key]: value }));

	const errors: Partial<Record<FieldName, string>> = {
		...apiFieldErrors(save.error, draft.scope),
	};
	for (const [field, code] of Object.entries(draftErrors)) {
		if (code) errors[field as FieldName] = f.errors.number;
	}
	const formError =
		save.error instanceof ApiError && save.error.code === "edit_conflict"
			? f.errors.conflict
			: save.isError &&
					!(
						save.error instanceof ApiError &&
						["validation_failed", "duplicate_rule"].includes(save.error.code)
					)
				? f.errors.generic
				: null;

	const isDefault = draft.scope === "default";
	const showCategory =
		draft.scope === "category" || draft.scope === "brand_category";

	return (
		<Dialog open={open} onOpenChange={(o) => (o ? onOpenChange(o) : close())}>
			<DialogContent className="sm:max-w-lg">
				<DialogHeader>
					<DialogTitle>{rule ? f.editTitle : f.addTitle}</DialogTitle>
					<DialogDescription>{f.lead}</DialogDescription>
				</DialogHeader>
				<form
					id={`${id}-form`}
					className="flex flex-col gap-4"
					onSubmit={(e) => {
						e.preventDefault();
						const out = toRuleInput(draft);
						if ("errors" in out) {
							setDraftErrors(out.errors);
							return;
						}
						setDraftErrors({});
						save.mutate(out.input);
					}}
				>
					<Field id={`${id}-scope`} label={f.scope} error={errors.scope}>
						{isDefault ? (
							<p className="text-muted-foreground text-sm">{f.defaultScope}</p>
						) : (
							<Select
								items={scopeItems}
								value={draft.scope}
								onValueChange={(v) => set("scope", v as RuleScope)}
							>
								<SelectTrigger id={`${id}-scope`} className="w-full">
									<SelectValue />
								</SelectTrigger>
								<SelectContent>
									{scopeItems.map((s) => (
										<SelectItem key={s.value} value={s.value}>
											{s.label}
										</SelectItem>
									))}
								</SelectContent>
							</Select>
						)}
					</Field>
					{showCategory ? (
						<Field
							id={`${id}-category`}
							label={f.category}
							error={errors.category}
						>
							<Select
								items={categoryItems}
								value={draft.category || null}
								onValueChange={(v) => set("category", (v ?? "") as Category)}
							>
								<SelectTrigger
									id={`${id}-category`}
									className="w-full"
									aria-invalid={errors.category ? true : undefined}
								>
									<SelectValue placeholder={f.pickCategory} />
								</SelectTrigger>
								<SelectContent>
									{categoryItems.map((c) => (
										<SelectItem key={c.value} value={c.value}>
											{c.label}
										</SelectItem>
									))}
								</SelectContent>
							</Select>
						</Field>
					) : null}
					{draft.scope === "brand_category" ? (
						<Field
							id={`${id}-brand`}
							label={f.brand}
							hint={f.brandHint}
							error={errors.brand}
						>
							<Input
								id={`${id}-brand`}
								value={draft.brand}
								onChange={(e) => set("brand", e.target.value)}
								aria-invalid={errors.brand ? true : undefined}
								maxLength={100}
							/>
						</Field>
					) : null}
					{draft.scope === "sku" ? (
						<Field
							id={`${id}-sku`}
							label={f.sku}
							hint={f.skuHint}
							error={errors.sku}
						>
							<Input
								id={`${id}-sku`}
								value={draft.sku}
								onChange={(e) => set("sku", e.target.value)}
								aria-invalid={errors.sku ? true : undefined}
								maxLength={100}
								className="font-mono"
							/>
						</Field>
					) : null}
					<div className="grid gap-4 sm:grid-cols-2">
						<Field
							id={`${id}-markup`}
							label={f.markup}
							error={errors.markup_bp}
						>
							<Input
								id={`${id}-markup`}
								value={draft.markup}
								onChange={(e) => set("markup", e.target.value)}
								inputMode="decimal"
								aria-invalid={errors.markup_bp ? true : undefined}
								className="tabular-nums"
							/>
						</Field>
						<Field
							id={`${id}-margin`}
							label={f.minMargin}
							hint={f.minMarginHint}
							error={errors.min_margin_cents}
						>
							<Input
								id={`${id}-margin`}
								value={draft.minMargin}
								onChange={(e) => set("minMargin", e.target.value)}
								inputMode="decimal"
								aria-invalid={errors.min_margin_cents ? true : undefined}
								className="tabular-nums"
							/>
						</Field>
					</div>
					<Field
						id={`${id}-rounding`}
						label={f.rounding}
						error={errors.rounding}
					>
						<Select
							items={roundingItems}
							value={draft.rounding}
							onValueChange={(v) => set("rounding", v as Rounding)}
						>
							<SelectTrigger id={`${id}-rounding`} className="w-full">
								<SelectValue />
							</SelectTrigger>
							<SelectContent>
								{roundingItems.map((r) => (
									<SelectItem key={r.value} value={r.value}>
										{r.label}
									</SelectItem>
								))}
							</SelectContent>
						</Select>
					</Field>
				</form>
				{formError ? (
					<p role="alert" className="text-destructive text-sm">
						{formError}
					</p>
				) : null}
				<DialogFooter>
					<Button variant="outline" onClick={close}>
						{f.cancel}
					</Button>
					<Button type="submit" form={`${id}-form`} disabled={save.isPending}>
						{save.isPending ? f.saving : f.save}
					</Button>
				</DialogFooter>
			</DialogContent>
		</Dialog>
	);
}

function Field({
	id,
	label,
	hint,
	error,
	children,
}: {
	id: string;
	label: string;
	hint?: string;
	error?: string;
	children: React.ReactNode;
}) {
	return (
		<div className="flex flex-col gap-1.5">
			<Label htmlFor={id}>{label}</Label>
			{children}
			{error ? (
				<p className="text-destructive text-xs">{error}</p>
			) : hint ? (
				<p className="text-muted-foreground text-xs">{hint}</p>
			) : null}
		</div>
	);
}

/** Errors from a failed save, by API field. A duplicate target is shown on the target field. */
function apiFieldErrors(
	err: unknown,
	scope: RuleScope,
): Partial<Record<FieldName, string>> {
	if (!(err instanceof ApiError)) return {};
	if (err.code === "duplicate_rule") {
		const field: FieldName =
			scope === "sku"
				? "sku"
				: scope === "brand_category"
					? "brand"
					: "category";
		return { [field]: f.errors.duplicate };
	}
	if (err.code !== "validation_failed") return {};
	const out: Partial<Record<FieldName, string>> = {};
	for (const fe of err.fields) {
		out[fe.field as FieldName] =
			fe.code === "required"
				? f.errors.required
				: fe.code === "out_of_range"
					? f.errors.outOfRange
					: fe.code === "too_long"
						? f.errors.tooLong
						: f.errors.unknown;
	}
	return out;
}
