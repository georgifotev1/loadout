import { keepPreviousData, useQuery } from "@tanstack/react-query";
import { Clock, MapPin, Search } from "lucide-react";
import { useState } from "react";
import { Button } from "#/components/ui/button";
import { Input } from "#/components/ui/input";
import { Label } from "#/components/ui/label";
import { RadioGroup, RadioGroupItem } from "#/components/ui/radio-group";
import { useDebouncedValue } from "#/hooks/use-debounced-value";
import { m } from "#/i18n";
import { type Office, officesQuery } from "#/lib/api/orders";

const t = m.checkout;

// The API returns at most this many offices; a full page means there may be more.
const maxOffices = 20;

// OfficePicker finds one of the courier's offices or lockers by city, area or street, and shows
// the chosen one with a way to change it. The list sits under the search box rather than in a
// popover, so it stays usable on a phone with the keyboard open.
export function OfficePicker({
	method,
	value,
	onChange,
	error,
}: {
	method: "office" | "apt";
	value: Office | null;
	onChange: (office: Office | null) => void;
	error?: string;
}) {
	const [text, setText] = useState("");
	const q = useDebouncedValue(text.trim(), 250);
	const { data, isFetching, isError, isEnabled } = useQuery({
		...officesQuery(q, method),
		placeholderData: keepPreviousData,
	});
	const offices = isEnabled ? (data?.offices ?? []) : [];
	const id = "field-office_id";
	const described = error ? `${id}-error` : `${id}-hint`;

	if (value) {
		return (
			<div className="flex items-start gap-3 rounded-lg border p-4">
				<MapPin className="mt-0.5 size-5 shrink-0 text-muted-foreground" />
				<OfficeText office={value} />
				<Button
					type="button"
					variant="outline"
					size="sm"
					onClick={() => onChange(null)}
				>
					{t.changeOffice}
				</Button>
			</div>
		);
	}

	return (
		<div className="flex flex-col gap-2">
			<Label htmlFor={id}>{t.officeSearch[method]}</Label>
			<div className="relative">
				<Search className="-translate-y-1/2 pointer-events-none absolute top-1/2 left-3 size-4 text-muted-foreground" />
				<Input
					id={id}
					// The form focuses the first wrong field by name, and clears its mistake on input.
					name="office_id"
					type="search"
					autoComplete="off"
					placeholder={t.officeSearchPlaceholder}
					value={text}
					onChange={(e) => setText(e.target.value)}
					aria-invalid={error ? true : undefined}
					aria-describedby={described}
					className="pl-9"
				/>
			</div>
			{error ? (
				<p id={`${id}-error`} className="text-destructive text-sm">
					{error}
				</p>
			) : (
				<p id={`${id}-hint`} className="text-muted-foreground text-sm">
					{!isEnabled
						? t.officeSearchHint
						: isError
							? t.officesFailed
							: isFetching && !data
								? t.officesSearching
								: offices.length === 0
									? t.officesNone
									: offices.length >= maxOffices
										? t.officesMore
										: null}
				</p>
			)}
			{offices.length > 0 ? (
				<RadioGroup
					value={null}
					onValueChange={(v) =>
						onChange(offices.find((o) => o.id === v) ?? null)
					}
					aria-label={t.officeSearch[method]}
					className="mt-1 grid max-h-80 gap-0 overflow-y-auto rounded-lg border"
				>
					{offices.map((o) => (
						<Label
							key={o.id}
							htmlFor={`office-${o.id}`}
							className="flex cursor-pointer items-start gap-3 border-b p-3 font-normal transition-colors last:border-b-0 hover:bg-muted"
						>
							<RadioGroupItem
								value={o.id}
								id={`office-${o.id}`}
								className="mt-0.5"
							/>
							<OfficeText office={o} />
						</Label>
					))}
				</RadioGroup>
			) : null}
		</div>
	);
}

function OfficeText({ office }: { office: Office }) {
	return (
		<div className="min-w-0 flex-1 text-sm">
			<p className="font-medium">{office.name}</p>
			<p className="text-muted-foreground">
				{office.address}, {office.postcode} {office.city}
			</p>
			{office.hours ? (
				<p className="mt-0.5 flex items-center gap-1.5 text-muted-foreground text-xs">
					<Clock className="size-3" />
					{office.hours}
				</p>
			) : null}
		</div>
	);
}
