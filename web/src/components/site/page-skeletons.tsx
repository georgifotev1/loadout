import { Skeleton } from "#/components/ui/skeleton";
import { m } from "#/i18n";
import { Container } from "./container";

// Pending components: the router shows one in place of a page whose data is taking a while
// (defaultPendingMs), until the page renders. Each keeps the real page's layout, so nothing
// jumps when the content arrives. Only for a page that replaces another: a page that reloads
// for new search params (filters, paging, builder picks) stays and dims (useRefreshing), as a
// pending component would unmount it and lose its state.

export function ProductSkeleton() {
	return (
		<Container className="pt-6">
			<Loading />
			<Skeleton className="h-4 w-48" />
			<div className="mt-6 grid gap-8 lg:grid-cols-[1.15fr_1fr] lg:gap-12">
				<Skeleton className="aspect-square rounded-xl" />
				<div>
					<Skeleton className="h-3 w-20" />
					<Skeleton className="mt-3 h-8 w-full" />
					<Skeleton className="mt-2 h-8 w-2/3" />
					<Skeleton className="mt-4 h-4 w-48" />
					<Skeleton className="mt-8 h-10 w-40" />
					<Skeleton className="mt-6 h-11 w-full sm:w-56" />
					<Skeleton className="mt-6 h-36 w-full rounded-lg" />
				</div>
			</div>
		</Container>
	);
}

// PageSkeleton is for the pages without one of their own: a title and a block of content.
export function PageSkeleton() {
	return (
		<Container className="pt-6">
			<Loading />
			<Skeleton className="h-4 w-48" />
			<Skeleton className="mt-6 h-9 w-72 max-w-full" />
			<Skeleton className="mt-3 h-4 w-48" />
			<Skeleton className="mt-8 h-96 w-full rounded-xl" />
		</Container>
	);
}

// Loading tells screen readers what the grey blocks are.
export function Loading() {
	return <output className="sr-only">{m.ui.loading}</output>;
}
