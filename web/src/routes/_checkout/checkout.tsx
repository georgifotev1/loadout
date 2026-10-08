import {
	useMutation,
	useQuery,
	useQueryClient,
	useSuspenseQuery,
} from "@tanstack/react-query";
import { createFileRoute, Link, useNavigate } from "@tanstack/react-router";
import { Banknote, CreditCard, Lock, TriangleAlert, Truck } from "lucide-react";
import { useState } from "react";
import { Field } from "#/components/auth/field";
import { CompactLines } from "#/components/order/compact-lines";
import { Totals } from "#/components/order/totals";
import { Price } from "#/components/product/price";
import { Container } from "#/components/site/container";
import { PageSkeleton } from "#/components/site/page-skeletons";
import { StatusPage } from "#/components/site/status-page";
import { Button } from "#/components/ui/button";
import { Label } from "#/components/ui/label";
import { RadioGroup, RadioGroupItem } from "#/components/ui/radio-group";
import { m } from "#/i18n";
import { meQuery } from "#/lib/api/auth";
import { cartQuery } from "#/lib/api/cart";
import {
	type Checkout,
	checkoutQuery,
	type PaymentMethod,
	placeOrder,
} from "#/lib/api/orders";
import {
	type CheckoutField,
	cartChanged,
	checkoutError,
	checkoutFieldError,
} from "#/lib/checkout";
import { cn } from "#/lib/utils";

const t = m.checkout;

// /checkout: contact, delivery address and payment, next to the cart with the API's totals. The
// cart loads in the browser only (see cartQuery); the logged-in customer's email is filled in.
export const Route = createFileRoute("/_checkout/checkout")({
	loader: ({ context }) => context.queryClient.query(meQuery).catch(() => null),
	head: () => ({
		meta: [
			{ title: m.site.title(t.title) },
			{ name: "robots", content: "noindex" },
		],
	}),
	component: CheckoutPage,
});

function CheckoutPage() {
	const queryClient = useQueryClient();
	const { data: checkout, isError } = useQuery({
		...checkoutQuery,
		enabled: typeof window !== "undefined",
	});

	if (isError && !checkout) {
		return (
			<StatusPage title={m.error.title} detail={m.error.detail}>
				<Button
					size="lg"
					onClick={() =>
						queryClient.refetchQueries({ queryKey: checkoutQuery.queryKey })
					}
				>
					{m.error.retry}
				</Button>
			</StatusPage>
		);
	}
	if (!checkout) return <PageSkeleton />;
	if (checkout.cart.lines.length === 0) {
		return (
			<StatusPage title={m.cart.empty}>
				<Button size="lg" render={<Link to="/" />}>
					{m.cart.continueShopping}
				</Button>
			</StatusPage>
		);
	}
	return <CheckoutForm checkout={checkout} />;
}

const paymentOptions = {
	cod: {
		icon: Banknote,
		title: t.cashOnDelivery,
		detail: t.cashOnDeliveryDetail,
	},
	card: { icon: CreditCard, title: t.card, detail: t.cardDetail },
} satisfies Record<PaymentMethod, unknown>;

function CheckoutForm({ checkout }: { checkout: Checkout }) {
	const { data: me } = useSuspenseQuery(meQuery);
	const queryClient = useQueryClient();
	const navigate = useNavigate();
	const [payment, setPayment] = useState<PaymentMethod>("cod");
	const submit = useMutation({
		mutationFn: placeOrder,
		onSuccess: async (placed) => {
			// A card order goes on to Stripe's page. The page stays "placing" until it unloads.
			if (placed.payment_url) {
				window.location.assign(placed.payment_url);
				return;
			}
			await navigate({
				to: "/order/$id",
				params: { id: placed.id },
				search: { token: placed.token },
			});
			// The cart is gone: the header's count and a later checkout start fresh.
			queryClient.removeQueries({ queryKey: checkoutQuery.queryKey });
			await queryClient.invalidateQueries({ queryKey: cartQuery.queryKey });
		},
		onError: (err) => {
			// Show the cart as it is now, with its new total to agree to.
			if (cartChanged(err)) {
				void queryClient.invalidateQueries({
					queryKey: checkoutQuery.queryKey,
				});
			}
		},
	});
	const err = submit.error;
	const formError = checkoutError(err);
	const field = (name: CheckoutField) => ({
		name,
		error: checkoutFieldError(err, name),
	});

	return (
		<Container className="pt-10">
			<h1 className="font-semibold text-3xl tracking-[-0.03em]">{t.title}</h1>

			<form
				className="mt-8 grid items-start gap-8 lg:grid-cols-[1fr_24rem]"
				onSubmit={(e) => {
					e.preventDefault();
					const data = new FormData(e.currentTarget);
					const text = (k: CheckoutField) => String(data.get(k) ?? "").trim();
					submit.mutate({
						name: text("name"),
						email: text("email"),
						phone: text("phone"),
						city: text("city"),
						postcode: text("postcode"),
						address: text("address"),
						payment,
						expected_total_cents: checkout.total.amount_cents,
					});
				}}
			>
				<div className="flex flex-col gap-4">
					<Step n={1} title={t.contact}>
						<div className="grid gap-4 sm:grid-cols-2">
							<div className="sm:col-span-2">
								<Field
									{...field("name")}
									label={t.name}
									autoComplete="name"
									required
									maxLength={100}
								/>
							</div>
							<Field
								{...field("email")}
								label={t.email}
								type="email"
								autoComplete="email"
								placeholder="ivan@example.bg"
								defaultValue={me?.email}
								hint={t.emailHint}
								required
							/>
							<Field
								{...field("phone")}
								label={t.phone}
								type="tel"
								autoComplete="tel"
								placeholder="0888 123 456"
								hint={t.phoneHint}
								required
							/>
						</div>
					</Step>

					<Step n={2} title={t.delivery}>
						<div className="flex items-center gap-3 rounded-lg border border-foreground p-4">
							<Truck className="size-5 shrink-0 text-muted-foreground" />
							<p className="flex-1 font-medium text-sm">{t.toAddress}</p>
							<Price
								cents={checkout.delivery.amount_cents}
								className="text-sm"
							/>
						</div>
						<p className="mt-2 text-muted-foreground text-xs">
							{t.officesSoon}
						</p>
						<div className="mt-5 grid gap-4 sm:grid-cols-[2fr_1fr]">
							<Field
								{...field("city")}
								label={t.city}
								autoComplete="address-level2"
								placeholder="София"
								required
								maxLength={100}
							/>
							<Field
								{...field("postcode")}
								label={t.postcode}
								autoComplete="postal-code"
								inputMode="numeric"
								placeholder="1000"
								required
								maxLength={4}
							/>
							<div className="sm:col-span-2">
								<Field
									{...field("address")}
									label={t.address}
									autoComplete="street-address"
									placeholder="ул. Граф Игнатиев 1, ет. 2"
									hint={t.addressHint}
									required
									maxLength={200}
								/>
							</div>
						</div>
					</Step>

					<Step n={3} title={t.payment}>
						<RadioGroup
							value={payment}
							onValueChange={(v) => setPayment(v as PaymentMethod)}
							className="grid gap-3 sm:grid-cols-2"
						>
							{checkout.payments.map((p) => {
								const o = paymentOptions[p];
								return (
									<Label
										key={p}
										htmlFor={`payment-${p}`}
										className={cn(
											"flex cursor-pointer items-start gap-3 rounded-lg border p-4 font-normal transition-colors hover:border-border-strong",
											payment === p &&
												"border-foreground hover:border-foreground",
										)}
									>
										<RadioGroupItem
											value={p}
											id={`payment-${p}`}
											className="mt-0.5"
										/>
										<div className="min-w-0 flex-1">
											<p className="font-medium text-sm">{o.title}</p>
											<p className="mt-0.5 text-muted-foreground text-xs">
												{o.detail}
											</p>
										</div>
										<o.icon className="size-5 shrink-0 text-muted-foreground" />
									</Label>
								);
							})}
						</RadioGroup>
					</Step>
				</div>

				<aside className="rounded-xl border bg-card p-6 lg:sticky lg:top-6">
					<h2 className="font-medium">{t.yourOrder}</h2>
					<div className="mt-5">
						<CompactLines
							lines={checkout.cart.lines.map((l) => ({
								key: l.product_id,
								name: l.name,
								image: l.product?.image,
								quantity: l.quantity,
								cents: l.line_total?.amount_cents,
							}))}
						/>
					</div>
					<Totals
						subtotal={checkout.cart.subtotal.amount_cents}
						delivery={checkout.delivery.amount_cents}
						total={checkout.total.amount_cents}
						className="mt-6 border-t pt-5"
					/>
					{formError || !checkout.orderable ? (
						<p
							role="alert"
							className="mt-5 flex items-start gap-2 rounded-md bg-destructive/10 px-3 py-2 text-destructive text-sm"
						>
							<TriangleAlert className="mt-0.5 size-4 shrink-0" />
							<span>
								{formError ?? t.notOrderable}{" "}
								{checkout.orderable ? null : (
									<Link to="/cart" className="underline underline-offset-4">
										{t.backToCart}
									</Link>
								)}
							</span>
						</p>
					) : null}
					<Button
						type="submit"
						size="lg"
						className="mt-6 h-11 w-full"
						disabled={
							!checkout.orderable || submit.isPending || submit.isSuccess
						}
					>
						<Lock />{" "}
						{submit.isPending || submit.isSuccess
							? t.placing
							: payment === "card"
								? t.placeOrderAndPay
								: t.placeOrder}
					</Button>
				</aside>
			</form>
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
