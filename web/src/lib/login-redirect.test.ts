import { describe, expect, it } from "vitest";
import type { Me } from "#/lib/api/auth";
import { afterLogin } from "./login-redirect";

const admin: Me = {
	id: "00000000-0000-0000-0000-000000000001",
	email: "admin@example.com",
	role: "admin",
	second_factor: "verified",
};
const customer: Me = {
	...admin,
	role: "customer",
	second_factor: "not_required",
};

describe("afterLogin", () => {
	it("follows a path on this site", () => {
		expect(afterLogin(admin, "/admin/products?page=2")).toBe(
			"/admin/products?page=2",
		);
	});

	it("goes to the role's home without a redirect", () => {
		expect(afterLogin(admin)).toBe("/admin");
		expect(afterLogin(customer)).toBe("/");
	});

	it.each([
		"https://evil.example",
		"//evil.example",
		"/\\evil.example",
		"javascript:alert(1)",
		"admin",
	])("ignores %s", (redirect) => {
		expect(afterLogin(admin, redirect)).toBe("/admin");
	});
});
