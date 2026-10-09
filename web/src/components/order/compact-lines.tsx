import { Price } from "#/components/product/price";
import { ProductImage } from "#/components/product/product-image";
import { PcGroup } from "./pc-group";

export type CompactLine = {
	key: string;
	name: string;
	image?: string;
	quantity: number;
	cents?: number; // absent for a line no longer for sale
	inBuild?: boolean; // a part of the PC we assemble
};

// CompactLines lists products small, with the quantity on the picture: the checkout summary and
// the order page. With pcTotalCents (assembly), the lines inBuild show as one PC on top.
export function CompactLines({
	lines,
	pcTotalCents,
}: {
	lines: CompactLine[];
	pcTotalCents?: number;
}) {
	if (pcTotalCents === undefined) return <List lines={lines} />;
	const pc = lines.filter((l) => l.inBuild);
	const rest = lines.filter((l) => !l.inBuild);
	return (
		<div className="flex flex-col gap-4">
			<PcGroup
				totalCents={pcTotalCents}
				parts={pc.reduce((n, l) => n + l.quantity, 0)}
				images={pc.map((l) => l.image)}
			>
				<div className="p-4">
					<List lines={pc} />
				</div>
			</PcGroup>
			{rest.length ? <List lines={rest} /> : null}
		</div>
	);
}

function List({ lines }: { lines: CompactLine[] }) {
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
