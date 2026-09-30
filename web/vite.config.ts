import tailwindcss from "@tailwindcss/vite";
import { devtools } from "@tanstack/devtools-vite";
import { tanstackStart } from "@tanstack/react-start/plugin/vite";
import viteReact from "@vitejs/plugin-react";
import { defineConfig } from "vite";
import { apiInternalUrl } from "./src/lib/api/client";

// The browser only ever talks to one origin: in dev, /api/* is proxied to the Go API
// (in production the load balancer does the same).
const config = defineConfig({
	resolve: { tsconfigPaths: true },
	server: {
		proxy: {
			"/api": { target: apiInternalUrl(), changeOrigin: false },
		},
	},
	plugins: [devtools(), tailwindcss(), tanstackStart(), viteReact()],
});

export default config;
