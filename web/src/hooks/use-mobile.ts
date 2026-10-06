import { useMediaQuery } from "#/hooks/use-media-query";

// useIsMobile is what shadcn's sidebar asks to choose between the sheet (phones) and the fixed
// sidebar. Below Tailwind's md breakpoint; false during SSR, like useMediaQuery.
export function useIsMobile() {
	return useMediaQuery("(max-width: 767px)");
}
