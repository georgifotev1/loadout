import { ChevronDown, Cpu } from "lucide-react";
import { useState } from "react";
import { Price } from "#/components/product/price";
import { ProductImage } from "#/components/product/product-image";
import { m } from "#/i18n";
import { cn } from "#/lib/utils";

const t = m.assembly;

// PcGroup shows the PC we assemble as one item: its price with the fee, and its parts folded
// away under it. The parts stay separate lines underneath (each has its own warranty, stock and
// price); this only groups them on screen. action is an extra control in the header, e.g. a link
// back to the builder.
export function PcGroup({
	totalCents,
	parts,
	images = [],
	action,
	className,
	children,
}: {
	totalCents: number;
	parts: number; // units, for "8 части"
	images?: (string | undefined)[]; // the parts' pictures, shown small while folded
	action?: React.ReactNode;
	className?: string;
	children: React.ReactNode;
}) {
	const [open, setOpen] = useState(false);
	return (
		<div className={cn("rounded-xl border bg-card", className)}>
			<div className="flex items-start gap-3 p-4">
				<span className="flex size-10 shrink-0 items-center justify-center rounded-lg border bg-muted/40">
					<Cpu className="size-5 text-muted-foreground" />
				</span>
				<div className="min-w-0 flex-1">
					<p className="font-medium text-sm">{t.pc}</p>
					<p className="mt-0.5 text-muted-foreground text-xs">
						{t.pcDetail(parts)}
					</p>
					{!open && images.length ? (
						<div className="mt-2 flex flex-wrap gap-1.5" aria-hidden>
							{images.map((src, i) => (
								<ProductImage
									// biome-ignore lint/suspicious/noArrayIndexKey: a fixed list of pictures
									key={i}
									src={src}
									alt=""
									className="size-9 rounded-md border p-0.5"
								/>
							))}
						</div>
					) : null}
					<div className="mt-1.5 flex flex-wrap items-center gap-x-3 gap-y-1">
						<button
							type="button"
							aria-expanded={open}
							onClick={() => setOpen(!open)}
							className="-ml-0.5 flex items-center gap-1 rounded px-0.5 text-xs underline-offset-4 hover:underline"
						>
							{open ? t.hideParts : t.showParts}
							<ChevronDown
								className={cn(
									"size-3.5 transition-transform",
									open && "rotate-180",
								)}
							/>
						</button>
						{action}
					</div>
				</div>
				<Price cents={totalCents} className="shrink-0 font-medium text-sm" />
			</div>
			{open ? <div className="border-t">{children}</div> : null}
		</div>
	);
}
