import { beforeEach, describe, expect, it } from "vitest";
import { useUserStore, type UserInfo } from "@/stores/userStore";
import {
	MANAGE_ENTRY_PERMISSIONS,
	hasManageAccess,
	hasPermission,
	rolePermissions,
} from "./permissions";

function setUser(overrides: Partial<UserInfo>) {
	useUserStore.setState({
		user: {
			id: 1,
			uuid: "u-1",
			name: "alice",
			display_name: "Alice",
			avatar: "",
			role: "user",
			...overrides,
		},
		userStale: false,
	});
}

describe("fallback role permission table", () => {
	beforeEach(() => {
		useUserStore.setState({ user: null, userStale: false });
	});

	it("keeps manage-only codes out of the plain user role", () => {
		expect(rolePermissions.user).not.toContain("user:read");
		expect(rolePermissions.user).not.toContain("role:read");
		expect(rolePermissions.user).toContain("domain:create");
	});

	it("does not expose the manage entry to a plain user", () => {
		setUser({ role: "user" });
		expect(hasPermission("user:read")).toBe(false);
		expect(hasPermission("role:read")).toBe(false);
		expect(hasManageAccess()).toBe(false);
		expect(MANAGE_ENTRY_PERMISSIONS.some((code) => hasPermission(code))).toBe(
			false,
		);
	});

	it("still lets a plain user create rooms and domains", () => {
		setUser({ role: "user" });
		expect(hasPermission("room:create")).toBe(true);
		expect(hasPermission("domain:create")).toBe(true);
		expect(hasPermission("message:send")).toBe(true);
	});

	it("keeps admin able to enter the manage shell", () => {
		setUser({ role: "admin" });
		expect(hasPermission("user:read")).toBe(true);
		expect(hasManageAccess()).toBe(true);
	});

	it("denies everything to a banned user", () => {
		setUser({ role: "ban" });
		expect(hasManageAccess()).toBe(false);
		expect(hasPermission("room:read")).toBe(false);
	});

	it("treats server-provided profile permissions as authoritative", () => {
		setUser({ role: "user", permissions: ["user:read"] });
		expect(hasPermission("user:read")).toBe(true);
		expect(hasPermission("domain:create")).toBe(false);
	});
});
