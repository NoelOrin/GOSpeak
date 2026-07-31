import type { AxiosResponse } from "axios";
import type { Result } from "./apiClient";
import apiClient from "./apiClient";

export interface Guild {
	id: number;
	uuid: string;
	name: string;
	icon_url: string;
	description: string;
	owner_uuid: string;
	invite_code: string;
	max_rooms: number;
	is_public: boolean;
	created_at: string;
}

export interface GuildMember {
	id: number;
	guild_uuid: string;
	user_uuid: string;
	nickname: string;
	role_name: string;
	joined_at: string;
}

function unwrap<T>(res: AxiosResponse<Result<T>>): T {
	const data = res.data.data;
	if (data == null) throw new Error("guild data is missing");
	return data;
}

export async function createGuild(data: {
	name: string;
	description?: string;
	is_public?: boolean;
}): Promise<Guild> {
	const res = await apiClient.post({
		url: "/api/v1/guild/create",
		data,
	});
	return unwrap<Guild>(res as AxiosResponse<Result<Guild>>);
}

export async function getGuild(uuid: string): Promise<Guild> {
	const res = await apiClient.post({
		url: "/api/v1/guild/get",
		data: { uuid },
	});
	return unwrap<Guild>(res as AxiosResponse<Result<Guild>>);
}

export async function listGuilds(
	page = 1,
	pageSize = 20,
): Promise<{ guilds: Guild[]; total: number }> {
	const res = await apiClient.post({
		url: "/api/v1/guild/list",
		data: { page, page_size: pageSize },
	});
	return unwrap<{ guilds: Guild[]; total: number }>(
		res as AxiosResponse<Result<{ guilds: Guild[]; total: number }>>,
	);
}

export async function listPublicGuilds(
	page = 1,
	pageSize = 20,
	keyword?: string,
): Promise<{ guilds: Guild[]; total: number }> {
	const data: { page: number; page_size: number; keyword?: string } = {
		page,
		page_size: pageSize,
	};
	if (keyword?.trim()) {
		data.keyword = keyword.trim();
	}
	const res = await apiClient.post({
		url: "/api/v1/guild/list-public",
		data,
	});
	return unwrap<{ guilds: Guild[]; total: number }>(
		res as AxiosResponse<Result<{ guilds: Guild[]; total: number }>>,
	);
}

export async function myGuilds(): Promise<string[]> {
	const res = await apiClient.post({
		url: "/api/v1/guild/my-guilds",
		data: {},
	});
	const data = (res as AxiosResponse<Result<{ guild_uuids: string[] }>>).data
		.data;
	return data?.guild_uuids ?? [];
}

export async function previewGuildInvite(inviteCode: string): Promise<Guild> {
	const res = await apiClient.post({
		url: "/api/v1/guild/preview",
		data: { invite_code: inviteCode },
	});
	return unwrap<Guild>(res as AxiosResponse<Result<Guild>>);
}

export async function joinGuild(inviteCode: string): Promise<Guild> {
	const res = await apiClient.post({
		url: "/api/v1/guild/join",
		data: { invite_code: inviteCode },
	});
	return unwrap<Guild>(res as AxiosResponse<Result<Guild>>);
}

export async function leaveGuild(uuid: string): Promise<void> {
	await apiClient.post({
		url: "/api/v1/guild/leave",
		data: { uuid },
	});
}

export async function updateGuild(data: {
	uuid: string;
	name?: string;
	description?: string;
	icon_url?: string;
	is_public?: boolean;
	max_rooms?: number;
}): Promise<Guild> {
	const res = await apiClient.post({
		url: "/api/v1/guild/update",
		data,
	});
	return unwrap<Guild>(res as AxiosResponse<Result<Guild>>);
}

export async function deleteGuild(uuid: string): Promise<void> {
	await apiClient.post({
		url: "/api/v1/guild/delete",
		data: { uuid },
	});
}

export async function kickGuildMember(
	guildUUID: string,
	userUUID: string,
): Promise<void> {
	await apiClient.post({
		url: "/api/v1/guild/kick",
		data: { guild_uuid: guildUUID, user_uuid: userUUID },
	});
}

export async function guildMembers(guildUUID: string): Promise<GuildMember[]> {
	const res = await apiClient.post({
		url: "/api/v1/guild/members",
		data: { guild_uuid: guildUUID },
	});
	const data = (res as AxiosResponse<Result<{ members: GuildMember[] }>>).data
		.data;
	return data?.members ?? [];
}
