import { Check, Copy, Download } from "lucide-react";
import { useId, useState } from "react";
import { Button } from "#/components/ui/button";
import { Checkbox } from "#/components/ui/checkbox";
import { Label } from "#/components/ui/label";
import { m } from "#/i18n";
import { AuthCard } from "./auth-card";

// RecoveryCodes shows the codes the API returns only once, and asks the admin to confirm they
// saved them before going on.
export function RecoveryCodes({
	codes,
	onContinue,
}: {
	codes: string[];
	onContinue: () => void;
}) {
	return (
		<AuthCard
			title={m.auth.recovery.title}
			lead={m.auth.recovery.lead}
			className="max-w-md"
		>
			<RecoveryCodeList codes={codes} />
			<SavedConfirm onContinue={onContinue} />
		</AuthCard>
	);
}

// RecoveryCodeList is the codes with copy and download: on the login page after setup, and on
// the account page when an admin makes new ones.
export function RecoveryCodeList({ codes }: { codes: string[] }) {
	const [copied, setCopied] = useState(false);
	const text = `${codes.join("\n")}\n`;

	function download() {
		const url = URL.createObjectURL(new Blob([text], { type: "text/plain" }));
		const a = document.createElement("a");
		a.href = url;
		a.download = m.auth.recovery.fileName;
		a.click();
		URL.revokeObjectURL(url);
	}

	return (
		<>
			<ul
				aria-label={m.auth.recovery.listLabel}
				className="grid grid-cols-1 gap-x-4 gap-y-1.5 rounded-md border bg-muted px-4 py-3 font-mono text-sm sm:grid-cols-2"
			>
				{codes.map((code) => (
					<li key={code} className="whitespace-nowrap">
						{code}
					</li>
				))}
			</ul>
			<div className="mt-3 flex gap-2">
				<Button
					variant="outline"
					size="sm"
					onClick={() =>
						navigator.clipboard.writeText(text).then(
							() => setCopied(true),
							// No clipboard access: download them or copy by hand.
							() => {},
						)
					}
				>
					{copied ? <Check /> : <Copy />}
					{copied ? m.auth.recovery.copied : m.auth.recovery.copy}
				</Button>
				<Button variant="outline" size="sm" onClick={download}>
					<Download />
					{m.auth.recovery.download}
				</Button>
			</div>
		</>
	);
}

// SavedConfirm is "I saved the codes" and the button it unlocks.
export function SavedConfirm({
	onContinue,
	label = m.auth.recovery.continue,
}: {
	onContinue: () => void;
	label?: string;
}) {
	const [saved, setSaved] = useState(false);
	const id = useId();
	return (
		<>
			<div className="mt-6 flex items-center gap-2">
				<Checkbox id={id} checked={saved} onCheckedChange={setSaved} />
				<Label htmlFor={id} className="font-normal">
					{m.auth.recovery.saved}
				</Label>
			</div>
			<Button
				size="lg"
				className="mt-4 w-full"
				disabled={!saved}
				onClick={onContinue}
			>
				{label}
			</Button>
		</>
	);
}
