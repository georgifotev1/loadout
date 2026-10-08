import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useId, useState } from "react";
import { Button } from "#/components/ui/button";
import {
	Dialog,
	DialogClose,
	DialogContent,
	DialogDescription,
	DialogFooter,
	DialogHeader,
	DialogTitle,
	DialogTrigger,
} from "#/components/ui/dialog";
import { Label } from "#/components/ui/label";
import { Textarea } from "#/components/ui/textarea";
import { m } from "#/i18n";
import {
	type AdminOrder,
	type AdminOrderEvent,
	adminOrderQuery,
	applyOrderEvent,
} from "#/lib/api/admin-orders";
import { ApiError } from "#/lib/api/client";

const t = m.admin.orders;

// OrderActions are the order's actions as the API lists them, each behind a confirmation.
export function OrderActions({ order }: { order: AdminOrder }) {
	if (order.actions.length === 0) return null;
	return (
		<div className="flex flex-wrap gap-2">
			{order.actions.map((ev) => (
				<ActionDialog key={ev} order={order} event={ev} />
			))}
		</div>
	);
}

// What's done, the most likely first; and what can't be undone, red.
const triggerVariant: Record<AdminOrderEvent, "default" | "outline"> = {
	supplier_confirmed: "default",
	supplier_rejected: "outline",
	cancel: "outline",
	refund_succeeded: "outline",
};
const destructive: AdminOrderEvent[] = ["supplier_rejected", "cancel"];

function ActionDialog({
	order,
	event,
}: {
	order: AdminOrder;
	event: AdminOrderEvent;
}) {
	const queryClient = useQueryClient();
	const [open, setOpen] = useState(false);
	const [note, setNote] = useState("");
	const noteId = useId();
	const noteRequired = event === "refund_succeeded";

	const apply = useMutation({
		mutationFn: () => applyOrderEvent(order.id, { event, note }),
		onSuccess: (o) => {
			queryClient.setQueryData(adminOrderQuery(order.id).queryKey, o);
			queryClient.invalidateQueries({ queryKey: ["admin", "orders", "list"] });
			queryClient.invalidateQueries({ queryKey: ["admin", "overview"] });
			setOpen(false);
		},
		onError: (err) => {
			// Someone or something else moved the order: show where it is now.
			if (err instanceof ApiError && err.code === "invalid_transition") {
				queryClient.invalidateQueries({
					queryKey: adminOrderQuery(order.id).queryKey,
				});
			}
		},
	});

	const error = apply.error;
	const noteInvalid =
		error instanceof ApiError &&
		error.fields.some((f) => f.field === "note" && f.code === "required");

	return (
		<Dialog
			open={open}
			onOpenChange={(o) => {
				setOpen(o);
				if (o) {
					setNote("");
					apply.reset();
				}
			}}
		>
			<DialogTrigger
				render={<Button variant={triggerVariant[event]} size="sm" />}
			>
				{t.actions[event]}
			</DialogTrigger>
			<DialogContent>
				<form
					className="flex flex-col gap-4"
					onSubmit={(e) => {
						e.preventDefault();
						apply.mutate();
					}}
				>
					<DialogHeader>
						<DialogTitle>{t.confirm[event].title}</DialogTitle>
						<DialogDescription>{t.confirm[event].body}</DialogDescription>
					</DialogHeader>
					<div className="flex flex-col gap-2">
						<Label htmlFor={noteId}>{t.note}</Label>
						<Textarea
							id={noteId}
							value={note}
							onChange={(e) => setNote(e.target.value)}
							maxLength={500}
							required={noteRequired}
							aria-invalid={noteInvalid || undefined}
							aria-describedby={`${noteId}-hint`}
							placeholder={
								noteRequired
									? t.noteRefundPlaceholder
									: event === "supplier_confirmed"
										? t.noteConfirmPlaceholder
										: undefined
							}
						/>
						<p id={`${noteId}-hint`} className="text-muted-foreground text-xs">
							{noteInvalid ? t.noteRequired : t.noteHint}
						</p>
					</div>
					{error && !noteInvalid ? (
						<p role="alert" className="text-destructive text-sm">
							{error instanceof ApiError && error.code === "invalid_transition"
								? t.changed
								: t.failed}
						</p>
					) : null}
					<DialogFooter>
						<DialogClose render={<Button variant="outline" />}>
							{t.cancelDialog}
						</DialogClose>
						<Button
							type="submit"
							variant={destructive.includes(event) ? "destructive" : "default"}
							disabled={apply.isPending || (noteRequired && !note.trim())}
						>
							{t.actions[event]}
						</Button>
					</DialogFooter>
				</form>
			</DialogContent>
		</Dialog>
	);
}
