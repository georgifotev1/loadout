import { m } from "#/i18n";
import { cn } from "#/lib/utils";

export function Stock({
	inStock,
	className,
}: {
	inStock: boolean;
	className?: string;
}) {
	return (
		<span
			className={cn(
				"inline-flex items-center gap-1.5 text-muted-foreground text-xs",
				className,
			)}
		>
			<span
				aria-hidden
				className={cn(
					"size-1.5 rounded-full",
					inStock ? "bg-success" : "bg-subtle-foreground/50",
				)}
			/>
			{inStock ? m.product.inStock : m.product.outOfStock}
		</span>
	);
}
