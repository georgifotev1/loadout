import { cn } from "#/lib/utils";

// ProductImage shows a supplier photo on white. The photos have white backgrounds, so the
// tile is white too and the photo blends into it; padding keeps products off the edges.
export function ProductImage({
	src,
	alt,
	className,
	loading = "lazy",
}: {
	src?: string;
	alt: string;
	className?: string;
	loading?: "lazy" | "eager";
}) {
	return (
		<div className={cn("relative aspect-square bg-white p-[12%]", className)}>
			{src ? (
				// Absolutely positioned (inheriting the padding) so the photo's own size never
				// changes the tile's height.
				<div className="absolute inset-0 p-[inherit]">
					<img
						src={src}
						alt={alt}
						loading={loading}
						referrerPolicy="no-referrer"
						className="size-full object-contain mix-blend-multiply"
					/>
				</div>
			) : null}
		</div>
	);
}
