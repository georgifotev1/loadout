import { Link } from "@tanstack/react-router";
import {
	Breadcrumb,
	BreadcrumbItem,
	BreadcrumbLink,
	BreadcrumbList,
	BreadcrumbPage,
	BreadcrumbSeparator,
} from "#/components/ui/breadcrumb";
import { m } from "#/i18n";
import type { Category } from "#/lib/categories";

// PageBreadcrumb is Home › [category] › current page. The category links to its listing.
export function PageBreadcrumb({
	category,
	current,
}: {
	category?: Category;
	current: string;
}) {
	return (
		<Breadcrumb>
			<BreadcrumbList>
				<BreadcrumbItem>
					<BreadcrumbLink render={<Link to="/" />}>
						{m.product.home}
					</BreadcrumbLink>
				</BreadcrumbItem>
				<BreadcrumbSeparator />
				{category ? (
					<>
						<BreadcrumbItem>
							<BreadcrumbLink
								render={<Link to="/c/$category" params={{ category }} />}
							>
								{m.categories[category]}
							</BreadcrumbLink>
						</BreadcrumbItem>
						<BreadcrumbSeparator />
					</>
				) : null}
				<BreadcrumbItem className="min-w-0">
					<BreadcrumbPage className="truncate">{current}</BreadcrumbPage>
				</BreadcrumbItem>
			</BreadcrumbList>
		</Breadcrumb>
	);
}
