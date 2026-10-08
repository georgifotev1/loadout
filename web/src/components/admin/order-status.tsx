import { StatusDot } from "#/components/admin/admin-page";
import { m } from "#/i18n";
import type { OrderStatus } from "#/lib/api/orders";

const t = m.admin.orders;

// Waiting for an admin is a warning; waiting too long, destructive.
const tones: Record<
	OrderStatus,
	"success" | "warning" | "destructive" | "muted"
> = {
	awaiting_payment: "muted",
	supplier_pending: "warning",
	ready_to_ship: "success",
	shipped: "success",
	delivered: "success",
	refunding: "warning",
	cancelled: "muted",
};

export function OrderStatusDot({
	status,
	stale = false,
}: {
	status: OrderStatus;
	stale?: boolean;
}) {
	return (
		<StatusDot tone={stale ? "destructive" : tones[status]}>
			{t.statuses[status]}
			{stale ? <span className="text-destructive">· {t.stale}</span> : null}
		</StatusDot>
	);
}
