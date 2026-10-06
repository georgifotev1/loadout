import { useRouterState } from "@tanstack/react-router";
import { useEffect, useState } from "react";
import { cn } from "#/lib/utils";

// Shown only if a page takes longer than this, so quick navigations don't flash it.
const showAfterMs = 150;
// It creeps towards this while loading, then jumps to the end when the page is ready.
const ceiling = 0.9;

// NavProgress is the thin bar along the top of the window while the next page loads. The
// router keeps the old page on screen until the new one's data is in, so without it a slow
// click looks like nothing happened.
export function NavProgress() {
	const loading = useRouterState({ select: (s) => s.isLoading });
	const [progress, setProgress] = useState<number>();

	useEffect(() => {
		if (!loading) {
			// Finish the bar if it showed; the effect below then hides it.
			setProgress((p) => (p === undefined ? undefined : 1));
			return;
		}
		let trickle: ReturnType<typeof setInterval>;
		const show = setTimeout(() => {
			setProgress(0.15);
			trickle = setInterval(
				() => setProgress((p = 0) => p + (ceiling - p) * 0.12),
				400,
			);
		}, showAfterMs);
		return () => {
			clearTimeout(show);
			clearInterval(trickle);
		};
	}, [loading]);

	useEffect(() => {
		if (progress !== 1) return;
		const hide = setTimeout(() => setProgress(undefined), 350);
		return () => clearTimeout(hide);
	}, [progress]);

	if (progress === undefined) return null;
	return (
		<div
			aria-hidden
			className="pointer-events-none fixed inset-x-0 top-0 z-[100] h-0.5"
		>
			<div
				className={cn(
					"h-full origin-left bg-brand transition-[transform,opacity] duration-300 ease-out",
					progress === 1 && "opacity-0 delay-150",
				)}
				style={{ transform: `scaleX(${progress})` }}
			/>
		</div>
	);
}
