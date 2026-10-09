import { Info, Wrench } from "lucide-react";
import { Price } from "#/components/product/price";
import { Button } from "#/components/ui/button";
import { Checkbox } from "#/components/ui/checkbox";
import { Label } from "#/components/ui/label";
import { m } from "#/i18n";
import { cn } from "#/lib/utils";

const t = m.assembly;

// AssemblyOption lets the shopper have their PC assembled, as the API offers it. When it was
// asked for but the parts no longer make a complete PC, it says so instead, with a way to drop it.
// Nothing shows when it's neither offered nor asked for.
// TODO(phase-7): whether an assembled PC can be returned, and what the shopper must be told
// here, comes with the lawyer's review (step 8).
export function AssemblyOption({
	offered,
	requested,
	priceCents,
	busy,
	onChange,
	id = "assembly",
}: {
	offered: boolean;
	requested: boolean;
	priceCents: number;
	busy?: boolean;
	onChange: (on: boolean) => void;
	id?: string;
}) {
	if (!offered && !requested) return null;
	if (!offered) {
		return (
			<div className="flex items-start gap-2 rounded-lg border border-warning/60 bg-warning/5 p-3 text-sm">
				<Info className="mt-0.5 size-4 shrink-0 text-muted-foreground" />
				<div className="min-w-0 flex-1">
					<p className="text-muted-foreground">{t.notPossible}</p>
					<Button
						variant="link"
						size="sm"
						className="mt-1 h-auto p-0"
						disabled={busy}
						onClick={() => onChange(false)}
					>
						{t.drop}
					</Button>
				</div>
			</div>
		);
	}
	// The label is the checkbox's sibling, not its parent: wrapping it would toggle it twice.
	return (
		<div
			className={cn(
				"flex items-start gap-3 rounded-lg border p-4 transition-colors hover:border-border-strong",
				requested && "border-foreground hover:border-foreground",
			)}
		>
			<Checkbox
				id={id}
				checked={requested}
				onCheckedChange={(on) => onChange(on)}
				disabled={busy}
				className="mt-0.5"
			/>
			<Label
				htmlFor={id}
				className="block min-w-0 flex-1 cursor-pointer font-normal"
			>
				<span className="flex items-center gap-1.5 font-medium text-sm">
					<Wrench className="size-4 text-muted-foreground" />
					{t.option}
				</span>
				<span className="mt-0.5 block text-muted-foreground text-xs">
					{t.detail}
				</span>
			</Label>
			<Price cents={priceCents} className="shrink-0 text-sm" />
		</div>
	);
}
