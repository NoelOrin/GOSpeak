import userStore from "@/stores/userStore";

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

function decodeJWTPayload<T = Record<string, unknown>>(
	token: string,
): T | null {
	try {
		const payload = token.split(".")[1];
		if (!payload) return null;
		return JSON.parse(atob(payload)) as T;
	} catch {
		return null;
	}
}

/** Bot / 细粒度 token：claims.permissions 非空时优先，与后端 PermissionGranted 口径一致。 */
function claimPermissions(): string[] | null {
	const token = userStore.accessToken();
	if (!token) return null;
	const payload = decodeJWTPayload<{ permissions?: unknown }>(token);
	const perms = payload?.permissions;
	if (!Array.isArray(perms) || perms.length === 0) return null;
	return perms.filter(
		(p): p is string => typeof p === "string" && p.length > 0,
	);
}

export function hasPermission(code: string): boolean {
	const claims = claimPermissions();
	if (claims) return claims.includes(code);

	const role = userStore.user()?.role;
	if (!role) return false;
	return rolePermissions[role]?.includes(code) ?? false;
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
