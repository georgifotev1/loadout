import { Link } from "@tanstack/react-router";
import { FileWarning } from "lucide-react";
import { Container } from "#/components/site/container";
import { formatLongDate, m } from "#/i18n";
import { cn } from "#/lib/utils";

// legalPaths are the legal and help pages' paths, in the order of their menu.
export const legalPaths = {
	terms: "/terms",
	privacy: "/privacy",
	cookies: "/cookies",
	delivery: "/delivery",
	returns: "/returns",
	warranty: "/warranty",
	contacts: "/contacts",
} as const;

export type LegalPageKey = keyof typeof legalPaths;

const legalPages = Object.keys(legalPaths) as LegalPageKey[];

// legalHead is a legal page's <head>: its title and a description for search results.
export function legalHead(page: LegalPageKey, description: string) {
	return {
		meta: [
			{ title: m.site.title(m.legal.titles[page]) },
			{ name: "description", content: description },
		],
	};
}

// LegalPage is the frame of a legal or help page: the menu of the others, the title, the date it
// took effect and the text in long-form type.
export function LegalPage({
	page,
	effective,
	children,
}: {
	page: LegalPageKey;
	// The day the text took effect, "2026-10-12"; none for a page that isn't an agreement.
	effective?: string;
	children: React.ReactNode;
}) {
	return (
		<Container className="grid gap-10 pt-10 lg:grid-cols-[14rem_1fr]">
			<nav
				aria-label={m.legal.menu}
				className="lg:sticky lg:top-6 lg:self-start"
			>
				<h2 className="font-medium text-muted-foreground text-xs uppercase tracking-wide">
					{m.legal.menu}
				</h2>
				<ul className="mt-3 flex flex-wrap gap-x-4 gap-y-2 text-sm lg:flex-col">
					{legalPages.map((key) => (
						<li key={key}>
							<Link
								to={legalPaths[key]}
								className={cn(
									"text-muted-foreground transition-colors hover:text-foreground",
									key === page && "font-medium text-foreground",
								)}
							>
								{m.legal.pages[key]}
							</Link>
						</li>
					))}
				</ul>
			</nav>
			<article className="min-w-0">
				<h1 className="font-semibold text-3xl tracking-[-0.03em]">
					{m.legal.titles[page]}
				</h1>
				{effective ? (
					<p className="mt-2 text-muted-foreground text-sm">
						{m.legal.effective(formatLongDate(effective))}
					</p>
				) : null}
				{/* TODO(lawyer): drop the notice once the lawyer has approved the texts. */}
				<p className="mt-6 flex max-w-prose items-start gap-2 rounded-lg border border-warning/60 bg-warning/5 p-3 text-sm">
					<FileWarning className="mt-0.5 size-4 shrink-0 text-warning" />
					{m.legal.draft}
				</p>
				<div className="prose prose-sm sm:prose-base mt-8 max-w-prose prose-headings:font-semibold prose-headings:tracking-tight prose-a:underline-offset-4 prose-code:font-medium prose-code:before:content-none prose-code:after:content-none">
					{children}
				</div>
			</article>
		</Container>
	);
}
