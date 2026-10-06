import { cn } from "#/lib/utils";

// AdminPage is the frame every admin page shares: the title, a one-line lead and the page's
// actions on the right, then the content.
export function AdminPage({
	title,
	lead,
	actions,
	className,
	children,
}: {
	title: React.ReactNode;
	lead?: React.ReactNode;
	actions?: React.ReactNode;
	className?: string;
	children: React.ReactNode;
}) {
	return (
		<div className={cn("mx-auto w-full max-w-7xl", className)}>
			<div className="flex flex-wrap items-end justify-between gap-4">
				<div className="max-w-2xl">
					<h1 className="font-semibold text-2xl tracking-[-0.02em]">{title}</h1>
					{lead ? (
						<p className="mt-2 text-muted-foreground text-sm">{lead}</p>
					) : null}
				</div>
				{actions ? (
					<div className="flex flex-wrap items-center gap-2">{actions}</div>
				) : null}
			</div>
			<div className="mt-6">{children}</div>
		</div>
	);
}

// StatusDot is a coloured dot before a status word, so the colour is never the only signal.
// It stays on one line for tables; `wrap` lets a sentence wrap with the dot on its first line.
export function StatusDot({
	tone,
	wrap = false,
	children,
}: {
	tone: "success" | "warning" | "destructive" | "muted";
	wrap?: boolean;
	children: React.ReactNode;
}) {
	return (
		<span
			className={cn(
				"inline-flex gap-1.5",
				wrap ? "items-start" : "items-center whitespace-nowrap",
			)}
		>
			<span
				aria-hidden
				className={cn(
					"size-2 shrink-0 rounded-full",
					wrap && "mt-1.5", // centred on a 20px text-sm line
					tone === "success" && "bg-success",
					tone === "warning" && "bg-warning",
					tone === "destructive" && "bg-destructive",
					tone === "muted" && "bg-border-strong",
				)}
			/>
			{children}
		</span>
	);
}
