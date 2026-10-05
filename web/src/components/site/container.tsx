import { cn } from "#/lib/utils";

// Container centers page content at the page width with the responsive side gutter.
export function Container({
	className,
	...props
}: React.ComponentProps<"div">) {
	return (
		<div
			className={cn(
				"mx-auto w-full max-w-page px-4 sm:px-6 lg:px-8",
				className,
			)}
			{...props}
		/>
	);
}
