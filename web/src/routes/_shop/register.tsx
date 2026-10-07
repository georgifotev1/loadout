import { useMutation } from "@tanstack/react-query";
import { createFileRoute, Link, redirect } from "@tanstack/react-router";
import { MailCheck } from "lucide-react";
import { useState } from "react";
import {
	AuthCard,
	FormError,
	fieldError,
	isFieldError,
} from "#/components/auth/auth-card";
import { Field } from "#/components/auth/field";
import { Container } from "#/components/site/container";
import { Button } from "#/components/ui/button";
import { m } from "#/i18n";
import { meQuery, register, signedIn } from "#/lib/api/auth";

const t = m.auth.register;

// /register: customer sign-up. The API answers the same for a taken email, so the page always
// ends at "check your email"; the email says what to do next.
export const Route = createFileRoute("/_shop/register")({
	beforeLoad: async ({ context }) => {
		const me = await context.queryClient.query({ ...meQuery, staleTime: 0 });
		if (signedIn(me)) throw redirect({ to: "/account" });
	},
	head: () => ({
		meta: [
			{ title: m.site.title(t.title) },
			{ name: "robots", content: "noindex" },
		],
	}),
	component: RegisterPage,
});

function RegisterPage() {
	const [email, setEmail] = useState("");
	const submit = useMutation({ mutationFn: register });
	const err = submit.error;

	return (
		<Container className="py-12 sm:py-20">
			{submit.isSuccess ? (
				<AuthCard title={t.sentTitle}>
					<MailCheck className="size-8 text-brand-ink" aria-hidden />
					<p className="mt-4 text-sm">{t.sent(email)}</p>
					<p className="mt-3 text-muted-foreground text-sm">{t.sentHint}</p>
					<Button
						className="mt-6 w-full"
						size="lg"
						render={<Link to="/login" />}
					>
						{m.auth.login.title}
					</Button>
				</AuthCard>
			) : (
				<AuthCard title={t.title} lead={t.lead}>
					<form
						className="flex flex-col gap-4"
						onSubmit={(e) => {
							e.preventDefault();
							const data = new FormData(e.currentTarget);
							const next = String(data.get("email")).trim();
							setEmail(next);
							submit.mutate({
								email: next,
								password: String(data.get("password")),
							});
						}}
					>
						<FormError
							id="register-error"
							error={isFieldError(err) ? null : err}
						/>
						<Field
							name="email"
							label={t.email}
							type="email"
							autoComplete="email"
							required
							autoFocus
							error={fieldError(err, "email")}
						/>
						<Field
							name="password"
							label={t.password}
							type="password"
							autoComplete="new-password"
							required
							minLength={10}
							maxLength={128}
							hint={t.passwordHint}
							error={fieldError(err, "password")}
						/>
						<Button type="submit" size="lg" disabled={submit.isPending}>
							{submit.isPending ? t.submitting : t.submit}
						</Button>
					</form>
					<p className="mt-6 text-center text-muted-foreground text-sm">
						{t.haveAccount}{" "}
						<Link
							to="/login"
							className="font-medium text-foreground underline-offset-4 hover:underline"
						>
							{t.login}
						</Link>
					</p>
				</AuthCard>
			)}
		</Container>
	);
}
