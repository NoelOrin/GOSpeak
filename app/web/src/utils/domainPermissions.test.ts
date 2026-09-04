import { describe, expect, it, vi, beforeEach } from "vitest";

vi.mock("@/stores/domainStore", () => ({
	default: {
		state: {
			myRolePermissions: {
				"g-1": ["room:delete"],
			},
		},
	},
}));

vi.mock("@/utils/permissions", () => ({
	hasPermission: (code: string) => code === "domain:manage",
}));

import { hasDomainPermission } from "./domainPermissions";

describe("hasDomainPermission", () => {
	beforeEach(() => {
		vi.clearAllMocks();
	});

	it("falls through to global permission", () => {
		expect(hasDomainPermission("g-1", "domain:manage")).toBe(true);
	});

	it("checks cached domain role permissions", () => {
		expect(hasDomainPermission("g-1", "room:delete")).toBe(true);
		expect(hasDomainPermission("g-1", "room:read")).toBe(false);
	});
});
