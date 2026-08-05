// Bot 可被授予的权限码白名单，须与后端 model.BotScopedPermissions 保持一致。
export const BOT_ALLOWED_PERMISSION_CODES = [
	"room:read",
	"user:read",
	"signal:kick",
	"mute:manage",
];

import apiClient from "./apiClient";

export interface BotAPIKey {
	id: number;
	uuid: string;
	name: string;
	permissions: string[];
	user_uuid: string;
	revoked: boolean;
	expires_at: string;
	created_at: string;
	updated_at: string;
}

export interface CreateBotKeyInput {
	name: string;
	permissions: string[];
	expires_in?: string;
}

export interface CreateBotKeyResult {
	token: string;
	token_uuid: string;
	permissions: string[];
	user: {
		id: number;
		uuid: string;
		name: string;
		display_name: string;
		role: string;
	};
	permanent: boolean;
	expires_at?: string;
}

export function createBotKey(
	input: CreateBotKeyInput,
): Promise<CreateBotKeyResult> {
	return apiClient.post<CreateBotKeyResult>({
		url: "/api/v1/bot/create",
		data: input,
	});
}

export function listBotKeys(): Promise<BotAPIKey[]> {
	return apiClient.post<BotAPIKey[]>({ url: "/api/v1/bot/list" });
}

export function revokeBotKey(uuid: string): Promise<null> {
	return apiClient.post<null>({
		url: "/api/v1/bot/revoke",
		data: { uuid },
	});
}
