import { useMutation } from "@tanstack/react-query";
import { useState } from "react";
import { Button } from "#/components/ui/button";
import { m } from "#/i18n";
import { type Me, verifyTotp } from "#/lib/api/auth";
import { AuthCard, FormError, isSessionError } from "./auth-card";
import { CodeField } from "./code-field";

// TotpVerify asks an admin with TOTP on for a code from their app, or one of their recovery
// codes if they don't have the phone.
export function TotpVerify({
	onVerified,
	onSessionError,
	footer,
}: {
	onVerified: (me: Me) => void;
	/** Gets the errors that mean this step is out of date (see isSessionError). */
	onSessionError: (err: unknown) => void;
	footer: React.ReactNode;
}) {
	const [recovery, setRecovery] = useState(false);
	const verify = useMutation({
		mutationFn: verifyTotp,
		onSuccess: onVerified,
		onError: (err) => isSessionError(err) && onSessionError(err),
	});
	const error =
		verify.error && !isSessionError(verify.error) ? verify.error : null;

	return (
		<AuthCard
			title={m.auth.verify.title}
			lead={recovery ? m.auth.verify.recoveryLead : m.auth.verify.lead}
		>
			<form
				className="flex flex-col gap-4"
				onSubmit={(e) => {
					e.preventDefault();
					verify.mutate(String(new FormData(e.currentTarget).get("code")));
				}}
			>
				<FormError id="verify-error" error={error} />
				<CodeField
					label={recovery ? m.auth.verify.recoveryCode : m.auth.verify.code}
					recovery={recovery}
					autoFocus
					errorId="verify-error"
					invalid={Boolean(error)}
				/>
				<Button type="submit" size="lg" disabled={verify.isPending}>
					{verify.isPending ? m.auth.verify.submitting : m.auth.verify.submit}
				</Button>
			</form>
			<Button
				variant="link"
				className="mt-2 h-auto px-0 text-muted-foreground hover:text-foreground"
				onClick={() => {
					setRecovery(!recovery);
					verify.reset();
				}}
			>
				{recovery ? m.auth.verify.useApp : m.auth.verify.useRecovery}
			</Button>
			<div className="mt-4 border-t pt-4">{footer}</div>
		</AuthCard>
	);
}
