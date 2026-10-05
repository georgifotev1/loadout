import { Checkbox as CheckboxPrimitive } from "@base-ui/react/checkbox";
import { CheckIcon } from "lucide-react";
import { cn } from "#/lib/utils";

// Checkbox renders a native <button>, so a sibling <Label htmlFor> works.
function Checkbox({
	className,
	...props
}: Omit<CheckboxPrimitive.Root.Props, "render" | "nativeButton">) {
	return (
		<CheckboxPrimitive.Root
			nativeButton
			render={<button type="button" />}
			className={cn(
				"peer flex size-4 shrink-0 items-center justify-center rounded-[4px] border border-input bg-card shadow-xs outline-none transition-shadow focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50 disabled:cursor-not-allowed disabled:opacity-50 aria-invalid:border-destructive data-checked:border-primary data-checked:bg-primary data-checked:text-primary-foreground",
				className,
			)}
			{...props}
		>
			<CheckboxPrimitive.Indicator className="flex data-unchecked:hidden">
				<CheckIcon className="size-3.5" />
			</CheckboxPrimitive.Indicator>
		</CheckboxPrimitive.Root>
	);
}

export { Checkbox };
