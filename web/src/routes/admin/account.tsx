import { useMutation } from "@tanstack/react-query";
import { createFileRoute } from "@tanstack/react-router";
import { KeyRound } from "lucide-react";
import { useState } from "react";
import { AdminPage, StatusDot } from "#/components/admin/admin-page";
import { AdminError, AdminNotFound } from "#/components/admin/admin-status";
import {
	RecoveryCodeList,
	SavedConfirm,
} from "#/components/auth/recovery-codes";
import {
	AlertDialog,
	AlertDialogAction,
	AlertDialogCancel,
	AlertDialogContent,
	AlertDialogDescription,
	AlertDialogFooter,
	AlertDialogHeader,
	AlertDialogTitle,
	AlertDialogTrigger,
} from "#/components/ui/alert-dialog";
import { Button } from "#/components/ui/button";
import { m } from "#/i18n";
import { regenerateRecoveryCodes } from "#/lib/api/auth";

// /admin/account: the signed-in admin's profile and second factor. Changing the password comes
// with customer accounts (Phase 7), which build password changes and resets for everyone.
export const Route = createFileRoute("/admin/account")({
	head: () => ({ meta: [{ title: m.site.title(m.admin.account.title) }] }),
	errorComponent: AdminError,
	notFoundComponent: AdminNotFound,
	component: AccountPage,
});

const t = m.admin.account;

function AccountPage() {
	const { me } = Route.useRouteContext();
	const totp = me.second_factor === "verified";

	return (
		<AdminPage title={t.title} lead={t.lead}>
			<div className="flex max-w-3xl flex-col gap-4">
				<Section title={t.profile}>
					<dl className="grid grid-cols-[auto_1fr] gap-x-8 gap-y-2 text-sm">
						<dt className="text-muted-foreground">{t.email}</dt>
						<dd className="min-w-0 truncate">{me.email}</dd>
						<dt className="text-muted-foreground">{t.role}</dt>
						<dd>{t.roleAdmin}</dd>
					</dl>
				</Section>

				<Section title={t.twoFactor}>
					<p className="text-sm">
						<StatusDot wrap tone={totp ? "success" : "warning"}>
							{totp ? t.twoFactorOn : t.twoFactorOff}
						</StatusDot>
					</p>
					{totp ? (
						<div className="mt-5 border-t pt-5">
							<h3 className="font-medium text-sm">{t.recovery}</h3>
							<p className="mt-1 max-w-xl text-muted-foreground text-sm">
								{t.recoveryLead}
							</p>
							<NewRecoveryCodes />
						</div>
					) : null}
				</Section>
			</div>
		</AdminPage>
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
		<section className="rounded-lg border bg-card p-5">
			<h2 className="mb-4 font-semibold">{title}</h2>
			{children}
		</section>
	);
}

// NewRecoveryCodes asks first (the old codes stop working), then shows the new ones once, in a
// dialog that closes only after the admin says they saved them.
function NewRecoveryCodes() {
	const [confirming, setConfirming] = useState(false);
	const regenerate = useMutation({ mutationFn: regenerateRecoveryCodes });
	const codes = regenerate.data?.recovery_codes;

	return (
		<>
			<AlertDialog open={confirming} onOpenChange={setConfirming}>
				<AlertDialogTrigger
					render={
						<Button
							variant="outline"
							className="mt-4"
							disabled={regenerate.isPending}
						/>
					}
				>
					<KeyRound />
					{t.regenerate}
				</AlertDialogTrigger>
				<AlertDialogContent>
					<AlertDialogHeader>
						<AlertDialogTitle>{t.regenerateTitle}</AlertDialogTitle>
						<AlertDialogDescription>{t.regenerateBody}</AlertDialogDescription>
					</AlertDialogHeader>
					<AlertDialogFooter>
						<AlertDialogCancel>{t.cancel}</AlertDialogCancel>
						<AlertDialogAction
							onClick={() => {
								setConfirming(false);
								regenerate.mutate();
							}}
						>
							{t.regenerate}
						</AlertDialogAction>
					</AlertDialogFooter>
				</AlertDialogContent>
			</AlertDialog>
			{regenerate.isError ? (
				<p role="alert" className="mt-2 text-destructive text-sm">
					{t.failed}
				</p>
			) : null}

			<AlertDialog
				open={!!codes}
				onOpenChange={(open) => {
					if (!open) regenerate.reset();
				}}
			>
				<AlertDialogContent>
					<AlertDialogHeader>
						<AlertDialogTitle>{t.newCodesTitle}</AlertDialogTitle>
						<AlertDialogDescription>{t.newCodesLead}</AlertDialogDescription>
					</AlertDialogHeader>
					<div>
						{codes ? <RecoveryCodeList codes={codes} /> : null}
						<SavedConfirm
							label={t.done}
							onContinue={() => regenerate.reset()}
						/>
					</div>
				</AlertDialogContent>
			</AlertDialog>
		</>
	);
}
