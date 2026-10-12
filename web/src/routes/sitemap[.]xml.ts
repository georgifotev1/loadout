import { createFileRoute } from "@tanstack/react-router";
import { legalPaths } from "#/components/legal/legal-page";
import { api, unwrap } from "#/lib/api/client";
import { categories } from "#/lib/categories";

// /sitemap.xml lists the home page, every category, the legal pages and every visible product, so search
// engines find products that no listing page links to directly.
export const Route = createFileRoute("/sitemap.xml")({
	server: {
		handlers: {
			GET: async ({ request }) => {
				// SITE_URL is the public origin; behind a proxy the request's own origin may be internal.
				const origin = process.env.SITE_URL || new URL(request.url).origin;
				const paths = [
					"/",
					...categories.map((c) => `/c/${c}`),
					...Object.values(legalPaths),
					...(await productSlugs()).map((s) => `/p/${encodeURIComponent(s)}`),
				];
				const urls = paths
					.map((p) => `<url><loc>${escapeXml(origin + p)}</loc></url>`)
					.join("");
				return new Response(
					`<?xml version="1.0" encoding="UTF-8"?><urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">${urls}</urlset>`,
					{
						headers: {
							"Content-Type": "application/xml; charset=utf-8",
							"Cache-Control": "public, max-age=3600",
						},
					},
				);
			},
		},
	},
});

// productSlugs pages through GET /api/products. One sitemap holds up to 50,000 URLs, far
// more than the catalog has.
async function productSlugs(): Promise<string[]> {
	const slugs: string[] = [];
	for (let page = 1; ; page++) {
		const res = unwrap(
			await api.GET("/products", {
				params: { query: { page, page_size: 100 } },
			}),
		);
		slugs.push(...res.products.map((p) => p.slug));
		if (page * res.page_size >= res.total) return slugs;
	}
}

function escapeXml(s: string) {
	return s.replace(
		/[<>&'"]/g,
		(c) =>
			({
				"<": "&lt;",
				">": "&gt;",
				"&": "&amp;",
				"'": "&apos;",
				'"': "&quot;",
			})[c] as string,
	);
}
