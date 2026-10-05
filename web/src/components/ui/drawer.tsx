import { Drawer as DrawerPrimitive } from "@base-ui/react/drawer";
import { cn } from "#/lib/utils";

// Drawer is a swipe-to-dismiss panel: a sidebar from the left or right, or a bottom sheet.
// Built on Base UI (vaul, which shadcn's drawer wraps, is unmaintained).

export type DrawerSide = "left" | "right" | "bottom";

const swipeDirection = {
	left: "left",
	right: "right",
	bottom: "down",
} as const;

// The iOS sheet curve: fast start, long soft landing.
const motion =
	"transition-transform duration-[450ms] ease-[cubic-bezier(0.32,0.72,0,1)] data-swiping:select-none data-ending-style:duration-[calc(var(--drawer-swipe-strength)*400ms)] motion-reduce:duration-0";

const popupBySide: Record<DrawerSide, string> = {
	right:
		"h-full w-[26rem] max-w-[calc(100vw-2rem)] border-l [transform:translateX(var(--drawer-swipe-movement-x))] data-starting-style:[transform:translateX(100%)] data-ending-style:[transform:translateX(100%)]",
	left: "h-full w-80 max-w-[calc(100vw-3rem)] border-r [transform:translateX(var(--drawer-swipe-movement-x))] data-starting-style:[transform:translateX(-100%)] data-ending-style:[transform:translateX(-100%)]",
	// The 3rem bleed below the screen keeps the sheet's bottom covered when it overshoots.
	bottom:
		"-mb-12 max-h-[calc(88dvh+3rem)] w-full rounded-t-2xl border-t pb-[calc(env(safe-area-inset-bottom,0px)+3rem)] [transform:translateY(var(--drawer-swipe-movement-y))] data-starting-style:[transform:translateY(calc(100%-3rem+2px))] data-ending-style:[transform:translateY(calc(100%-3rem+2px))]",
};

const viewportBySide: Record<DrawerSide, string> = {
	right: "justify-end items-stretch",
	left: "justify-start items-stretch",
	bottom: "items-end justify-center",
};

export function Drawer({
	side,
	open,
	onOpenChange,
	children,
}: {
	side: DrawerSide;
	open: boolean;
	onOpenChange: (open: boolean) => void;
	children: React.ReactNode;
}) {
	return (
		<DrawerPrimitive.Root
			open={open}
			onOpenChange={onOpenChange}
			swipeDirection={swipeDirection[side]}
		>
			{children}
		</DrawerPrimitive.Root>
	);
}

export function DrawerPopup({
	side,
	className,
	children,
}: {
	side: DrawerSide;
	className?: string;
	children: React.ReactNode;
}) {
	return (
		<DrawerPrimitive.Portal>
			<DrawerPrimitive.Backdrop className="fixed inset-0 z-50 min-h-dvh bg-black opacity-[calc(0.32*(1-var(--drawer-swipe-progress)))] transition-opacity duration-[450ms] ease-[cubic-bezier(0.32,0.72,0,1)] data-ending-style:opacity-0 data-starting-style:opacity-0 data-swiping:duration-0 data-ending-style:duration-[calc(var(--drawer-swipe-strength)*400ms)]" />
			<DrawerPrimitive.Viewport
				className={cn("fixed inset-0 z-50 flex", viewportBySide[side])}
			>
				<DrawerPrimitive.Popup
					className={cn(
						"flex flex-col overflow-hidden bg-card text-card-foreground shadow-popover outline-none",
						motion,
						popupBySide[side],
						className,
					)}
				>
					{side === "bottom" ? (
						<div
							aria-hidden
							className="mx-auto mt-2.5 mb-1 h-1 w-10 shrink-0 rounded-full bg-border-strong"
						/>
					) : null}
					{children}
				</DrawerPrimitive.Popup>
			</DrawerPrimitive.Viewport>
		</DrawerPrimitive.Portal>
	);
}

export const DrawerContent = DrawerPrimitive.Content;
export const DrawerTitle = DrawerPrimitive.Title;
export const DrawerDescription = DrawerPrimitive.Description;
export const DrawerClose = DrawerPrimitive.Close;
