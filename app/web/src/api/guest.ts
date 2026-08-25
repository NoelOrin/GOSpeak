import apiClient from "./apiClient";

export interface GuestUser {
	id: number;
	uuid: string;
	name: string;
	display_name: string;
	avatar: string;
	role: string;
	is_guest?: boolean;
}

export interface GuestDomain {
	id: number;
	uuid: string;
	name: string;
	icon_url?: string;
	description?: string;
	allow_guest?: boolean;
	guest_can_listen?: boolean;
	guest_can_speak?: boolean;
	guest_can_message?: boolean;
	guest_limit?: number;
}

export interface GuestJoinData {
	access_token: string;
	refresh_token: string;
	user: GuestUser;
	domain: GuestDomain;
}

export interface GuestJoinReq {
	nickname: string;
	invite_code?: string;
	domain_uuid?: string;
}

export interface DomainGuestConfig {
	domain_uuid: string;
	allow_guest: boolean;
	guest_can_listen: boolean;
	guest_can_speak: boolean;
	guest_can_message: boolean;
	guest_limit: number;
}

export interface GuestBanItem {
	id: number;
	domain_uuid: string;
	user_uuid: string;
	reason: string;
	banned_by: string;
	created_at: string;
	expires_at?: string | null;
}

export async function guestJoin(req: GuestJoinReq): Promise<GuestJoinData> {
	const data = await apiClient.post<GuestJoinData>({
		url: "/api/v1/auth/guest",
		data: req,
	});
	if (!data) throw new Error("guest join data is missing");
	return data;
}

export async function guestRenew(
	req: Omit<GuestJoinReq, "nickname">,
): Promise<GuestJoinData> {
	const data = await apiClient.post<GuestJoinData>({
		url: "/api/v1/auth/guest/renew",
		data: req,
	});
	if (!data) throw new Error("guest renew data is missing");
	return data;
}

export async function getGuestConfig(domainUUID: string): Promise<GuestDomain> {
	const data = await apiClient.post<GuestDomain>({
		url: "/api/v1/domain/guest/config",
		data: { domain_uuid: domainUUID },
	});
	if (!data) throw new Error("guest config is missing");
	return data;
}

export async function updateGuestConfig(
	payload: Partial<
		Pick<
			DomainGuestConfig,
			| "allow_guest"
			| "guest_can_listen"
			| "guest_can_speak"
			| "guest_can_message"
			| "guest_limit"
		>
	> & { domain_uuid: string },
): Promise<GuestDomain> {
	const data = await apiClient.post<GuestDomain>({
		url: "/api/v1/domain/guest/config",
		data: payload,
	});
	if (!data) throw new Error("guest config update is missing");
	return data;
}

export async function banGuest(payload: {
	domain_uuid: string;
	user_uuid: string;
	reason?: string;
	duration_hours?: number;
}): Promise<void> {
	await apiClient.post({
		url: "/api/v1/domain/guest/ban",
		data: payload,
	});
}

export async function unbanGuest(payload: {
	domain_uuid: string;
	user_uuid: string;
}): Promise<void> {
	await apiClient.post({
		url: "/api/v1/domain/guest/unban",
		data: payload,
	});
}

export async function listGuestBans(
	domainUUID: string,
): Promise<GuestBanItem[]> {
	const data = await apiClient.post<GuestBanItem[]>({
		url: "/api/v1/domain/guest/ban-list",
		data: { domain_uuid: domainUUID },
	});
	return data ?? [];
}

export async function cleanupInactiveGuests(payload: {
	domain_uuid: string;
	days: number;
}): Promise<{ removed: number }> {
	const data = await apiClient.post<{ removed: number }>({
		url: "/api/v1/domain/guest/cleanup",
		data: payload,
	});
	return data ?? { removed: 0 };
}
