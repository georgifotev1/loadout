import { createCn } from "cn/config";

// cn joins class names and resolves Tailwind conflicts. It is told about our custom theme
// tokens (styles.css), otherwise `text-meta` reads as a colour and `cn("text-meta",
// "text-muted-foreground")` would drop the size.
export const cn = createCn({
	extend: {
		theme: {
			text: ["meta"],
			spacing: ["header"],
			container: ["page"],
		},
	},
});
