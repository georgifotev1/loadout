import { formatPrice } from "#/i18n";
import { cn } from "#/lib/utils";

export function Price({
	cents,
	className,
}: {
	cents: number;
	className?: string;
}) {
	return (
		<span
			className={cn("font-semibold tabular-nums tracking-tight", className)}
		>
			{formatPrice(cents)}
		</span>
	);
}
