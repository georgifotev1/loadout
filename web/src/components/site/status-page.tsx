import {
	type ErrorComponentProps,
	Link,
	useRouter,
} from "@tanstack/react-router";
import { useEffect } from "react";
import { Button } from "#/components/ui/button";
import { m } from "#/i18n";
import { Container } from "./container";

// NotFound is the 404 page body: an unknown URL, product or category.
export function NotFound() {
	return (
		<StatusPage title={m.notFound.title} detail={m.notFound.detail}>
			<Button size="lg" render={<Link to="/" />}>
				{m.notFound.backHome}
			</Button>
		</StatusPage>
	);
}

// PageError is shown when a page's data couldn't load, usually because the API is down.
// Retrying reruns the loaders, which also clears the error.
export function PageError({ error }: ErrorComponentProps) {
	useLogError(error);
	return (
		<StatusPage title={m.error.title} detail={m.error.detail}>
			<RetryButton />
		</StatusPage>
	);
}

export function RetryButton() {
	const router = useRouter();
	return (
		<Button size="lg" onClick={() => router.invalidate()}>
			{m.error.retry}
		</Button>
	);
}

// useLogError puts the error the page hides behind "something went wrong" in the browser
// console, for whoever is debugging it.
export function useLogError(error: unknown) {
	useEffect(() => {
		console.error(error);
	}, [error]);
}

export function StatusPage({
	title,
	detail,
	children,
}: {
	title: string;
	detail: string;
	children: React.ReactNode;
}) {
	return (
		<Container className="flex flex-col items-start py-20 lg:py-28">
			<h1 className="font-semibold text-3xl tracking-[-0.03em]">{title}</h1>
			<p className="mt-3 max-w-lg text-muted-foreground">{detail}</p>
			<div className="mt-8">{children}</div>
		</Container>
	);
}
