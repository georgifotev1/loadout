import { Link } from "@tanstack/react-router";
import { ChevronLeft, ChevronRight } from "lucide-react";
import { Button } from "#/components/ui/button";
import { m } from "#/i18n";
import { pageNumbers } from "#/lib/listing";
import { cn } from "#/lib/utils";

// Pagination links to the other pages of the current URL (its `page` search param, absent for
// page 1), so crawlers can follow them too.
export function Pagination({
	page,
	last,
	className,
}: {
	page: number;
	last: number;
	className?: string;
}) {
	if (last <= 1) return null;
	const to =
		(n: number) =>
		<T extends { page?: number }>(prev: T) => ({
			...prev,
			page: n === 1 ? undefined : n,
		});
	return (
		<nav
			aria-label={m.listing.pages}
			className={cn("mt-10 flex items-center justify-center gap-1", className)}
		>
			<Button
				variant="ghost"
				size="sm"
				disabled={page <= 1}
				render={page > 1 ? <Link to="." search={to(page - 1)} /> : undefined}
			>
				<ChevronLeft />
				<span className="hidden sm:inline">{m.listing.previous}</span>
			</Button>
			{pageNumbers(page, last).map((n, i) =>
				n === null ? (
					<span
						// biome-ignore lint/suspicious/noArrayIndexKey: a gap has no identity
						key={`gap-${i}`}
						className="px-1 text-subtle-foreground"
					>
						…
					</span>
				) : (
					<Button
						key={n}
						variant={n === page ? "outline" : "ghost"}
						size="icon-sm"
						className={cn("tabular-nums", n === page && "bg-card")}
						aria-label={m.listing.page(n)}
						aria-current={n === page ? "page" : undefined}
						render={<Link to="." search={to(n)} />}
					>
						{n}
					</Button>
				),
			)}
			<Button
				variant="ghost"
				size="sm"
				disabled={page >= last}
				render={page < last ? <Link to="." search={to(page + 1)} /> : undefined}
			>
				<span className="hidden sm:inline">{m.listing.next}</span>
				<ChevronRight />
			</Button>
		</nav>
	);
}
