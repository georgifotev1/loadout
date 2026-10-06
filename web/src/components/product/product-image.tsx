import { ImageOff } from "lucide-react";
import { useState } from "react";
import { cn } from "#/lib/utils";

// ProductImage shows a supplier photo on white. The photos have white backgrounds, so the
// tile is white too and the photo blends into it; padding keeps products off the edges.
export function ProductImage({
	src,
	alt,
	className,
	loading = "lazy",
	fetchPriority,
}: {
	src?: string;
	alt: string;
	className?: string;
	loading?: "lazy" | "eager";
	// "high" for the page's main photo, so the browser fetches it before the rest.
	fetchPriority?: "high" | "low" | "auto";
}) {
	return (
		<div className={cn("relative aspect-square bg-white p-[12%]", className)}>
			{src ? (
				// Keyed so a new photo (the product page's gallery) starts hidden again.
				<Photo
					key={src}
					src={src}
					alt={alt}
					loading={loading}
					fetchPriority={fetchPriority}
				/>
			) : (
				<Missing />
			)}
		</div>
	);
}

// Photo stays invisible until it has fully loaded and fades in, instead of painting top to
// bottom as the bytes arrive: the supplier serves full-size, non-progressive JPEGs.
function Photo({
	src,
	alt,
	loading,
	fetchPriority,
}: {
	src: string;
	alt: string;
	loading: "lazy" | "eager";
	fetchPriority?: "high" | "low" | "auto";
}) {
	const [state, setState] = useState<"loading" | "loaded" | "failed">(
		"loading",
	);
	if (state === "failed") return <Missing />;
	return (
		// Absolutely positioned (inheriting the padding) so the photo's own size never
		// changes the tile's height.
		<div className="absolute inset-0 p-[inherit]">
			<img
				// A photo the browser had cached may finish before hydration, when onLoad isn't
				// listening yet. (`complete` alone isn't enough: a lazy image not yet fetched can
				// report it too.)
				ref={(img) => {
					if (img?.complete && img.naturalWidth) setState("loaded");
				}}
				src={src}
				alt={alt}
				loading={loading}
				decoding="async"
				fetchPriority={fetchPriority}
				referrerPolicy="no-referrer"
				onLoad={() => setState("loaded")}
				onError={() => setState("failed")}
				className={cn(
					"size-full object-contain mix-blend-multiply transition-opacity duration-300 ease-out motion-reduce:transition-none",
					state === "loaded" ? "opacity-100" : "opacity-0",
				)}
			/>
		</div>
	);
}

// Missing stands in for a product without a photo, or one that didn't load.
function Missing() {
	return (
		<div className="absolute inset-0 flex items-center justify-center text-border-strong">
			<ImageOff className="size-1/5 max-h-10 max-w-10" strokeWidth={1.25} />
		</div>
	);
}
