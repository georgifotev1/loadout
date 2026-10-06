import { useMutation } from "@tanstack/react-query";
import { Button } from "#/components/ui/button";
import { Input } from "#/components/ui/input";
import { Label } from "#/components/ui/label";
import { m } from "#/i18n";
import { login, type Me } from "#/lib/api/auth";
import { AuthCard, FormError } from "./auth-card";

// LoginForm is the email and password step. An admin goes on to the second factor.
export function LoginForm({
	notice,
	onLoggedIn,
}: {
	/** Shown above the form, e.g. why the user is back here. */
	notice?: unknown;
	onLoggedIn: (me: Me) => void;
}) {
	const submit = useMutation({ mutationFn: login, onSuccess: onLoggedIn });
	const error = submit.error ?? notice;

	return (
		<AuthCard title={m.auth.login.title}>
			<form
				className="flex flex-col gap-4"
				aria-describedby={error ? "login-error" : undefined}
				onSubmit={(e) => {
					e.preventDefault();
					const data = new FormData(e.currentTarget);
					submit.mutate({
						email: String(data.get("email")),
						password: String(data.get("password")),
					});
				}}
			>
				<FormError id="login-error" error={error} />
				<div className="flex flex-col gap-2">
					<Label htmlFor="email">{m.auth.login.email}</Label>
					<Input
						id="email"
						name="email"
						type="email"
						autoComplete="username"
						required
						autoFocus
					/>
				</div>
				<div className="flex flex-col gap-2">
					<Label htmlFor="password">{m.auth.login.password}</Label>
					<Input
						id="password"
						name="password"
						type="password"
						autoComplete="current-password"
						required
					/>
				</div>
				<Button type="submit" size="lg" disabled={submit.isPending}>
					{submit.isPending ? m.auth.login.submitting : m.auth.login.submit}
				</Button>
			</form>
		</AuthCard>
	);
}
