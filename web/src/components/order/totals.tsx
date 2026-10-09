import { Price } from "#/components/product/price";
import { m } from "#/i18n";
import { cn } from "#/lib/utils";

// Totals shows an order's sums as the API computed them.
export function Totals({
	subtotal,
	delivery,
	assembly,
	total,
	className,
}: {
	subtotal: number;
	delivery: number;
	/** The assembly fee; no row when the PC isn't assembled. */
	assembly?: number;
	total: number;
	className?: string;
}) {
	return (
		<dl className={cn("flex flex-col gap-2.5 text-sm", className)}>
			<div className="flex justify-between">
				<dt className="text-muted-foreground">{m.cart.subtotal}</dt>
				<dd>
					<Price cents={subtotal} className="font-medium" />
				</dd>
			</div>
			<div className="flex justify-between">
				<dt className="text-muted-foreground">{m.cart.delivery}</dt>
				<dd>
					<Price cents={delivery} className="font-medium" />
				</dd>
			</div>
			{assembly === undefined ? null : (
				<div className="flex justify-between">
					<dt className="text-muted-foreground">{m.cart.assembly}</dt>
					<dd>
						<Price cents={assembly} className="font-medium" />
					</dd>
				</div>
			)}
			<div className="mt-2 flex items-baseline justify-between border-t pt-4">
				<dt className="font-medium">{m.cart.total}</dt>
				<dd>
					<Price cents={total} className="text-2xl tracking-[-0.02em]" />
				</dd>
			</div>
			<p className="text-right text-subtle-foreground text-xs">
				{m.cart.vatIncluded}
			</p>
		</dl>
	);
}
