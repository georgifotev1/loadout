import { useMutation } from "@tanstack/react-query";
import { createFileRoute, Link } from "@tanstack/react-router";
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
import { forgotPassword } from "#/lib/api/auth";

const t = m.auth.forgot;

// /forgot-password: asks for a reset link. The API answers the same for an unknown email.
export const Route = createFileRoute("/_shop/forgot-password")({
	head: () => ({
		meta: [
			{ title: m.site.title(t.title) },
			{ name: "robots", content: "noindex" },
		],
	}),
	component: ForgotPasswordPage,
});

function ForgotPasswordPage() {
	const [email, setEmail] = useState("");
	const submit = useMutation({ mutationFn: forgotPassword });
	const err = submit.error;

	const back = (
		<Link
			to="/login"
			className="mt-6 block text-center text-muted-foreground text-sm hover:text-foreground"
		>
			{t.back}
		</Link>
	);

	return (
		<Container className="py-12 sm:py-20">
			{submit.isSuccess ? (
				<AuthCard title={t.sentTitle}>
					<MailCheck className="size-8 text-brand-ink" aria-hidden />
					<p className="mt-4 text-sm">{t.sent(email)}</p>
					<p className="mt-3 text-muted-foreground text-sm">
						{m.auth.register.sentHint}
					</p>
					{back}
				</AuthCard>
			) : (
				<AuthCard title={t.title} lead={t.lead}>
					<form
						className="flex flex-col gap-4"
						onSubmit={(e) => {
							e.preventDefault();
							const next = String(
								new FormData(e.currentTarget).get("email"),
							).trim();
							setEmail(next);
							submit.mutate(next);
						}}
					>
						<FormError
							id="forgot-error"
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
						<Button type="submit" size="lg" disabled={submit.isPending}>
							{submit.isPending ? t.submitting : t.submit}
						</Button>
					</form>
					{back}
				</AuthCard>
			)}
		</Container>
	);
}
