import { TanStackDevtools } from "@tanstack/react-devtools";
import type { QueryClient } from "@tanstack/react-query";
import {
	createRootRouteWithContext,
	HeadContent,
	Scripts,
} from "@tanstack/react-router";
import { TanStackRouterDevtoolsPanel } from "@tanstack/react-router-devtools";
import { NavProgress } from "#/components/site/nav-progress";
import { ShopLayout } from "#/components/site/shop-layout";
import { NotFound } from "#/components/site/status-page";
import { locale, m } from "#/i18n";
import TanStackQueryDevtools from "../integrations/tanstack-query/devtools";
import appCss from "../styles.css?url";

interface MyRouterContext {
	queryClient: QueryClient;
}

export const Route = createRootRouteWithContext<MyRouterContext>()({
	head: () => ({
		meta: [
			{
				charSet: "utf-8",
			},
			{
				name: "viewport",
				content: "width=device-width, initial-scale=1",
			},
			{
				title: m.site.title(m.site.tagline),
			},
		],
		links: [
			{
				rel: "stylesheet",
				href: appCss,
			},
		],
	}),
	shellComponent: RootDocument,
	// A URL no route matches. Unknown products and categories are handled inside /_shop.
	notFoundComponent: () => (
		<ShopLayout>
			<NotFound />
		</ShopLayout>
	),
});

// Flags Apple devices on <html> before first paint, so shortcut hints show ⌘ there and Ctrl
// everywhere else without the label switching after hydration. The server can't tell.
const platformScript = `if (/Mac|iPhone|iPad/.test(navigator.userAgent)) document.documentElement.dataset.mac = ""`;

function RootDocument({ children }: { children: React.ReactNode }) {
	return (
		<html lang={locale.split("-")[0]} suppressHydrationWarning>
			<head>
				{/* biome-ignore lint/security/noDangerouslySetInnerHtml: static inline script, no user input */}
				<script dangerouslySetInnerHTML={{ __html: platformScript }} />
				<HeadContent />
			</head>
			<body>
				<NavProgress />
				{children}
				<TanStackDevtools
					config={{
						position: "bottom-right",
					}}
					plugins={[
						{
							name: "Tanstack Router",
							render: <TanStackRouterDevtoolsPanel />,
						},
						TanStackQueryDevtools,
					]}
				/>
				<Scripts />
			</body>
		</html>
	);
}
