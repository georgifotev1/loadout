import { StatusDot } from "#/components/admin/admin-page";
import { m } from "#/i18n";
import type { SyncRun } from "#/lib/api/admin";

const tone = {
	running: "muted",
	succeeded: "success",
	failed: "destructive",
} as const;

export function SyncStatus({ run }: { run: Pick<SyncRun, "status"> }) {
	return (
		<StatusDot tone={tone[run.status]}>
			{m.admin.sync.statuses[run.status]}
		</StatusDot>
	);
}
