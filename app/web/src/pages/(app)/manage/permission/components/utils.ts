import type { PermissionItem } from "@/api/permission";

export const DOMAIN_LABELS: Record<string, string> = {
	bot: "BOT",
	room: "房间",
	user: "用户",
	role: "角色",
	signal: "信令",
	sfu: "SFU",
};

export const getDomain = (permission: PermissionItem) =>
	permission.code.split(":")[0] || "other";

export const isDefaultRole = (name: string) =>
	name === "admin" || name === "user" || name === "ban";
