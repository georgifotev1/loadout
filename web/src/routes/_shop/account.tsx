import {
	useMutation,
	useQueryClient,
	useSuspenseQuery,
} from "@tanstack/react-query";
import {
	createFileRoute,
	Link,
	redirect,
	useNavigate,
} from "@tanstack/react-router";
import { LogOut, ShieldCheck } from "lucide-react";
import { ChangePasswordForm } from "#/components/auth/change-password-form";
import { Container } from "#/components/site/container";
import { Button } from "#/components/ui/button";
import { m } from "#/i18n";
import {
	logout,
	type Me,
	meQuery,
	resendVerification,
	signedIn,
} from "#/lib/api/auth";
import { cn } from "#/lib/utils";

const t = m.auth.account;

// /account: the signed-in customer's profile. Order history comes with orders (checkout).
export const Route = createFileRoute("/_shop/account")({
	beforeLoad: async ({ context, location }) => {
		const me = await context.queryClient.query({ ...meQuery, staleTime: 0 });
		if (!signedIn(me)) {
			throw redirect({ to: "/login", search: { redirect: location.href } });
		}
	},
	head: () => ({
		meta: [
			{ title: m.site.title(t.title) },
			{ name: "robots", content: "noindex" },
		],
	}),
	component: AccountPage,
});

function AccountPage() {
	const { data: me } = useSuspenseQuery(meQuery);
	const navigate = useNavigate();
	const queryClient = useQueryClient();
	const out = useMutation({
		mutationFn: () => logout(queryClient),
		onSuccess: () => navigate({ to: "/" }),
	});
	// Logged out elsewhere (another tab): beforeLoad sends a reload to /login.
	if (!me) return null;

	return (
		<Container className="max-w-3xl py-10 sm:py-14">
			<div className="flex flex-wrap items-center justify-between gap-4">
				<h1 className="font-semibold text-3xl tracking-[-0.02em]">{t.title}</h1>
				<div className="flex gap-2">
					{me.role === "admin" ? (
						<Button variant="outline" render={<Link to="/admin" />}>
							<ShieldCheck />
							{t.admin}
						</Button>
					) : null}
					<Button
						variant="outline"
						onClick={() => out.mutate()}
						disabled={out.isPending}
					>
						<LogOut />
						{t.logout}
					</Button>
				</div>
			</div>

			<div className="mt-8 flex flex-col gap-4">
				<Section title={t.profile}>
					<Profile me={me} />
				</Section>
				<Section title={m.auth.changePassword.title}>
					<p className="-mt-2 mb-4 text-muted-foreground text-sm">
						{m.auth.changePassword.lead}
					</p>
					<ChangePasswordForm />
				</Section>
			</div>
		</Container>
	);
}

function Profile({ me }: { me: Me }) {
	const resend = useMutation({ mutationFn: resendVerification });
	return (
		<>
			<dl className="grid grid-cols-[auto_1fr] items-baseline gap-x-8 gap-y-2 text-sm">
				<dt className="text-muted-foreground">{t.email}</dt>
				<dd className="flex min-w-0 flex-wrap items-baseline gap-x-3 gap-y-1">
					<span className="truncate">{me.email}</span>
					<span
						className={cn(
							"inline-flex items-center gap-1.5 text-xs",
							me.email_verified ? "text-muted-foreground" : "text-foreground",
						)}
					>
						<span
							aria-hidden
							className={cn(
								"size-2 rounded-full",
								me.email_verified ? "bg-success" : "bg-warning",
							)}
						/>
						{me.email_verified ? t.verified : t.unverified}
					</span>
				</dd>
			</dl>
			{me.email_verified ? null : (
				<div className="mt-5 border-t pt-5">
					<p className="text-muted-foreground text-sm">{t.unverifiedHint}</p>
					<div className="mt-3 flex flex-wrap items-center gap-3">
						<Button
							variant="outline"
							onClick={() => resend.mutate()}
							disabled={resend.isPending || resend.isSuccess}
						>
							{resend.isPending ? t.resending : t.resend}
						</Button>
						{resend.isSuccess ? (
							<output className="block text-muted-foreground text-sm">
								{t.resent}
							</output>
						) : null}
						{resend.isError ? (
							<p role="alert" className="text-destructive text-sm">
								{m.auth.errors.generic}
							</p>
						) : null}
					</div>
				</div>
			)}
		</>
	);
}

function Section({
	title,
	children,
}: {
	title: string;
	children: React.ReactNode;
}) {
	return (
		<section className="rounded-lg border bg-card p-5 sm:p-6">
			<h2 className="mb-4 font-semibold">{title}</h2>
			{children}
		</section>
	);
}
