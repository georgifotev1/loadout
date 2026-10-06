import { CircleAlert } from "lucide-react";
import { m } from "#/i18n";
import { ApiError } from "#/lib/api/client";
import { cn } from "#/lib/utils";

// AuthCard is the narrow card every login step sits in.
export function AuthCard({
	title,
	lead,
	children,
	className,
}: {
	title: string;
	lead?: string;
	children: React.ReactNode;
	className?: string;
}) {
	return (
		<div
			className={cn(
				"mx-auto w-full max-w-sm rounded-xl border bg-card p-6 shadow-xs sm:p-8",
				className,
			)}
		>
			<h1 className="font-semibold text-2xl tracking-[-0.02em]">{title}</h1>
			{lead ? (
				<p className="mt-2 text-muted-foreground text-sm">{lead}</p>
			) : null}
			<div className="mt-6">{children}</div>
		</div>
	);
}

/**
 * Whether a code step failed because the page is out of date rather than because of the code:
 * the session expired (401), or its second factor state changed elsewhere (409), e.g. in
 * another tab. The login page then shows the step that fits.
 */
export function isSessionError(err: unknown): boolean {
	return (
		err instanceof ApiError &&
		(err.status === 401 || err.code === "second_factor_state") &&
		err.code !== "invalid_code"
	);
}

/** The message for a failed login or code step. */
export function authErrorMessage(err: unknown): string {
	if (!(err instanceof ApiError)) return m.auth.errors.generic;
	switch (err.code) {
		case "invalid_credentials":
			return m.auth.errors.invalidCredentials;
		case "invalid_code":
			return m.auth.errors.invalidCode;
		case "too_many_attempts":
			return m.auth.errors.tooManyAttempts(err.retryAfter);
		case "unauthenticated":
			return m.auth.errors.sessionExpired;
		default:
			return m.auth.errors.generic;
	}
}

export function FormError({ id, error }: { id: string; error: unknown }) {
	if (!error) return null;
	return (
		<p
			id={id}
			role="alert"
			className="flex items-start gap-2 rounded-md bg-destructive/10 px-3 py-2 text-destructive text-sm"
		>
			<CircleAlert className="mt-0.5 size-4 shrink-0" />
			{authErrorMessage(error)}
		</p>
	);
}
