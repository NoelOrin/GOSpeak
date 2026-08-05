import apiClient from "./apiClient";

export interface Domain {
	id: number;
	uuid: string;
	name: string;
	icon_url: string;
	description: string;
	owner_uuid: string;
	invite_code: string;
	is_public: boolean;
	created_at: string;
}

export interface DomainDetail extends Domain {
	member_count: number;
	room_count: number;
}

export interface DomainMember {
	id: number;
	domain_uuid: string;
	user_uuid: string;
	nickname: string;
	role_name: string;
	joined_at: string;
	name: string;
	display_name: string;
}

export interface DomainPage {
	domains: Domain[];
	total: number;
}

export interface DomainApiError {
	code?: number;
	msg?: string;
}

export type DomainMutation =
	| "create"
	| "update"
	| "delete"
	| "join"
	| "leave"
	| "kick";

export async function createDomain(data: {
	name: string;
	description?: string;
	is_public?: boolean;
}): Promise<Domain> {
	const result = await apiClient.post<Domain>({
		url: "/api/v1/domain/create",
		data,
	});
	if (!result) throw new Error("domain data is missing");
	return result;
}

export async function getDomain(uuid: string): Promise<Domain> {
	const result = await apiClient.post<Domain>({
		url: "/api/v1/domain/get",
		data: { domain_uuid: uuid },
	});
	if (!result) throw new Error("domain data is missing");
	return result;
}

export async function listDomains(
	page = 1,
	pageSize = 20,
): Promise<DomainPage> {
	const result = await apiClient.post<DomainPage>({
		url: "/api/v1/domain/list",
		data: { page, page_size: pageSize },
	});
	if (!result) throw new Error("domain data is missing");
	return result;
}

export async function listPublicDomains(
	page = 1,
	pageSize = 20,
	keyword?: string,
): Promise<DomainPage> {
	const data: { page: number; page_size: number; keyword?: string } = {
		page,
		page_size: pageSize,
	};
	if (keyword?.trim()) {
		data.keyword = keyword.trim();
	}
	const result = await apiClient.post<DomainPage>({
		url: "/api/v1/domain/list-public",
		data,
	});
	if (!result) throw new Error("domain data is missing");
	return result;
}

export async function getMyDomainsDetailed(): Promise<DomainDetail[]> {
	const result = await apiClient.post<DomainDetail[]>({
		url: "/api/v1/domain/my-domains",
		data: {},
	});
	if (!result) throw new Error("domain data is missing");
	return result;
}

export async function myDomains(): Promise<string[]> {
	const details = await getMyDomainsDetailed();
	return details.map((detail) => detail.uuid);
}

export async function previewDomainInvite(inviteCode: string): Promise<Domain> {
	const result = await apiClient.post<Domain>({
		url: "/api/v1/domain/preview",
		data: { invite_code: inviteCode },
	});
	if (!result) throw new Error("domain data is missing");
	return result;
}

export async function joinDomain(inviteCode: string): Promise<Domain> {
	const result = await apiClient.post<Domain>({
		url: "/api/v1/domain/join",
		data: { invite_code: inviteCode },
	});
	if (!result) throw new Error("domain data is missing");
	return result;
}

export async function leaveDomain(uuid: string): Promise<void> {
	await apiClient.post({
		url: "/api/v1/domain/leave",
		data: { domain_uuid: uuid },
	});
}

export async function updateDomain(data: {
	domain_uuid: string;
	name?: string;
	description?: string;
	icon_url?: string;
	is_public?: boolean;
}): Promise<Domain> {
	const result = await apiClient.post<Domain>({
		url: "/api/v1/domain/update",
		data,
	});
	if (!result) throw new Error("domain data is missing");
	return result;
}

export async function deleteDomain(uuid: string): Promise<void> {
	await apiClient.post({
		url: "/api/v1/domain/delete",
		data: { domain_uuid: uuid },
	});
}

export async function kickDomainMember(
	domainUUID: string,
	userUUID: string,
): Promise<void> {
	await apiClient.post({
		url: "/api/v1/domain/kick",
		data: { domain_uuid: domainUUID, user_uuid: userUUID },
	});
}

export async function domainMembers(
	domainUUID: string,
): Promise<DomainMember[]> {
	const data = await apiClient.post<{ members: DomainMember[] }>({
		url: "/api/v1/domain/members",
		data: { domain_uuid: domainUUID },
	});
	return data?.members ?? [];
}
