import { useMutation } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import { Check, Copy, Link2, X } from "lucide-react";
import { useEffect, useState } from "react";
import { Button } from "#/components/ui/button";
import { m } from "#/i18n";
import { saveBuild } from "#/lib/api/builder";
import type { BuildSearch } from "#/lib/builder";

// useShareBuild saves the build and copies its link. Saving again before the build changes
// reuses the link instead of making another one.
export function useShareBuild(build: BuildSearch) {
	const key = JSON.stringify(build);
	const [shared, setShared] = useState<{ key: string; slug: string }>();
	const [copied, setCopied] = useState(false);
	const [open, setOpen] = useState(false);
	const save = useMutation({ mutationFn: saveBuild });

	const link = shared?.key === key ? shared.slug : undefined;

	useEffect(() => {
		if (!copied) return;
		const t = setTimeout(() => setCopied(false), 2000);
		return () => clearTimeout(t);
	}, [copied]);

	async function copy(slug: string) {
		try {
			await navigator.clipboard.writeText(shareUrl(slug));
			setCopied(true);
		} catch {
			// No clipboard access (an http page, a denied permission): the link is on screen to
			// copy by hand.
		}
	}

	async function share() {
		setOpen(true);
		if (link) return copy(link);
		const saved = await save.mutateAsync(build).catch(() => undefined);
		if (!saved) return;
		setShared({ key, slug: saved.slug });
		await copy(saved.slug);
	}

	return {
		share,
		copy,
		link,
		copied,
		open,
		pending: save.isPending,
		failed: save.isError && !link,
		dismiss: () => {
			setOpen(false);
			save.reset();
		},
	};
}

export function shareUrl(slug: string): string {
	return new URL(`/build/${slug}`, window.location.origin).toString();
}

// ShareBar shows the saved build's link once there is one, or why saving failed.
export function ShareBar({
	state,
}: {
	state: ReturnType<typeof useShareBuild>;
}) {
	if (!state.open) return null;
	if (state.failed) {
		return (
			<div className="flex items-center justify-between gap-3 rounded-xl border border-destructive/40 bg-destructive/3 px-4 py-3 text-destructive text-meta">
				{m.builder.shareFailed}
				<DismissButton onClick={state.dismiss} />
			</div>
		);
	}
	if (!state.link) return null;
	const url = shareUrl(state.link);
	return (
		<div className="flex flex-wrap items-center gap-x-4 gap-y-2 rounded-xl border bg-card px-4 py-3">
			<p className="flex items-center gap-2 text-meta text-muted-foreground">
				<Link2 className="size-4" /> {m.builder.shareLink}
			</p>
			<Link
				to="/build/$slug"
				params={{ slug: state.link }}
				className="order-last min-w-0 basis-full truncate font-mono text-meta hover:underline sm:order-0 sm:basis-0 sm:flex-1"
			>
				{url}
			</Link>
			<div className="ml-auto flex items-center gap-1">
				<Button
					variant="outline"
					size="sm"
					onClick={() => state.link && state.copy(state.link)}
				>
					{state.copied ? <Check /> : <Copy />}
					{state.copied ? m.builder.copied : m.builder.copy}
				</Button>
				<DismissButton onClick={state.dismiss} />
			</div>
		</div>
	);
}

function DismissButton({ onClick }: { onClick: () => void }) {
	return (
		<Button
			variant="ghost"
			size="icon-sm"
			aria-label={m.nav.close}
			onClick={onClick}
			className="text-subtle-foreground"
		>
			<X />
		</Button>
	);
}
