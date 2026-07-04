import userStore from "@/stores/userStore";

/** 匹配后端 server/internal/model/permission.go DefaultRolePermissions */
const rolePermissions: Record<string, string[]> = {
	admin: [
		"room:create",
		"room:read",
		"room:update",
		"room:delete",
		"user:read",
		"user:update",
		"user:delete",
		"role:read",
		"role:manage",
		"signal:kick",
		"mute:manage",
		"sfu:manage",
	],
	user: ["room:create", "room:read", "user:read", "role:read"],
	ban: [],
};

export function hasPermission(code: string): boolean {
	const role = userStore.user()?.role;
	if (!role) return false;
	return rolePermissions[role]?.includes(code) ?? false;
}

export { rolePermissions };
