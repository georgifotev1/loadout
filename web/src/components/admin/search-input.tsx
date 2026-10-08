import { Search } from "lucide-react";
import { useEffect, useState } from "react";
import { Input } from "#/components/ui/input";
import { useDebouncedValue } from "#/hooks/use-debounced-value";

// SearchInput puts what the admin types into the URL once they pause, and follows the URL when
// it changes from outside (back, a cleared filter).
export function SearchInput({
	value,
	onChange,
	label,
}: {
	value: string;
	onChange: (q: string) => void;
	label: string;
}) {
	const [text, setText] = useState(value);
	const debounced = useDebouncedValue(text, 300);

	// Keep a trailing space the admin is still typing after.
	useEffect(() => setText((t) => (t.trim() === value ? t : value)), [value]);
	// biome-ignore lint/correctness/useExhaustiveDependencies: only a new debounced text should navigate; onChange and value change with every navigation
	useEffect(() => {
		if (debounced.trim() !== value) onChange(debounced);
	}, [debounced]);

	return (
		<div className="relative w-full sm:w-72">
			<Search className="-translate-y-1/2 pointer-events-none absolute top-1/2 left-2.5 size-4 text-muted-foreground" />
			<Input
				type="search"
				value={text}
				onChange={(e) => setText(e.target.value)}
				placeholder={label}
				aria-label={label}
				className="bg-card pl-8"
			/>
		</div>
	);
}
