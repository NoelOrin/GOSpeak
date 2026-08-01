import { describe, expect, it, vi } from "vitest";

vi.mock("idb-keyval", () => ({
	get: vi.fn(async () => undefined),
	set: vi.fn(async () => {}),
	del: vi.fn(async () => {}),
}));
import { rolePermissions } from "./permissions";

const guildPermissions = [
	"guild:create",
	"guild:read",
	"guild:manage",
	"guild:delete",
	"guild:invite",
	"guild:kick",
	"guild:role:manage",
];

describe("rolePermissions", () => {
	it("admin has all guild permissions", () => {
		for (const code of guildPermissions) {
			expect(rolePermissions.admin).toContain(code);
		}
	});

	it("keeps guild management permissions in the admin fallback", () => {
		expect(rolePermissions.admin).toEqual(
			expect.arrayContaining([
				"guild:manage",
				"guild:delete",
				"guild:invite",
				"guild:kick",
			]),
		);
	});

	it("user has create guild permission", () => {
		expect(rolePermissions.user).toContain("guild:create");
	});
});
