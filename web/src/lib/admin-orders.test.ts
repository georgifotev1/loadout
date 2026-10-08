import { describe, expect, it } from "vitest";
import { ordersParams, parseOrdersSearch } from "./admin-orders";

describe("parseOrdersSearch", () => {
	it("keeps valid params", () => {
		expect(
			parseOrdersSearch({ q: " 1001 ", view: "needs_action", page: "2" }),
		).toEqual({ q: "1001", view: "needs_action", page: 2 });
		expect(parseOrdersSearch({ view: "refunding" })).toEqual({
			view: "refunding",
		});
	});

	it("drops malformed and default params", () => {
		expect(parseOrdersSearch({ q: "  ", view: "lost", page: "1" })).toEqual({});
	});
});

describe("ordersParams", () => {
	it("maps the view to the API's filters", () => {
		expect(ordersParams({ view: "needs_action", page: 2 })).toEqual({
			needs_action: true,
			page: 2,
			page_size: 50,
		});
		expect(ordersParams({ q: "ivan", view: "shipped" })).toEqual({
			q: "ivan",
			status: "shipped",
			page_size: 50,
		});
		expect(ordersParams({})).toEqual({ page_size: 50 });
	});
});
