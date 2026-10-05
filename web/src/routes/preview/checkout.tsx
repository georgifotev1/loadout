import { createFileRoute, Link } from "@tanstack/react-router";
import { Lock } from "lucide-react";
import { useState } from "react";
import { Price } from "#/components/product/price";
import { Container } from "#/components/site/container";
import { Button } from "#/components/ui/button";
import { Checkbox } from "#/components/ui/checkbox";
import { Input } from "#/components/ui/input";
import { Label } from "#/components/ui/label";
import { RadioGroup, RadioGroupItem } from "#/components/ui/radio-group";
import {
	Select,
	SelectContent,
	SelectItem,
	SelectTrigger,
	SelectValue,
} from "#/components/ui/select";
import { m } from "#/i18n";
import { cn } from "#/lib/utils";
import { CompactLines, deliveryCents, Totals } from "./-order";

export const Route = createFileRoute("/preview/checkout")({
	component: CheckoutPage,
});

const deliveryOptions = [
	{
		value: "address",
		title: m.checkout.toAddress,
		detail: m.checkout.toAddressDetail,
		cents: 599,
		days: 2,
	},
	{
		value: "econt",
		title: m.checkout.econtOffice,
		detail: m.checkout.chooseOffice,
		cents: 399,
		days: 2,
	},
	{
		value: "speedy",
		title: m.checkout.speedyOffice,
		detail: m.checkout.chooseOffice,
		cents: 449,
		days: 1,
	},
];

const paymentOptions = [
	{
		value: "cod",
		title: m.checkout.cashOnDelivery,
		detail: m.checkout.cashOnDeliveryDetail,
	},
	{ value: "card", title: m.checkout.card, detail: m.checkout.cardDetail },
];

const offices = [
	{ value: "sofia-center", label: "София, Център" },
	{ value: "sofia-mladost-1", label: "София, Младост 1" },
	{ value: "sofia-lyulin-5", label: "София, Люлин 5" },
];

function CheckoutPage() {
	const [delivery, setDelivery] = useState("address");
	const [payment, setPayment] = useState("cod");

	return (
		<Container className="pt-10">
			<h1 className="font-semibold text-3xl tracking-[-0.03em]">
				{m.checkout.title}
			</h1>

			<div className="mt-8 grid items-start gap-8 lg:grid-cols-[1fr_24rem]">
				<form
					className="flex flex-col gap-4"
					onSubmit={(e) => e.preventDefault()}
				>
					<Step n={1} title={m.checkout.contact}>
						<div className="grid gap-4 sm:grid-cols-2">
							<Field
								id="first"
								label={m.checkout.firstName}
								autoComplete="given-name"
							/>
							<Field
								id="last"
								label={m.checkout.lastName}
								autoComplete="family-name"
							/>
							<Field
								id="email"
								label={m.checkout.email}
								type="email"
								autoComplete="email"
								placeholder="ivan@example.bg"
							/>
							<Field
								id="phone"
								label={m.checkout.phone}
								type="tel"
								autoComplete="tel"
								placeholder="0888 123 456"
							/>
						</div>
					</Step>

					<Step n={2} title={m.checkout.delivery}>
						<RadioGroup
							value={delivery}
							onValueChange={setDelivery}
							className="grid gap-3 sm:grid-cols-3"
						>
							{deliveryOptions.map((o) => (
								<OptionCard
									key={o.value}
									value={o.value}
									checked={delivery === o.value}
								>
									<p className="font-medium text-sm">{o.title}</p>
									<p className="mt-0.5 text-muted-foreground text-xs">
										{m.checkout.days(o.days)}
									</p>
									<Price cents={o.cents} className="mt-3 block text-sm" />
								</OptionCard>
							))}
						</RadioGroup>
						<div className="mt-5 grid gap-4 sm:grid-cols-[2fr_1fr]">
							<Field
								id="city"
								label={m.checkout.city}
								autoComplete="address-level2"
								placeholder="София"
							/>
							<Field
								id="postcode"
								label={m.checkout.postcode}
								autoComplete="postal-code"
								placeholder="1000"
							/>
							{delivery === "address" ? (
								<Field
									id="address"
									label={m.checkout.address}
									autoComplete="street-address"
									className="sm:col-span-2"
									placeholder="ул. Граф Игнатиев 1, ет. 2"
								/>
							) : (
								<div className="grid gap-2 sm:col-span-2">
									<Label htmlFor="office">{m.checkout.office}</Label>
									<Select items={offices}>
										<SelectTrigger id="office" className="w-full bg-card">
											<SelectValue placeholder={m.checkout.chooseOffice} />
										</SelectTrigger>
										<SelectContent>
											{offices.map((o) => (
												<SelectItem key={o.value} value={o.value}>
													{o.label}
												</SelectItem>
											))}
										</SelectContent>
									</Select>
								</div>
							)}
						</div>
					</Step>

					<Step n={3} title={m.checkout.payment}>
						<RadioGroup
							value={payment}
							onValueChange={setPayment}
							className="grid gap-3 sm:grid-cols-2"
						>
							{paymentOptions.map((o) => (
								<OptionCard
									key={o.value}
									value={o.value}
									checked={payment === o.value}
								>
									<p className="font-medium text-sm">{o.title}</p>
									<p className="mt-0.5 text-muted-foreground text-xs">
										{o.detail}
									</p>
								</OptionCard>
							))}
						</RadioGroup>
						<div className="mt-5 flex flex-col gap-3">
							<CheckRow id="invoice" label={m.checkout.invoice} />
							<CheckRow id="terms" label={m.checkout.terms} />
						</div>
					</Step>
				</form>

				<aside className="rounded-xl border bg-card p-6 lg:sticky lg:top-[calc(var(--spacing-header)+2.5rem+1.5rem)]">
					<h2 className="font-medium">{m.checkout.yourOrder}</h2>
					<div className="mt-5">
						<CompactLines />
					</div>
					<Totals
						delivery={
							deliveryOptions.find((o) => o.value === delivery)?.cents ??
							deliveryCents
						}
						className="mt-6 border-t pt-5"
					/>
					<Button
						size="lg"
						className="mt-6 h-11 w-full"
						render={<Link to="/preview/confirmation" />}
					>
						<Lock /> {m.checkout.placeOrder}
					</Button>
				</aside>
			</div>
		</Container>
	);
}

function Step({
	n,
	title,
	children,
}: {
	n: number;
	title: string;
	children: React.ReactNode;
}) {
	return (
		<section className="rounded-xl border bg-card p-6">
			<h2 className="mb-5 flex items-center gap-3 font-medium">
				<span className="flex size-6 items-center justify-center rounded-full border font-mono text-muted-foreground text-xs">
					{n}
				</span>
				{title}
			</h2>
			{children}
		</section>
	);
}

function Field({
	id,
	label,
	className,
	...props
}: { id: string; label: string } & React.ComponentProps<"input">) {
	return (
		<div className={cn("grid gap-2", className)}>
			<Label htmlFor={id}>{label}</Label>
			<Input id={id} className="h-10 bg-card" {...props} />
		</div>
	);
}

function OptionCard({
	value,
	checked,
	children,
}: {
	value: string;
	checked: boolean;
	children: React.ReactNode;
}) {
	return (
		<Label
			htmlFor={`opt-${value}`}
			className={cn(
				"flex cursor-pointer items-start gap-3 rounded-lg border p-4 font-normal transition-colors hover:border-border-strong",
				checked && "border-foreground hover:border-foreground",
			)}
		>
			<RadioGroupItem value={value} id={`opt-${value}`} className="mt-0.5" />
			<div className="min-w-0">{children}</div>
		</Label>
	);
}

function CheckRow({ id, label }: { id: string; label: string }) {
	return (
		<div className="flex items-center gap-2.5">
			<Checkbox id={id} />
			<Label htmlFor={id} className="font-normal">
				{label}
			</Label>
		</div>
	);
}
