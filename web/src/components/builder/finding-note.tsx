import { CircleAlert, Info, TriangleAlert } from "lucide-react";
import { type Finding, findingText, type Severity } from "#/lib/builder";
import { cn } from "#/lib/utils";

const icons: Record<Severity, typeof Info> = {
	error: CircleAlert,
	warning: TriangleAlert,
	info: Info,
};

export const severityText: Record<Severity, string> = {
	error: "text-destructive",
	warning: "text-warning",
	info: "text-subtle-foreground",
};

export function SeverityIcon({
	severity,
	className,
}: {
	severity: Severity;
	className?: string;
}) {
	const Icon = icons[severity];
	return (
		<Icon
			aria-hidden
			className={cn("size-4 shrink-0", severityText[severity], className)}
		/>
	);
}

// FindingNote is one finding as a line of text. Errors are red text; warnings and notes keep
// the text readable and only colour the icon (amber text on white is too faint).
export function FindingNote({
	finding,
	className,
}: {
	finding: Finding;
	className?: string;
}) {
	return (
		<p
			className={cn(
				"flex gap-2 text-meta",
				finding.severity === "error"
					? "text-destructive"
					: finding.severity === "warning"
						? "text-foreground"
						: "text-muted-foreground",
				className,
			)}
		>
			<SeverityIcon severity={finding.severity} className="mt-0.5" />
			<span>{findingText(finding)}</span>
		</p>
	);
}
