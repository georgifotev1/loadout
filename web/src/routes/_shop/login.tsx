import { useQueryClient, useSuspenseQuery } from "@tanstack/react-query";
import { createFileRoute, redirect, useNavigate } from "@tanstack/react-router";
import { useCallback, useState } from "react";
import { LoginForm } from "#/components/auth/login-form";
import { RecoveryCodes } from "#/components/auth/recovery-codes";
import { TotpSetup } from "#/components/auth/totp-setup";
import { TotpVerify } from "#/components/auth/totp-verify";
import { Container } from "#/components/site/container";
import { Button } from "#/components/ui/button";
import { m } from "#/i18n";
import { logout, type Me, meQuery, signedIn } from "#/lib/api/auth";
import { ApiError } from "#/lib/api/client";
import { afterLogin } from "#/lib/login-redirect";

// /login: email and password, then for an admin the second factor. The step comes from the
// session (GET /auth/me), so a reload or a second tab lands on the right one.
export const Route = createFileRoute("/_shop/login")({
	validateSearch: (search): { redirect?: string } =>
		typeof search.redirect === "string" ? { redirect: search.redirect } : {},
	beforeLoad: async ({ context, search }) => {
		const me = await context.queryClient.query({ ...meQuery, staleTime: 0 });
		if (signedIn(me)) throw redirect({ href: afterLogin(me, search.redirect) });
	},
	head: () => ({
		meta: [
			{ title: m.site.title(m.auth.login.title) },
			{ name: "robots", content: "noindex" },
		],
	}),
	component: LoginPage,
});

function LoginPage() {
	const search = Route.useSearch();
	const navigate = useNavigate();
	const queryClient = useQueryClient();
	const { data: me } = useSuspenseQuery(meQuery);
	const [recoveryCodes, setRecoveryCodes] = useState<string[] | null>(null);
	const [notice, setNotice] = useState<unknown>(null);

	function finish(next: Me) {
		navigate({ href: afterLogin(next, search.redirect), replace: true });
	}

	function update(next: Me) {
		queryClient.setQueryData(meQuery.queryKey, next);
		setNotice(null);
		if (signedIn(next)) finish(next);
	}

	// An expired session goes back to the password step; a changed state reloads it. Stable, as
	// TotpSetup calls it from an effect.
	const onSessionError = useCallback(
		(err: unknown) => {
			if (err instanceof ApiError && err.status === 401) {
				queryClient.setQueryData(meQuery.queryKey, null);
				setNotice(err);
			} else {
				queryClient.invalidateQueries({ queryKey: meQuery.queryKey });
			}
		},
		[queryClient],
	);

	const otherAccount = (
		<Button
			variant="link"
			className="h-auto px-0 text-muted-foreground hover:text-foreground"
			onClick={() => logout(queryClient).catch(() => {})}
		>
			{m.auth.otherAccount}
		</Button>
	);

	let step: React.ReactNode = null;
	if (recoveryCodes && me) {
		step = (
			<RecoveryCodes codes={recoveryCodes} onContinue={() => finish(me)} />
		);
	} else if (!me) {
		step = <LoginForm notice={notice} onLoggedIn={update} />;
	} else if (me.second_factor === "enroll") {
		step = (
			<TotpSetup
				onConfirmed={(res) => {
					setRecoveryCodes(res.recovery_codes);
					queryClient.setQueryData(meQuery.queryKey, res.me);
				}}
				onSessionError={onSessionError}
				footer={otherAccount}
			/>
		);
	} else if (me.second_factor === "verify") {
		step = (
			<TotpVerify
				onVerified={update}
				onSessionError={onSessionError}
				footer={otherAccount}
			/>
		);
	}
	// Otherwise the session is signed in and finish is navigating away.

	return <Container className="py-12 sm:py-20">{step}</Container>;
}
