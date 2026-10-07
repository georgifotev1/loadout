import { Price } from "#/components/product/price";
import { ProductImage } from "#/components/product/product-image";

export type CompactLine = {
	key: string;
	name: string;
	image?: string;
	quantity: number;
	cents?: number; // absent for a line no longer for sale
};

// CompactLines lists products small, with the quantity on the picture: the checkout summary and
// the order page.
export function CompactLines({ lines }: { lines: CompactLine[] }) {
	return (
		<ul className="flex flex-col gap-4">
			{lines.map((l) => (
				<li key={l.key} className="flex items-center gap-3">
					<div className="relative shrink-0">
						<ProductImage
							src={l.image}
							alt={l.name}
							className="size-14 rounded-md border p-1.5"
						/>
						<span className="-top-1.5 -right-1.5 absolute flex size-5 items-center justify-center rounded-full bg-primary font-medium text-[11px] text-primary-foreground tabular-nums">
							{l.quantity}
						</span>
					</div>
					<p className="line-clamp-2 flex-1 text-meta">{l.name}</p>
					{l.cents === undefined ? null : (
						<Price cents={l.cents} className="font-medium text-sm" />
					)}
				</li>
			))}
		</ul>
	);
}
