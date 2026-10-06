import { Input } from "#/components/ui/input";
import { Label } from "#/components/ui/label";
import { cn } from "#/lib/utils";

// CodeField is the input for a one-time code: the app's 6 digits, or a recovery code when
// `recovery` is set. Both are what the API accepts as typed (spaces, dashes, any case).
export function CodeField({
	label,
	recovery = false,
	autoFocus = false,
	errorId,
	invalid,
}: {
	label: string;
	recovery?: boolean;
	autoFocus?: boolean;
	errorId: string;
	invalid: boolean;
}) {
	return (
		<div className="flex flex-col gap-2">
			<Label htmlFor="code">{label}</Label>
			<Input
				// A new element when the mode changes, so the browser drops the other mode's value.
				key={recovery ? "recovery" : "app"}
				id="code"
				name="code"
				required
				autoFocus={autoFocus}
				autoComplete={recovery ? "off" : "one-time-code"}
				inputMode={recovery ? "text" : "numeric"}
				pattern={recovery ? undefined : "[0-9 ]{6,7}"}
				maxLength={recovery ? 24 : 7}
				placeholder={recovery ? "xxxx-xxxx-xxxx-xxxx" : "123 456"}
				spellCheck={false}
				aria-invalid={invalid || undefined}
				aria-describedby={invalid ? errorId : undefined}
				className={cn(
					"h-11 font-mono text-lg tracking-[0.2em] md:text-lg",
					recovery && "tracking-normal",
				)}
			/>
		</div>
	);
}
