import { m } from "#/i18n";
import { cn } from "#/lib/utils";

export function Logo({ className }: { className?: string }) {
	return (
		<span
			className={cn(
				"font-semibold text-lg tracking-[-0.04em] text-foreground",
				className,
			)}
		>
			{m.site.name}
			<span className="text-brand">.</span>
		</span>
	);
}
