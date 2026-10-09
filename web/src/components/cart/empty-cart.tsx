import { Trash2 } from "lucide-react";
import { useState } from "react";
import {
	AlertDialog,
	AlertDialogAction,
	AlertDialogCancel,
	AlertDialogContent,
	AlertDialogDescription,
	AlertDialogFooter,
	AlertDialogHeader,
	AlertDialogTitle,
	AlertDialogTrigger,
} from "#/components/ui/alert-dialog";
import { Button } from "#/components/ui/button";
import { m } from "#/i18n";
import { cn } from "#/lib/utils";
import { useCart } from "./cart-provider";

// EmptyCart takes every product out of the cart, once the shopper confirms. On the cart page and
// in the drawer.
export function EmptyCart({ className }: { className?: string }) {
	const { empty, emptying } = useCart();
	const [confirming, setConfirming] = useState(false);
	return (
		<AlertDialog open={confirming} onOpenChange={setConfirming}>
			<AlertDialogTrigger
				render={
					<Button
						variant="ghost"
						size="sm"
						className={cn("text-muted-foreground", className)}
					/>
				}
			>
				<Trash2 /> {m.cart.emptyCart}
			</AlertDialogTrigger>
			<AlertDialogContent>
				<AlertDialogHeader>
					<AlertDialogTitle>{m.cart.emptyTitle}</AlertDialogTitle>
					<AlertDialogDescription>{m.cart.emptyBody}</AlertDialogDescription>
				</AlertDialogHeader>
				<AlertDialogFooter>
					<AlertDialogCancel>{m.cart.cancel}</AlertDialogCancel>
					<AlertDialogAction
						variant="destructive"
						disabled={emptying}
						onClick={() =>
							void empty()
								.catch(() => {})
								.finally(() => setConfirming(false))
						}
					>
						{m.cart.emptyCart}
					</AlertDialogAction>
				</AlertDialogFooter>
			</AlertDialogContent>
		</AlertDialog>
	);
}
