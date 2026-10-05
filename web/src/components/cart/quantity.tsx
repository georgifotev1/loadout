import { Minus, Plus } from "lucide-react";
import { m } from "#/i18n";
import { cn } from "#/lib/utils";

export function Quantity({
	value,
	onChange,
	max,
	size = "sm",
	className,
}: {
	value: number;
	onChange: (value: number) => void;
	max?: number;
	size?: "sm" | "lg";
	className?: string;
}) {
	const button = cn(
		"flex items-center justify-center text-muted-foreground transition-colors hover:text-foreground disabled:opacity-40",
		size === "sm" ? "size-8" : "size-11",
	);
	return (
		<div
			className={cn(
				"flex items-center rounded-md border bg-card",
				size === "sm" ? "h-8" : "h-11",
				className,
			)}
		>
			<button
				type="button"
				aria-label={m.cart.decrease}
				disabled={value <= 1}
				onClick={() => onChange(value - 1)}
				className={button}
			>
				<Minus className="size-3.5" />
			</button>
			<output
				aria-label={m.product.quantity}
				className="min-w-5 text-center font-medium text-sm tabular-nums"
			>
				{value}
			</output>
			<button
				type="button"
				aria-label={m.cart.increase}
				disabled={max !== undefined && value >= max}
				onClick={() => onChange(value + 1)}
				className={button}
			>
				<Plus className="size-3.5" />
			</button>
		</div>
	);
}
