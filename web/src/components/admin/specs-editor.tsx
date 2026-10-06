import { useMutation, useQueryClient } from "@tanstack/react-query";
import { Link, useNavigate } from "@tanstack/react-router";
import { CircleAlert, CircleCheck, ExternalLink, X } from "lucide-react";
import { useState } from "react";
import { ProductImage } from "#/components/product/product-image";
import { Badge } from "#/components/ui/badge";
import { Button } from "#/components/ui/button";
import { Input } from "#/components/ui/input";
import {
	Select,
	SelectContent,
	SelectItem,
	SelectTrigger,
	SelectValue,
} from "#/components/ui/select";
import { m } from "#/i18n";
import { fieldLabel, type SpecsSearch } from "#/lib/admin-specs";
import {
	type ProductSpecs,
	productSpecsQuery,
	reviewQueueQuery,
	type SpecField,
	updateProductSpecs,
} from "#/lib/api/admin-specs";
import { ApiError } from "#/lib/api/client";
import { changedValues, draftOf, toText } from "#/lib/spec-form";
import { cn } from "#/lib/utils";

const t = m.admin.specs.editor;

const draftsOf = (fields: SpecField[]) =>
	Object.fromEntries(fields.map((f) => [f.name, draftOf(f)]));

// SpecsEditor shows the supplier's spec rows next to the product's canonical fields, so the
// admin can read a value on the left and type it on the right. Saving sends only the fields
// that changed; the API decides what's valid and what's still missing.
export function SpecsEditor({
	specs,
	search,
}: {
	specs: ProductSpecs;
	search: SpecsSearch;
}) {
	const queryClient = useQueryClient();
	const navigate = useNavigate();
	const [drafts, setDrafts] = useState(() => draftsOf(specs.fields));
	const [saved, setSaved] = useState(false);
	const id = specs.product_id;

	const save = useMutation({
		mutationFn: (values: Record<string, unknown>) =>
			updateProductSpecs(id, values),
		onSuccess: (res) => {
			queryClient.setQueryData(productSpecsQuery(id).queryKey, res);
			queryClient.invalidateQueries({ queryKey: ["admin", "specs", "queue"] });
			setDrafts(draftsOf(res.fields));
			setSaved(true);
		},
	});

	const set = (name: string, draft: string) => {
		setDrafts((d) => ({ ...d, [name]: draft }));
		setSaved(false);
	};

	// Saves what changed; resolves to false if the save failed.
	const submit = async () => {
		const values = changedValues(specs.fields, drafts);
		if (Object.keys(values).length === 0) return true;
		return save.mutateAsync(values).then(
			() => true,
			() => false,
		);
	};

	// The queue is oldest first and a save moves this product to its end, so the next product is
	// the first one that isn't this.
	const saveAndNext = async () => {
		if (!(await submit())) return;
		const queue = await queryClient.query({
			...reviewQueueQuery({ category: search.category, page_size: 2 }),
			staleTime: 0,
		});
		const next = queue.items.find((it) => it.product_id !== id);
		if (next) {
			navigate({
				to: "/admin/specs/$productId",
				params: { productId: next.product_id },
				search: { category: search.category },
			});
		} else {
			navigate({ to: "/admin/specs", search: { category: search.category } });
		}
	};

	const errors = fieldErrors(save.error, specs.fields);
	const missing = specs.missing.map((f) => fieldLabel(m.admin.specs.field, f));

	return (
		<>
			<Link
				to="/admin/specs"
				search={{ category: search.category }}
				className="text-muted-foreground text-sm hover:text-foreground"
			>
				← {t.back}
			</Link>
			<div className="mt-3 flex flex-wrap items-start gap-4">
				<ProductImage
					src={specs.image}
					alt=""
					className="size-16 shrink-0 rounded-md border p-1.5"
				/>
				<div className="min-w-0 flex-1">
					<h1 className="font-semibold text-xl tracking-[-0.01em]">
						{specs.product_name}
					</h1>
					<p className="mt-1 flex flex-wrap items-center gap-x-3 gap-y-1 text-muted-foreground text-sm">
						<span>{m.categories[specs.category]}</span>
						<span>{specs.brand}</span>
						<span className="font-mono text-xs">{specs.sku}</span>
						<Badge
							variant={specs.status === "approved" ? "outline" : "secondary"}
						>
							{specs.status === "approved"
								? t.statusApproved
								: t.statusIncomplete}
						</Badge>
						{specs.active ? (
							<a
								href={`/p/${specs.slug}`}
								target="_blank"
								rel="noreferrer"
								className="inline-flex items-center gap-1 hover:text-foreground"
							>
								{t.viewInShop}
								<ExternalLink className="size-3.5" />
							</a>
						) : (
							<span>{t.inactive}</span>
						)}
					</p>
				</div>
			</div>

			<div className="mt-6 grid items-start gap-6 lg:grid-cols-[minmax(0,2fr)_minmax(0,3fr)]">
				<section className="rounded-lg border bg-card lg:sticky lg:top-[calc(var(--spacing-header)+1.5rem)]">
					<h2 className="border-b px-4 py-3 font-medium text-sm">
						{t.supplierData}
					</h2>
					<Attributes attributes={specs.attributes} />
				</section>

				<form
					className="rounded-lg border bg-card"
					onSubmit={(e) => {
						e.preventDefault();
						submit();
					}}
				>
					<div className="border-b px-4 py-3">
						<h2 className="font-medium text-sm">{t.fields}</h2>
						<p className="mt-1 text-muted-foreground text-xs">{t.fieldsLead}</p>
					</div>
					<div className="divide-y">
						{specs.fields.map((f) => (
							<FieldRow
								key={f.name}
								field={f}
								category={specs.category}
								draft={drafts[f.name] ?? ""}
								error={errors[f.name]}
								onChange={(d) => set(f.name, d)}
							/>
						))}
					</div>
					<div className="sticky bottom-0 flex flex-wrap items-center gap-3 rounded-b-lg border-t bg-card px-4 py-3">
						<p
							className={cn(
								"mr-auto flex items-center gap-2 text-sm",
								missing.length ? "text-muted-foreground" : "text-success",
							)}
							aria-live="polite"
						>
							{save.isError && !Object.keys(errors).length ? (
								<span className="flex items-center gap-2 text-destructive">
									<CircleAlert className="size-4" />
									{t.errors.generic}
								</span>
							) : missing.length ? (
								t.missingNow(missing.join(", "))
							) : (
								<>
									<CircleCheck className="size-4" />
									{saved ? `${t.saved} ${t.complete}` : t.complete}
								</>
							)}
						</p>
						<Button type="submit" variant="outline" disabled={save.isPending}>
							{save.isPending ? t.saving : t.save}
						</Button>
						<Button
							type="button"
							disabled={save.isPending}
							onClick={saveAndNext}
						>
							{t.saveNext}
						</Button>
					</div>
				</form>
			</div>
		</>
	);
}

function Attributes({ attributes }: { attributes: Record<string, string> }) {
	const rows = Object.entries(attributes);
	if (rows.length === 0) {
		return (
			<p className="px-4 py-6 text-muted-foreground text-sm">
				{t.noAttributes}
			</p>
		);
	}
	return (
		<dl className="max-h-[70vh] divide-y overflow-y-auto text-sm">
			{rows.map(([k, v]) => (
				<div
					key={k}
					className="grid grid-cols-[minmax(0,2fr)_minmax(0,3fr)] gap-3 px-4 py-2"
				>
					<dt className="text-muted-foreground">{k}</dt>
					<dd className="select-all wrap-break-word">{v}</dd>
				</div>
			))}
		</dl>
	);
}

function FieldRow({
	field: f,
	category,
	draft,
	error,
	onChange,
}: {
	field: SpecField;
	category: string;
	draft: string;
	error?: string;
	onChange: (draft: string) => void;
}) {
	const label = fieldLabel(m.admin.specs.field, f.name);
	const inputId = `spec-${f.name}`;
	const hintId = `${inputId}-hint`;
	const missing = f.required && f.value === null;
	// What counts when the admin leaves the field empty: the supplier's value, or one derived from
	// other fields (a CPU's memory types from its socket).
	const fallback = f.admin === null ? f.value : f.supplier;
	const note =
		f.supplier !== null
			? t.fromSupplier(display(f, f.supplier))
			: fallback !== null
				? t.derived(display(f, fallback))
				: f.kind === "string_list"
					? t.listHint
					: undefined;
	const example =
		m.admin.specs.example[`${category}.${f.name}`] ??
		m.admin.specs.example[f.name];

	return (
		<div className="grid gap-x-4 gap-y-1.5 px-4 py-3 sm:grid-cols-[13rem_minmax(0,1fr)]">
			<div className="pt-2">
				<label
					htmlFor={inputId}
					className="flex items-center gap-1.5 font-medium text-sm"
				>
					{missing ? (
						<span
							className="size-1.5 shrink-0 rounded-full bg-destructive"
							aria-hidden
						/>
					) : null}
					{label}
				</label>
				<p className="mt-0.5 flex items-center gap-2 text-subtle-foreground text-xs">
					<span className="font-mono">{f.name}</span>
					{f.required ? <span>{t.required}</span> : null}
				</p>
			</div>
			<div>
				<div className="flex items-center gap-2">
					{f.kind === "boolean" ? (
						<BooleanInput
							id={inputId}
							value={draft}
							placeholder={fallback === null ? t.notSet : display(f, fallback)}
							invalid={!!error}
							describedBy={hintId}
							onChange={onChange}
						/>
					) : (
						<Input
							id={inputId}
							value={draft}
							inputMode={f.kind === "integer" ? "numeric" : undefined}
							placeholder={
								fallback !== null ? toText(fallback) : (example ?? "")
							}
							aria-invalid={error ? true : undefined}
							aria-describedby={hintId}
							autoComplete="off"
							onChange={(e) => onChange(e.target.value)}
						/>
					)}
					<Button
						type="button"
						variant="ghost"
						size="icon-sm"
						className={cn(draft === "" && "invisible")}
						aria-label={t.clear(label)}
						onClick={() => onChange("")}
					>
						<X />
					</Button>
				</div>
				<p id={hintId} className="mt-1 text-xs">
					{error ? (
						<span className="text-destructive">{error}</span>
					) : note ? (
						<span className="text-muted-foreground">{note}</span>
					) : null}
				</p>
			</div>
		</div>
	);
}

const booleanItems = [
	{ value: "", label: t.notSet },
	{ value: "true", label: t.yes },
	{ value: "false", label: t.no },
];

function BooleanInput({
	id,
	value,
	placeholder,
	invalid,
	describedBy,
	onChange,
}: {
	id: string;
	value: string;
	placeholder: string;
	invalid: boolean;
	describedBy: string;
	onChange: (v: string) => void;
}) {
	return (
		<Select
			items={booleanItems}
			value={value}
			onValueChange={(v) => onChange(typeof v === "string" ? v : "")}
		>
			<SelectTrigger
				id={id}
				className="w-full"
				aria-invalid={invalid || undefined}
				aria-describedby={describedBy}
			>
				{value === "" ? (
					<span className="text-muted-foreground">{placeholder}</span>
				) : (
					<SelectValue />
				)}
			</SelectTrigger>
			<SelectContent>
				{booleanItems.map((o) => (
					<SelectItem key={o.value} value={o.value}>
						{o.label}
					</SelectItem>
				))}
			</SelectContent>
		</Select>
	);
}

/** A value as the admin reads it: booleans as yes/no. */
function display(f: SpecField, v: unknown): string {
	if (f.kind === "boolean") return v === true ? t.yes : v === false ? t.no : "";
	return toText(v);
}

/** Field errors from a failed save, by canonical field name. */
function fieldErrors(
	err: unknown,
	fields: SpecField[],
): Record<string, string> {
	if (!(err instanceof ApiError) || err.code !== "validation_failed") return {};
	const kinds = new Map(fields.map((f) => [f.name, f.kind]));
	const out: Record<string, string> = {};
	for (const fe of err.fields) {
		out[fe.field] =
			fe.code === "invalid_type"
				? kinds.get(fe.field) === "boolean"
					? t.errors.boolean
					: t.errors.integer
				: fe.code === "out_of_range"
					? t.errors.outOfRange
					: fe.code === "required"
						? t.errors.required
						: t.errors.unknown;
	}
	return out;
}
