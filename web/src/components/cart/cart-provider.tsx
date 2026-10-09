import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { createContext, useContext, useState } from "react";
import { m } from "#/i18n";
import {
	addToCart,
	type Cart,
	type CartAddLine,
	cartQuery,
	emptyCart,
	removeFromCart,
	setCartAssembly,
	setCartQuantity,
} from "#/lib/api/cart";
import { ApiError } from "#/lib/api/client";
import { CartDrawer, type CartNotice } from "./cart-drawer";

type CartContext = {
	// undefined while it loads, and if loading failed (loadFailed).
	cart: Cart | undefined;
	loadFailed: boolean;
	// Adds products and opens the drawer, which says what was added or why it wasn't. With
	// assembly, also asks for the cart's PC to be assembled.
	add: (lines: CartAddLine[], opts?: { assembly?: boolean }) => void;
	adding: boolean;
	// Asks for the cart's PC to be assembled, or no longer.
	setAssembly: (on: boolean) => Promise<Cart>;
	settingAssembly: boolean;
	setQuantity: (productId: string, quantity: number) => void;
	remove: (productId: string) => void;
	// Takes every product out of the cart.
	empty: () => Promise<Cart>;
	emptying: boolean;
	// The product IDs with a change on the way, to disable their controls.
	busy: (productId: string) => boolean;
	error: string | undefined;
	openDrawer: () => void;
};

const Ctx = createContext<CartContext | null>(null);

export function useCart() {
	const cart = useContext(Ctx);
	if (!cart) throw new Error("useCart outside CartProvider");
	return cart;
}

// CartProvider holds the shopper's cart (from the API, fetched in the browser only) and the
// mini cart drawer, so any page can add to the cart and show what happened.
export function CartProvider({ children }: { children: React.ReactNode }) {
	const queryClient = useQueryClient();
	const { data: cart, isError: loadFailed } = useQuery({
		...cartQuery,
		enabled: typeof window !== "undefined",
	});
	const [open, setOpen] = useState(false);
	const [notice, setNotice] = useState<CartNotice>();
	const [error, setError] = useState<string>();

	const settle = (c: Cart) => {
		queryClient.setQueryData(cartQuery.queryKey, c);
		setError(undefined);
	};
	const fail = (err: unknown) => {
		// A line that's gone (another tab removed it, the cookie expired): show the cart as it is.
		if (err instanceof ApiError && err.status === 404) {
			void queryClient.invalidateQueries({ queryKey: cartQuery.queryKey });
		}
		setError(cartError(err));
	};

	const adder = useMutation({
		mutationFn: (v: { lines: CartAddLine[]; assembly?: boolean }) =>
			addToCart(v.lines, v.assembly),
		onSuccess: (c, v) => {
			settle(c);
			setNotice({ added: v.lines.map((l) => l.slug) });
		},
		onError: (err) => setNotice({ error: cartError(err) }),
		onSettled: () => setOpen(true),
	});
	const changer = useMutation({
		mutationFn: (v: { productId: string; quantity: number }) =>
			setCartQuantity(v.productId, v.quantity),
		onSuccess: settle,
		onError: fail,
	});
	const assembler = useMutation({
		mutationFn: setCartAssembly,
		onSuccess: settle,
		onError: fail,
	});
	const remover = useMutation({
		mutationFn: removeFromCart,
		onSuccess: settle,
		onError: fail,
	});
	const emptier = useMutation({
		mutationFn: emptyCart,
		onSuccess: settle,
		onError: fail,
	});

	const value: CartContext = {
		cart,
		loadFailed,
		add: (lines, opts) => adder.mutate({ lines, assembly: opts?.assembly }),
		adding: adder.isPending,
		setAssembly: (on) => assembler.mutateAsync(on),
		settingAssembly: assembler.isPending,
		setQuantity: (productId, quantity) =>
			changer.mutate({ productId, quantity }),
		remove: (productId) => remover.mutate(productId),
		empty: () => emptier.mutateAsync(),
		emptying: emptier.isPending,
		busy: (productId) =>
			emptier.isPending ||
			(changer.isPending && changer.variables?.productId === productId) ||
			(remover.isPending && remover.variables === productId),
		error,
		openDrawer: () => {
			setNotice(undefined);
			setOpen(true);
		},
	};

	return (
		<Ctx.Provider value={value}>
			{children}
			<CartDrawer
				open={open}
				onOpenChange={(o) => {
					setOpen(o);
					if (!o) setError(undefined);
				}}
				notice={notice}
			/>
		</Ctx.Provider>
	);
}

// cartError says why a cart change failed, from the API's field codes.
export function cartError(err: unknown): string {
	if (err instanceof ApiError && err.code === "validation_failed") {
		const codes = err.fields.map((f) => f.code);
		if (codes.includes("unavailable")) return m.cart.errors.unavailable;
		if (codes.includes("out_of_stock")) return m.cart.errors.outOfStock;
		if (codes.includes("too_many") || codes.includes("out_of_range"))
			return m.cart.errors.tooMany;
	}
	return m.cart.errors.generic;
}
