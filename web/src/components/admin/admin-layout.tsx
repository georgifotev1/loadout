import { useQuery, useQueryClient } from "@tanstack/react-query";
import {
	Link,
	type LinkProps,
	useMatchRoute,
	useNavigate,
} from "@tanstack/react-router";
import {
	ChevronsUpDown,
	House,
	ListChecks,
	LogOut,
	Package,
	Percent,
	RefreshCw,
	TriangleAlert,
	UserRound,
} from "lucide-react";
import { Logo } from "#/components/site/logo";
import { Badge } from "#/components/ui/badge";
import {
	DropdownMenu,
	DropdownMenuContent,
	DropdownMenuGroup,
	DropdownMenuItem,
	DropdownMenuLabel,
	DropdownMenuSeparator,
	DropdownMenuTrigger,
} from "#/components/ui/dropdown-menu";
import {
	Sidebar,
	SidebarContent,
	SidebarFooter,
	SidebarGroup,
	SidebarGroupLabel,
	SidebarHeader,
	SidebarInset,
	SidebarMenu,
	SidebarMenuBadge,
	SidebarMenuButton,
	SidebarMenuItem,
	SidebarProvider,
	SidebarRail,
	SidebarTrigger,
	useSidebar,
} from "#/components/ui/sidebar";
import { m } from "#/i18n";
import { overviewQuery } from "#/lib/api/admin";
import { logout, type Me } from "#/lib/api/auth";
import type { Schemas } from "#/lib/api/client";

type NavItem = {
	to: LinkProps["to"];
	label: string;
	icon: React.ComponentType;
	exact?: boolean;
	count?: (o: Schemas["AdminOverview"]) => number;
};

// The back office's sections. Orders get their own group in Phase 7.
const nav: { title?: string; items: NavItem[] }[] = [
	{
		items: [
			{ to: "/admin", label: m.admin.nav.dashboard, icon: House, exact: true },
		],
	},
	{
		title: m.admin.nav.catalog,
		items: [
			{ to: "/admin/products", label: m.admin.nav.products, icon: Package },
			{
				to: "/admin/markup-rules",
				label: m.admin.nav.markupRules,
				icon: Percent,
			},
			{ to: "/admin/specs", label: m.admin.nav.specs, icon: ListChecks },
		],
	},
	{
		title: m.admin.nav.system,
		items: [
			{ to: "/admin/sync", label: m.admin.nav.sync, icon: RefreshCw },
			{
				to: "/admin/jobs",
				label: m.admin.nav.jobs,
				icon: TriangleAlert,
				count: (o) => o.discarded_jobs,
			},
		],
	},
];

// AdminLayout is the back office chrome on shadcn's sidebar: it collapses to icons on desktop
// (the button in the top bar, the rail on its edge or Ctrl/⌘+B, remembered in a cookie) and
// is a sheet on phones. The account menu sits at the bottom.
export function AdminLayout({
	me,
	sidebarOpen,
	children,
}: {
	me: Me;
	sidebarOpen: boolean;
	children: React.ReactNode;
}) {
	return (
		<SidebarProvider defaultOpen={sidebarOpen}>
			<Sidebar collapsible="icon">
				<SidebarHeader className="h-header justify-center border-b">
					<Brand />
				</SidebarHeader>
				<SidebarContent>
					<AdminNav />
				</SidebarContent>
				<SidebarFooter className="border-t">
					<AccountMenu me={me} />
				</SidebarFooter>
				<SidebarRail />
			</Sidebar>
			<SidebarInset className="min-w-0">
				<header className="sticky top-0 z-10 flex h-header shrink-0 items-center gap-2 border-b bg-card px-3">
					<SidebarTrigger />
					{/* On phones the sidebar is a sheet, so the bar carries the brand. */}
					<span className="flex items-center gap-2 md:hidden">
						<Logo />
						<Badge variant="secondary">{m.admin.badge}</Badge>
					</span>
				</header>
				<div className="flex-1 px-4 py-6 sm:px-6 lg:px-10 lg:py-8">
					{children}
				</div>
			</SidebarInset>
		</SidebarProvider>
	);
}

// Brand is the sidebar's label, not a link: Начало is right below it.
function Brand() {
	return (
		<div className="flex h-8 items-center gap-2 px-2">
			{/* Collapsed, the sidebar is icon-wide: the wordmark shrinks to its dot. */}
			<span
				aria-hidden
				className="hidden size-4 shrink-0 items-center justify-center group-data-[collapsible=icon]:flex"
			>
				<span className="size-2 rounded-full bg-brand" />
			</span>
			<span className="flex items-center gap-2 group-data-[collapsible=icon]:hidden">
				<Logo />
				<Badge variant="secondary">{m.admin.badge}</Badge>
			</span>
		</div>
	);
}

function AdminNav() {
	// The counts are a hint: the nav renders without them while they load or if they fail.
	const { data: overview } = useQuery(overviewQuery);
	const matchRoute = useMatchRoute();
	const { setOpenMobile } = useSidebar();

	return nav.map((group) => (
		<SidebarGroup key={group.title ?? ""}>
			{group.title ? (
				<SidebarGroupLabel>{group.title}</SidebarGroupLabel>
			) : null}
			<SidebarMenu>
				{group.items.map((item) => {
					const count = overview && item.count?.(overview);
					return (
						<SidebarMenuItem key={item.label}>
							<SidebarMenuButton
								tooltip={item.label}
								isActive={!!matchRoute({ to: item.to, fuzzy: !item.exact })}
								render={
									<Link to={item.to} onClick={() => setOpenMobile(false)} />
								}
							>
								<item.icon />
								<span>{item.label}</span>
							</SidebarMenuButton>
							{count ? (
								<SidebarMenuBadge className="bg-destructive text-white peer-hover/menu-button:text-white peer-data-active/menu-button:text-white">
									{count}
								</SidebarMenuBadge>
							) : null}
						</SidebarMenuItem>
					);
				})}
			</SidebarMenu>
		</SidebarGroup>
	));
}

function AccountMenu({ me }: { me: Me }) {
	const queryClient = useQueryClient();
	const navigate = useNavigate();
	const { isMobile, setOpenMobile } = useSidebar();

	return (
		<SidebarMenu>
			<SidebarMenuItem>
				<DropdownMenu>
					<DropdownMenuTrigger
						render={
							<SidebarMenuButton
								size="lg"
								tooltip={me.email}
								className="data-popup-open:bg-sidebar-accent"
							/>
						}
					>
						<span className="flex size-8 shrink-0 items-center justify-center rounded-md bg-muted font-medium uppercase">
							{me.email.slice(0, 1)}
						</span>
						<span className="min-w-0 flex-1 truncate">{me.email}</span>
						<ChevronsUpDown className="ml-auto text-muted-foreground" />
					</DropdownMenuTrigger>
					<DropdownMenuContent
						side={isMobile ? "top" : "right"}
						align="end"
						sideOffset={8}
						className="w-56"
					>
						<DropdownMenuGroup>
							<DropdownMenuLabel className="truncate font-normal">
								{me.email}
							</DropdownMenuLabel>
						</DropdownMenuGroup>
						<DropdownMenuSeparator />
						<DropdownMenuItem
							render={
								<Link
									to="/admin/account"
									onClick={() => setOpenMobile(false)}
								/>
							}
						>
							<UserRound />
							{m.admin.nav.account}
						</DropdownMenuItem>
						<DropdownMenuSeparator />
						<DropdownMenuItem
							onClick={() =>
								logout(queryClient).then(
									() => navigate({ to: "/login", replace: true }),
									() => {},
								)
							}
						>
							<LogOut />
							{m.admin.logout}
						</DropdownMenuItem>
					</DropdownMenuContent>
				</DropdownMenu>
			</SidebarMenuItem>
		</SidebarMenu>
	);
}
