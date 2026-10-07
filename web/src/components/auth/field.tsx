import { Input } from "#/components/ui/input";
import { Label } from "#/components/ui/label";

// Field is a labelled input for the account forms, with an optional hint and the API's error
// for it.
export function Field({
	name,
	label,
	hint,
	error,
	...props
}: {
	name: string;
	label: string;
	hint?: string;
	error?: string;
} & Omit<React.ComponentProps<"input">, "name" | "id">) {
	const id = `field-${name}`;
	// The error replaces the hint.
	const described = error ? `${id}-error` : hint ? `${id}-hint` : undefined;
	return (
		<div className="flex flex-col gap-2">
			<Label htmlFor={id}>{label}</Label>
			<Input
				id={id}
				name={name}
				aria-invalid={error ? true : undefined}
				aria-describedby={described}
				{...props}
			/>
			{error ? (
				<p id={`${id}-error`} className="text-destructive text-sm">
					{error}
				</p>
			) : hint ? (
				<p id={`${id}-hint`} className="text-muted-foreground text-sm">
					{hint}
				</p>
			) : null}
		</div>
	);
}
