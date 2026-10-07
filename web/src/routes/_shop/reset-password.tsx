import { useMutation, useQueryClient } from "@tanstack/react-query";
import { createFileRoute, Link, useNavigate } from "@tanstack/react-router";
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
import { loggedIn, resetPassword, signedIn } from "#/lib/api/auth";
import { afterLogin } from "#/lib/login-redirect";

const t = m.auth.reset;

// /reset-password?token=…: the page the reset email links to. Setting the password logs in;
// an admin then goes on to the second factor on /login.
export const Route = createFileRoute("/_shop/reset-password")({
	validateSearch: (search): { token?: string } =>
		typeof search.token === "string" ? { token: search.token } : {},
	head: () => ({
		meta: [
			{ title: m.site.title(t.title) },
			{ name: "robots", content: "noindex" },
			// The token is in the URL: don't send it on in a Referer.
			{ name: "referrer", content: "no-referrer" },
		],
	}),
	component: ResetPasswordPage,
});

function ResetPasswordPage() {
	const { token = "" } = Route.useSearch();
	const navigate = useNavigate();
	const queryClient = useQueryClient();
	const submit = useMutation({
		mutationFn: resetPassword,
		onSuccess: (me) => {
			loggedIn(queryClient, me);
			navigate({
				href: signedIn(me) ? afterLogin(me) : "/login",
				replace: true,
			});
		},
	});
	const err = submit.error;

	return (
		<Container className="py-12 sm:py-20">
			{!token || fieldError(err, "token") ? (
				<AuthCard title={t.invalidTitle}>
					<p className="text-sm">{t.invalid}</p>
					<Button
						className="mt-6 w-full"
						size="lg"
						render={<Link to="/forgot-password" />}
					>
						{t.again}
					</Button>
				</AuthCard>
			) : (
				<AuthCard title={t.title} lead={t.lead}>
					<form
						className="flex flex-col gap-4"
						onSubmit={(e) => {
							e.preventDefault();
							submit.mutate({
								token,
								password: String(new FormData(e.currentTarget).get("password")),
							});
						}}
					>
						<FormError
							id="reset-error"
							error={isFieldError(err) ? null : err}
						/>
						<Field
							name="password"
							label={t.password}
							type="password"
							autoComplete="new-password"
							required
							autoFocus
							minLength={10}
							maxLength={128}
							hint={m.auth.register.passwordHint}
							error={fieldError(err, "password")}
						/>
						<Button type="submit" size="lg" disabled={submit.isPending}>
							{submit.isPending ? t.submitting : t.submit}
						</Button>
					</form>
				</AuthCard>
			)}
		</Container>
	);
}
