import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useRef } from "react";
import { Button } from "#/components/ui/button";
import { m } from "#/i18n";
import { changePassword, meQuery } from "#/lib/api/auth";
import { FormError, fieldError, isFieldError } from "./auth-card";
import { Field } from "./field";

const t = m.auth.changePassword;

// ChangePasswordForm changes the signed-in user's password. The API logs out the other
// sessions and gives this one a new cookie, so the page carries on as before.
export function ChangePasswordForm() {
	const queryClient = useQueryClient();
	const form = useRef<HTMLFormElement>(null);
	const change = useMutation({
		mutationFn: changePassword,
		onSuccess: (me) => {
			queryClient.setQueryData(meQuery.queryKey, me);
			form.current?.reset();
		},
	});
	const err = change.error;

	return (
		<form
			ref={form}
			className="flex max-w-sm flex-col gap-4"
			onSubmit={(e) => {
				e.preventDefault();
				const data = new FormData(e.currentTarget);
				change.mutate({
					current_password: String(data.get("current_password")),
					new_password: String(data.get("new_password")),
				});
			}}
		>
			<FormError
				id="change-password-error"
				error={isFieldError(err) ? null : err}
			/>
			<Field
				name="current_password"
				label={t.current}
				type="password"
				autoComplete="current-password"
				required
				error={fieldError(err, "current_password")}
			/>
			<Field
				name="new_password"
				label={t.next}
				type="password"
				autoComplete="new-password"
				required
				minLength={10}
				maxLength={128}
				hint={m.auth.register.passwordHint}
				error={fieldError(err, "new_password")}
			/>
			<div className="flex flex-wrap items-center gap-3">
				<Button type="submit" disabled={change.isPending}>
					{change.isPending ? t.submitting : t.submit}
				</Button>
				{change.isSuccess ? (
					<output className="block text-muted-foreground text-sm">
						{t.done}
					</output>
				) : null}
			</div>
		</form>
	);
}
