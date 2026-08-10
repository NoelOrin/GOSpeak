import { describe, expect, it, vi } from "vitest";

vi.mock("idb-keyval", () => ({
	get: vi.fn(async () => undefined),
	set: vi.fn(async () => {}),
	del: vi.fn(async () => {}),
}));

vi.mock("@/stores/userStore", () => ({
	default: {
		user: () => ({ role: "ban", permissions: ["room:create", "user:read"] }),
	},
}));

import {
	decodeBase64Url,
	hasManageAccess,
	hasPermission,
	rolePermissions,
} from "./permissions";

const domainPermissions = [
	"domain:create",
	"domain:read",
	"domain:manage",
	"domain:delete",
	"domain:invite",
	"domain:kick",
	"domain:role:manage",
];

describe("rolePermissions", () => {
	it("admin has all domain permissions", () => {
		for (const code of domainPermissions) {
			expect(rolePermissions.admin).toContain(code);
		}
	});

	it("keeps domain management permissions in the admin fallback", () => {
		expect(rolePermissions.admin).toEqual(
			expect.arrayContaining([
				"domain:manage",
				"domain:delete",
				"domain:invite",
				"domain:kick",
			]),
		);
	});

	it("user has create domain permission", () => {
		expect(rolePermissions.user).toContain("domain:create");
	});

	it("admin has message, plugin and cluster permissions", () => {
		for (const code of [
			"message:send",
			"message:read",
			"message:delete_others",
			"plugin:read",
			"plugin:manage",
			"cluster:read",
			"cluster:manage",
		]) {
			expect(rolePermissions.admin).toContain(code);
		}
	});

	it("user has message permissions", () => {
		expect(rolePermissions.user).toContain("message:send");
		expect(rolePermissions.user).toContain("message:read");
	});
});

describe("decodeBase64Url", () => {
	it("decodes base64url payload containing URL-safe characters", () => {
		const payload = JSON.stringify({
			sub: "user",
			role: "admin",
			permissions: ["room:create"],
		});
		const b64 = btoa(payload)
			.replace(/\+/g, "-")
			.replace(/\//g, "_")
			.replace(/=+$/, "");
		expect(decodeBase64Url(b64)).toBe(payload);
	});

	it("decodes unicode payload", () => {
		const payload = JSON.stringify({ name: "管理员" });
		const b64 = btoa(unescape(encodeURIComponent(payload)))
			.replace(/\+/g, "-")
			.replace(/\//g, "_")
			.replace(/=+$/, "");
		expect(decodeBase64Url(b64)).toBe(payload);
	});
});

describe("banned user", () => {
	it("has no permissions even when role permissions or claims exist", () => {
		expect(hasPermission("room:create")).toBe(false);
		expect(hasPermission("user:read")).toBe(false);
		expect(hasManageAccess()).toBe(false);
	});
});
