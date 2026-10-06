import { useQueryClient } from "@tanstack/react-query";
import { Link, useNavigate } from "@tanstack/react-router";
import { LogOut } from "lucide-react";
import { Container } from "#/components/site/container";
import { Logo } from "#/components/site/logo";
import { Badge } from "#/components/ui/badge";
import { Button } from "#/components/ui/button";
import { m } from "#/i18n";
import { logout, type Me } from "#/lib/api/auth";

const navLink =
	"rounded-md px-2.5 py-1.5 text-muted-foreground transition-colors hover:text-foreground data-[status=active]:bg-muted data-[status=active]:font-medium data-[status=active]:text-foreground";

// AdminLayout is the back office chrome: its own header with the admin's sections, the signed-in
// email and log out. No shop search, categories or footer.
export function AdminLayout({
	me,
	children,
}: {
	me: Me;
	children: React.ReactNode;
}) {
	const queryClient = useQueryClient();
	const navigate = useNavigate();

	return (
		<div className="flex min-h-dvh flex-col">
			<header className="sticky top-0 z-40 border-b bg-card">
				<Container className="flex h-header items-center gap-6">
					<Link to="/admin" className="flex shrink-0 items-center gap-2">
						<Logo />
						<Badge variant="secondary">{m.admin.badge}</Badge>
					</Link>
					<nav className="flex items-center gap-1 text-sm">
						<Link
							to="/admin"
							activeOptions={{ exact: true }}
							className={navLink}
						>
							{m.admin.nav.dashboard}
						</Link>
						<Link to="/admin/specs" className={navLink}>
							{m.admin.nav.specs}
						</Link>
					</nav>
					<div className="ml-auto flex items-center gap-2">
						<span className="hidden text-muted-foreground text-sm sm:inline">
							{me.email}
						</span>
						<Button
							variant="ghost"
							size="sm"
							onClick={() =>
								logout(queryClient).then(
									() => navigate({ to: "/login", replace: true }),
									() => {},
								)
							}
						>
							<LogOut />
							{m.admin.logout}
						</Button>
					</div>
				</Container>
			</header>
			<main className="flex-1">
				<Container className="py-8">{children}</Container>
			</main>
		</div>
	);
}
