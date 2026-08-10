import userStore from "@/stores/userStore";

/** 解码 JWT base64url payload；兼容 UTF-8 与 URL-safe 字符集。 */
export function decodeBase64Url(input: string): string {
	const base64 = input.replace(/-/g, "+").replace(/_/g, "/");
	const padded = base64.padEnd(Math.ceil(base64.length / 4) * 4, "=");
	const raw = atob(padded);
	const bytes = Uint8Array.from(raw, (ch) => ch.charCodeAt(0));
	return new TextDecoder().decode(bytes);
}

/** 与后端 model.DefaultRolePermissions 保持同步（前端兜底；DB 动态权限以服务端为准）。 */
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
		"bot:manage",
		"domain:create",
		"domain:read",
		"domain:manage",
		"domain:delete",
		"domain:invite",
		"domain:kick",
		"domain:role:manage",
		"email_config:read",
		"email_config:manage",
		"storage:read",
		"storage:manage",
		"storage:delete",
		"oauth:read",
		"oauth:manage",
		"plugin:read",
		"plugin:manage",
		"message:send",
		"message:read",
		"message:delete_others",
		"cluster:read",
		"cluster:manage",
	],
	user: [
		"room:create",
		"room:read",
		"domain:create",
		"user:read",
		"role:read",
		"message:send",
		"message:read",
	],
	ban: [],
};

/** 可进入 /manage 壳层的任一权限（页面内再做细粒度守卫）。 */
export const MANAGE_ENTRY_PERMISSIONS = [
	"user:read",
	"user:update",
	"user:delete",
	"role:read",
	"role:manage",
	"mute:manage",
	"sfu:manage",
	"bot:manage",
	"plugin:read",
	"plugin:manage",
	"email_config:read",
	"email_config:manage",
	"storage:read",
	"storage:manage",
	"oauth:read",
	"oauth:manage",
] as const;

export function hasPermission(code: string): boolean {
	// 被封禁用户一律无权限，即使旧 token 仍携带 permissions claims。
	if (isBannedUser()) return false;

	// access token 已移入 HttpOnly Cookie，前端无法解码 claims；
	// 服务端 profile 下发的权限是权威来源，未加载时退回本地兜底表。
	const profilePerms = userStore.user()?.permissions;
	if (profilePerms && profilePerms.length > 0) {
		return profilePerms.includes(code);
	}

	const role = userStore.user()?.role;
	if (!role) return false;
	return rolePermissions[role]?.includes(code) ?? false;
}

function isBannedUser(): boolean {
	return userStore.user()?.role === "ban";
}

export function hasAnyPermission(...codes: string[]): boolean {
	return codes.some((code) => hasPermission(code));
}

export function hasManageAccess(): boolean {
	return hasAnyPermission(...MANAGE_ENTRY_PERMISSIONS);
}

export function requirePermission(code: string): void {
	if (!hasPermission(code)) {
		throw new Error("FORBIDDEN");
	}
}

export { rolePermissions };
