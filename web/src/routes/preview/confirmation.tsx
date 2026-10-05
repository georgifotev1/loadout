import { createFileRoute, Link } from "@tanstack/react-router";
import { Check, PackageCheck, Truck } from "lucide-react";
import { Container } from "#/components/site/container";
import { Button } from "#/components/ui/button";
import { m } from "#/i18n";
import { cn } from "#/lib/utils";
import { CompactLines, deliveryCents, Totals } from "./-order";

export const Route = createFileRoute("/preview/confirmation")({
	component: ConfirmationPage,
});

const steps = [
	{
		icon: Check,
		title: m.confirmation.stepConfirmed,
		detail: m.confirmation.stepConfirmedDetail,
	},
	{
		icon: Truck,
		title: m.confirmation.stepShipped,
		detail: m.confirmation.stepShippedDetail,
	},
	{
		icon: PackageCheck,
		title: m.confirmation.stepDelivered,
		detail: m.confirmation.stepDeliveredDetail,
	},
];

function ConfirmationPage() {
	return (
		<Container className="max-w-3xl pt-16">
			<div className="flex size-12 items-center justify-center rounded-full bg-success/10 text-success">
				<Check className="size-6" strokeWidth={2.25} />
			</div>
			<p className="mt-6 font-mono text-muted-foreground text-sm">
				{m.confirmation.number("LO-260105-4821")}
			</p>
			<h1 className="mt-1 font-semibold text-3xl tracking-[-0.03em] sm:text-4xl">
				{m.confirmation.title}
			</h1>
			<p className="mt-3 max-w-xl text-muted-foreground">
				{m.confirmation.emailSent("ivan@example.bg")}
			</p>

			<section className="mt-10 rounded-xl border bg-card p-6">
				<h2 className="font-medium">{m.confirmation.next}</h2>
				<ol className="mt-5 grid gap-5 sm:grid-cols-3">
					{steps.map((s, i) => (
						<li key={s.title} className="flex gap-3 sm:flex-col">
							<span
								className={cn(
									"flex size-8 shrink-0 items-center justify-center rounded-full border",
									i === 0
										? "border-foreground bg-primary text-primary-foreground"
										: "text-muted-foreground",
								)}
							>
								<s.icon className="size-4" />
							</span>
							<div>
								<p className="font-medium text-sm">{s.title}</p>
								<p className="mt-0.5 text-muted-foreground text-meta">
									{s.detail}
								</p>
							</div>
						</li>
					))}
				</ol>
			</section>

			<section className="mt-4 rounded-xl border bg-card p-6">
				<CompactLines />
				<Totals delivery={deliveryCents} className="mt-6 border-t pt-5" />
			</section>

			<Button variant="outline" className="mt-8" render={<Link to="/" />}>
				{m.confirmation.backToShop}
			</Button>
		</Container>
	);
}
