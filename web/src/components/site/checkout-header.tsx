import { Link } from "@tanstack/react-router";
import { ChevronLeft, Lock } from "lucide-react";
import { m } from "#/i18n";
import { Container } from "./container";
import { Logo } from "./logo";

// CheckoutHeader replaces the site header during checkout: no search or category links to
// pull shoppers away, only the way back to the cart and a reassurance that it's secure.
export function CheckoutHeader() {
	return (
		<header className="border-b bg-card">
			<Container className="relative flex h-header items-center justify-between">
				<Link
					to="/cart"
					className="flex items-center gap-1 text-muted-foreground text-sm transition-colors hover:text-foreground"
				>
					<ChevronLeft className="size-4" />
					<span className="hidden sm:inline">{m.checkout.backToCart}</span>
				</Link>
				<Link to="/" className="-translate-x-1/2 absolute left-1/2">
					<Logo />
				</Link>
				<span className="flex items-center gap-1.5 text-muted-foreground text-xs">
					<Lock className="size-3.5" />
					<span className="hidden sm:inline">{m.checkout.secure}</span>
				</span>
			</Container>
		</header>
	);
}
