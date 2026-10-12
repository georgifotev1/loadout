import { Link } from "@tanstack/react-router";
import { type LegalPageKey, legalPaths } from "#/components/legal/legal-page";
import { m } from "#/i18n";
import type { Category } from "#/lib/categories";
import { Container } from "./container";
import { Logo } from "./logo";

const shopLinks: Category[] = [
	"cpu",
	"gpu",
	"motherboard",
	"peripheral.monitor",
];

const columns: { title: string; pages: LegalPageKey[] }[] = [
	{
		title: m.footer.help,
		pages: ["delivery", "returns", "warranty", "contacts"],
	},
	{ title: m.footer.company, pages: ["terms", "privacy", "cookies"] },
];

export function SiteFooter() {
	return (
		<footer className="mt-24 border-t bg-card">
			<Container className="grid gap-10 py-12 sm:grid-cols-2 lg:grid-cols-5">
				<div className="lg:col-span-2">
					<Logo />
					<p className="mt-3 max-w-xs text-muted-foreground text-sm">
						{m.site.tagline}
					</p>
				</div>
				<div>
					<h2 className="font-medium text-sm">{m.footer.shop}</h2>
					<ul className="mt-3 space-y-2 text-muted-foreground text-sm">
						{shopLinks.map((c) => (
							<li key={c}>
								<Link
									to="/c/$category"
									params={{ category: c }}
									className="transition-colors hover:text-foreground"
								>
									{m.categories[c]}
								</Link>
							</li>
						))}
					</ul>
				</div>
				{columns.map((col) => (
					<div key={col.title}>
						<h2 className="font-medium text-sm">{col.title}</h2>
						<ul className="mt-3 space-y-2 text-muted-foreground text-sm">
							{col.pages.map((key) => (
								<li key={key}>
									<Link
										to={legalPaths[key]}
										className="transition-colors hover:text-foreground"
									>
										{m.legal.pages[key]}
									</Link>
								</li>
							))}
						</ul>
					</div>
				))}
			</Container>
			<Container className="flex flex-col gap-2 border-t py-6 text-subtle-foreground text-xs sm:flex-row sm:justify-between">
				<p>{m.footer.rights(new Date().getFullYear())}</p>
				<p>
					Visa · Mastercard · Apple Pay · Google Pay ·{" "}
					{m.checkout.cashOnDelivery}
				</p>
			</Container>
		</footer>
	);
}
