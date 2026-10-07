import { queryOptions, useQuery, useQueryClient } from "@tanstack/react-query";
import { createFileRoute, Link } from "@tanstack/react-router";
import { CircleCheck } from "lucide-react";
import { useEffect } from "react";
import { AuthCard, fieldError } from "#/components/auth/auth-card";
import { Container } from "#/components/site/container";
import { Button } from "#/components/ui/button";
import { m } from "#/i18n";
import { meQuery, verifyEmail } from "#/lib/api/auth";

const t = m.auth.verifyEmail;

// /verify-email?token=…: the page the sign-up email links to. It verifies on arrival, in the
// browser only: a link preview fetching the page shouldn't use the token up.
export const Route = createFileRoute("/_shop/verify-email")({
	validateSearch: (search): { token?: string } =>
		typeof search.token === "string" ? { token: search.token } : {},
	head: () => ({
		meta: [
			{ title: m.site.title(t.title) },
			{ name: "robots", content: "noindex" },
			{ name: "referrer", content: "no-referrer" },
		],
	}),
	component: VerifyEmailPage,
});

// A query, not a mutation, so it runs once per token, not again on a re-render or a second
// mount in StrictMode (like totpEnrollQuery).
const verifyQuery = (token: string) =>
	queryOptions({
		queryKey: ["auth", "verify-email", token],
		queryFn: async () => {
			await verifyEmail(token);
			return true;
		},
		staleTime: Number.POSITIVE_INFINITY,
		gcTime: 0,
		retry: false,
		refetchOnWindowFocus: false,
	});

function VerifyEmailPage() {
	const { token = "" } = Route.useSearch();
	const queryClient = useQueryClient();
	const verify = useQuery({
		...verifyQuery(token),
		enabled: token !== "" && typeof window !== "undefined",
	});

	useEffect(() => {
		if (verify.isSuccess) {
			void queryClient.invalidateQueries({ queryKey: meQuery.queryKey });
		}
	}, [verify.isSuccess, queryClient]);

	if (verify.isSuccess) {
		return (
			<Container className="py-12 sm:py-20">
				<AuthCard title={t.doneTitle}>
					<CircleCheck className="size-8 text-success" aria-hidden />
					<p className="mt-4 text-sm">{t.done}</p>
					<div className="mt-6 flex flex-col gap-2">
						<Button size="lg" render={<Link to="/account" />}>
							{t.toAccount}
						</Button>
						<Button size="lg" variant="outline" render={<Link to="/" />}>
							{t.toShop}
						</Button>
					</div>
				</AuthCard>
			</Container>
		);
	}

	const failed = !token || verify.isError;
	return (
		<Container className="py-12 sm:py-20">
			<AuthCard title={failed ? t.invalidTitle : t.title}>
				{failed ? (
					<>
						<p className="text-sm">
							{fieldError(verify.error, "token") || !token
								? t.invalid
								: m.auth.errors.generic}
						</p>
						<Button
							className="mt-6 w-full"
							size="lg"
							render={<Link to="/account" />}
						>
							{t.toAccount}
						</Button>
					</>
				) : (
					<output className="block text-muted-foreground text-sm">
						{t.verifying}
					</output>
				)}
			</AuthCard>
		</Container>
	);
}
