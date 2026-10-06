import { type ErrorComponentProps, Link } from "@tanstack/react-router";
import { RetryButton, useLogError } from "#/components/site/status-page";
import { Button } from "#/components/ui/button";
import { m } from "#/i18n";
import { AdminPage } from "./admin-page";

// The admin's own error and not-found pages, inside the admin layout: the storefront's would
// send an admin back to the shop. The admin routes set them, as the router's defaults are the
// storefront's.

export function AdminError({ error }: ErrorComponentProps) {
	useLogError(error);
	return (
		<AdminPage title={m.error.title} lead={m.error.detail}>
			<RetryButton />
		</AdminPage>
	);
}

export function AdminNotFound() {
	return (
		<AdminPage title={m.admin.notFound.title} lead={m.admin.notFound.detail}>
			<Button size="lg" render={<Link to="/admin" />}>
				{m.admin.backToDashboard}
			</Button>
		</AdminPage>
	);
}
