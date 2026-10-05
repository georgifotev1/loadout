import { Link } from "@tanstack/react-router";

const pages = [
	{ to: "/preview", label: "Tokens" },
	{ to: "/preview/home", label: "Home" },
	{ to: "/preview/category", label: "Category" },
	{ to: "/preview/product", label: "Product" },
	{ to: "/preview/cart", label: "Cart" },
	{ to: "/preview/checkout", label: "Checkout" },
	{ to: "/preview/confirmation", label: "Done" },
] as const;

// PreviewToolbar jumps between the preview pages. Developer tooling, so its labels are not
// translated.
export function PreviewToolbar() {
	return (
		<nav className="-translate-x-1/2 fixed bottom-4 left-1/2 z-50 flex max-w-[calc(100vw-2rem)] items-center gap-0.5 overflow-x-auto rounded-full bg-primary px-2 py-1.5 text-primary-foreground text-xs shadow-popover [scrollbar-width:none]">
			{pages.map((p) => (
				<Link
					key={p.to}
					to={p.to}
					activeOptions={{ exact: true }}
					className="shrink-0 rounded-full px-2.5 py-1 text-white/60 hover:text-white data-[status=active]:bg-white/10 data-[status=active]:text-white"
				>
					{p.label}
				</Link>
			))}
		</nav>
	);
}
