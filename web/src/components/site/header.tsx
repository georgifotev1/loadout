import { Link } from "@tanstack/react-router";
import {
	ChevronRight,
	Menu,
	ShoppingCart,
	User,
	Wrench,
	X,
} from "lucide-react";
import { useState } from "react";
import { Button } from "#/components/ui/button";
import {
	Drawer,
	DrawerClose,
	DrawerContent,
	DrawerPopup,
	DrawerTitle,
} from "#/components/ui/drawer";
import { m } from "#/i18n";
import {
	type Category,
	componentCategories,
	peripheralCategories,
} from "#/lib/categories";
import { cn } from "#/lib/utils";
import { Container } from "./container";
import { Logo } from "./logo";

// SiteHeader is the storefront header. The account and cart buttons only show with
// onCartClick: the real cart arrives with checkout.
export function SiteHeader({
	search,
	cartCount = 0,
	onCartClick,
}: {
	search: React.ReactNode;
	cartCount?: number;
	onCartClick?: () => void;
}) {
	const [menuOpen, setMenuOpen] = useState(false);

	return (
		<header className="sticky top-0 z-40 border-b bg-card/85 backdrop-blur-md backdrop-saturate-150">
			<Container className="flex h-header items-center gap-2 md:gap-8">
				<Button
					variant="ghost"
					size="icon"
					className="-ml-2 md:hidden"
					aria-label={m.nav.menu}
					onClick={() => setMenuOpen(true)}
				>
					<Menu />
				</Button>
				<Link to="/" className="shrink-0">
					<Logo />
				</Link>

				<div className="hidden max-w-2xl flex-1 md:block">{search}</div>

				{onCartClick ? (
					<div className="ml-auto flex items-center gap-0.5">
						<Button variant="ghost" size="icon" aria-label={m.nav.account}>
							<User />
						</Button>
						<Button
							variant="ghost"
							className="-mr-2 gap-3 px-2.5 md:mr-0"
							onClick={onCartClick}
							aria-label={`${m.nav.cart}, ${m.cart.items(cartCount)}`}
						>
							<span className="relative">
								<ShoppingCart />
								{cartCount > 0 ? (
									<span className="-top-2 -right-2.5 absolute flex h-4 min-w-4 items-center justify-center rounded-full bg-brand px-1 font-semibold text-[10px] text-brand-foreground tabular-nums ring-2 ring-card">
										{cartCount}
									</span>
								) : null}
							</span>
							<span className="hidden lg:inline">{m.nav.cart}</span>
						</Button>
					</div>
				) : null}
			</Container>

			<Container className="pb-3 md:hidden">{search}</Container>

			<nav aria-label={m.nav.allCategories} className="hidden md:block">
				<Container>
					<ul className="-mb-px flex h-10 items-center gap-5 overflow-x-auto text-meta text-muted-foreground scrollbar-none">
						<li className="shrink-0">
							<Link
								to="/builder"
								className="flex h-10 items-center gap-1.5 border-transparent border-b-2 font-medium text-foreground transition-colors hover:text-brand-ink data-[status=active]:border-brand"
							>
								<Wrench className="size-3.5" /> {m.nav.builder}
							</Link>
						</li>
						<li aria-hidden className="h-4 w-px shrink-0 bg-border" />
						{componentCategories.map((c) => (
							<NavLink key={c} category={c} />
						))}
						<li aria-hidden className="h-4 w-px shrink-0 bg-border" />
						{peripheralCategories.slice(0, 4).map((c) => (
							<NavLink key={c} category={c} />
						))}
					</ul>
				</Container>
			</nav>

			<MobileMenu open={menuOpen} onOpenChange={setMenuOpen} />
		</header>
	);
}

function NavLink({ category }: { category: Category }) {
	return (
		<li className="shrink-0">
			<Link
				to="/c/$category"
				params={{ category }}
				className="flex h-10 items-center border-transparent border-b-2 transition-colors hover:text-foreground data-[status=active]:border-brand data-[status=active]:text-foreground"
			>
				{m.categories[category]}
			</Link>
		</li>
	);
}

function MobileMenu({
	open,
	onOpenChange,
}: {
	open: boolean;
	onOpenChange: (open: boolean) => void;
}) {
	const groups = [
		{ title: m.nav.components, items: componentCategories },
		{ title: m.nav.peripherals, items: peripheralCategories },
	];
	return (
		<Drawer side="left" open={open} onOpenChange={onOpenChange}>
			<DrawerPopup side="left">
				<DrawerContent className="flex min-h-0 flex-1 flex-col">
					<div className="flex h-header shrink-0 items-center justify-between border-b px-4">
						<DrawerTitle>
							<Logo />
						</DrawerTitle>
						<DrawerClose
							render={
								<Button
									variant="ghost"
									size="icon-sm"
									aria-label={m.nav.close}
								/>
							}
						>
							<X />
						</DrawerClose>
					</div>
					<div className="min-h-0 flex-1 overflow-y-auto overscroll-contain px-2 py-3">
						<Link
							to="/builder"
							onClick={() => onOpenChange(false)}
							className="mb-3 flex items-center justify-between rounded-lg bg-muted px-3 py-2.5 font-medium text-sm"
						>
							<span className="flex items-center gap-2">
								<Wrench className="size-4" /> {m.nav.builder}
							</span>
							<ChevronRight className="size-4 text-subtle-foreground" />
						</Link>
						{groups.map((g, i) => (
							<div key={g.title} className={cn(i > 0 && "mt-4")}>
								<p className="px-3 pb-1.5 font-medium text-[11px] text-subtle-foreground uppercase tracking-wider">
									{g.title}
								</p>
								<ul>
									{g.items.map((c) => (
										<li key={c}>
											<Link
												to="/c/$category"
												params={{ category: c }}
												onClick={() => onOpenChange(false)}
												className="flex items-center justify-between rounded-lg px-3 py-2.5 text-sm hover:bg-muted"
											>
												{m.categories[c]}
												<ChevronRight className="size-4 text-subtle-foreground" />
											</Link>
										</li>
									))}
								</ul>
							</div>
						))}
					</div>
				</DrawerContent>
			</DrawerPopup>
		</Drawer>
	);
}
