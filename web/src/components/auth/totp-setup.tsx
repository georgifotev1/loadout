import { useMutation, useQuery } from "@tanstack/react-query";
import { Check, ChevronDown, Copy } from "lucide-react";
import { useEffect, useState } from "react";
import { Button } from "#/components/ui/button";
import { m } from "#/i18n";
import { confirmTotp, totpEnrollQuery } from "#/lib/api/auth";
import type { Schemas } from "#/lib/api/client";
import { AuthCard, FormError, isSessionError } from "./auth-card";
import { CodeField } from "./code-field";
import { QrCode } from "./qr-code";

// TotpSetup walks an admin through turning TOTP on at their first login: install an app, scan
// the QR code (or type the key), then enter a first code to prove the app has it.
export function TotpSetup({
	onConfirmed,
	onSessionError,
	footer,
}: {
	onConfirmed: (res: Schemas["TotpConfirmed"]) => void;
	/** Gets the errors that mean this step is out of date (see isSessionError). */
	onSessionError: (err: unknown) => void;
	footer: React.ReactNode;
}) {
	const setup = useQuery(totpEnrollQuery);
	const confirm = useMutation({
		mutationFn: confirmTotp,
		onSuccess: onConfirmed,
		onError: (err) => isSessionError(err) && onSessionError(err),
	});
	useEffect(() => {
		if (isSessionError(setup.error)) onSessionError(setup.error);
	}, [setup.error, onSessionError]);
	const error = [confirm.error, setup.error].find(
		(err) => err && !isSessionError(err),
	);

	return (
		<AuthCard
			title={m.auth.setup.title}
			lead={m.auth.setup.lead}
			className="max-w-md"
		>
			<ol className="flex flex-col gap-6">
				<Step n={1} title={m.auth.setup.installTitle}>
					<p>{m.auth.setup.installDetail}</p>
				</Step>
				<Step n={2} title={m.auth.setup.scanTitle}>
					<p>{m.auth.setup.scanDetail}</p>
					{setup.data ? (
						<>
							<QrCode
								value={setup.data.uri}
								label={m.auth.setup.qrLabel}
								className="mt-3 size-44 rounded-md border"
							/>
							<ManualKey secret={setup.data.secret} />
						</>
					) : (
						<div className="mt-3 flex size-44 items-center justify-center rounded-md border bg-muted text-xs">
							{setup.isPending ? m.auth.setup.loading : null}
						</div>
					)}
				</Step>
				<Step n={3} title={m.auth.setup.codeTitle}>
					<p>{m.auth.setup.codeDetail}</p>
					<form
						className="mt-3 flex flex-col gap-4"
						onSubmit={(e) => {
							e.preventDefault();
							confirm.mutate(String(new FormData(e.currentTarget).get("code")));
						}}
					>
						<FormError id="setup-error" error={error} />
						<CodeField
							label={m.auth.setup.code}
							errorId="setup-error"
							invalid={Boolean(error)}
						/>
						<Button
							type="submit"
							size="lg"
							disabled={!setup.data || confirm.isPending}
						>
							{confirm.isPending
								? m.auth.setup.submitting
								: m.auth.setup.submit}
						</Button>
					</form>
				</Step>
			</ol>
			<div className="mt-6 border-t pt-4">{footer}</div>
		</AuthCard>
	);
}

function Step({
	n,
	title,
	children,
}: {
	n: number;
	title: string;
	children: React.ReactNode;
}) {
	return (
		<li className="grid grid-cols-[1.75rem_1fr] gap-x-3">
			<span className="flex size-7 items-center justify-center rounded-full bg-muted font-semibold text-sm tabular-nums">
				{n}
			</span>
			<h2 className="self-center font-medium">{title}</h2>
			<div className="col-start-2 mt-1 text-muted-foreground text-sm">
				{children}
			</div>
		</li>
	);
}

// ManualKey shows the secret for typing in by hand, in groups of four like most apps show it.
function ManualKey({ secret }: { secret: string }) {
	const [open, setOpen] = useState(false);
	const [copied, setCopied] = useState(false);

	return (
		<div className="mt-3">
			<button
				type="button"
				aria-expanded={open}
				onClick={() => setOpen(!open)}
				className="flex items-center gap-1 text-left text-foreground underline-offset-4 hover:underline"
			>
				{m.auth.setup.manualToggle}
				<ChevronDown
					className={
						open ? "size-4 rotate-180 transition" : "size-4 transition"
					}
				/>
			</button>
			{open ? (
				<div className="mt-2">
					<p>{m.auth.setup.manualDetail}</p>
					<div className="mt-2 flex items-center gap-2 rounded-md border bg-muted px-3 py-2">
						<code className="flex-1 break-all font-mono text-foreground text-sm">
							{secret.match(/.{1,4}/g)?.join(" ")}
						</code>
						<Button
							variant="ghost"
							size="icon-sm"
							aria-label={
								copied ? m.auth.recovery.copied : m.auth.recovery.copy
							}
							onClick={() =>
								navigator.clipboard.writeText(secret).then(
									() => setCopied(true),
									// No clipboard access: the key is on screen to copy by hand.
									() => {},
								)
							}
						>
							{copied ? <Check /> : <Copy />}
						</Button>
					</div>
				</div>
			) : null}
		</div>
	);
}
