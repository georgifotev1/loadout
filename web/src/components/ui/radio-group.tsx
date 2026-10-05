import { Radio } from "@base-ui/react/radio";
import { RadioGroup as RadioGroupPrimitive } from "@base-ui/react/radio-group";
import { cn } from "#/lib/utils";

function RadioGroup({ className, ...props }: RadioGroupPrimitive.Props) {
	return (
		<RadioGroupPrimitive className={cn("grid gap-3", className)} {...props} />
	);
}

// RadioGroupItem renders a native <button>, so a sibling <Label htmlFor> works.
function RadioGroupItem({
	className,
	...props
}: Omit<Radio.Root.Props, "render" | "nativeButton">) {
	return (
		<Radio.Root
			nativeButton
			render={<button type="button" />}
			className={cn(
				"flex aspect-square size-4 shrink-0 items-center justify-center rounded-full border border-input bg-card shadow-xs outline-none transition-[color,box-shadow] focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50 disabled:cursor-not-allowed disabled:opacity-50 data-checked:border-primary",
				className,
			)}
			{...props}
		>
			<Radio.Indicator className="size-2 rounded-full bg-primary data-unchecked:hidden" />
		</Radio.Root>
	);
}

export { RadioGroup, RadioGroupItem };
