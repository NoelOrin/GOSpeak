import type { AxiosResponse } from "axios";
import type { Result } from "./apiClient";
import apiClient from "./apiClient";

export interface Domain {
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

export interface DomainMember {
	id: number;
	domain_uuid: string;
	user_uuid: string;
	nickname: string;
	role_name: string;
	joined_at: string;
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

function unwrap<T>(res: AxiosResponse<Result<T>>): T {
	const data = res.data.data;
	if (data == null) throw new Error("domain data is missing");
	return data;
}

export async function createDomain(data: {
	name: string;
	description?: string;
	is_public?: boolean;
}): Promise<Domain> {
	const res = await apiClient.post({
		url: "/api/v1/domain/create",
		data,
	});
	return unwrap<Domain>(res as AxiosResponse<Result<Domain>>);
}

export async function getDomain(uuid: string): Promise<Domain> {
	const res = await apiClient.post({
		url: "/api/v1/domain/get",
		data: { domain_uuid: uuid },
	});
	return unwrap<Domain>(res as AxiosResponse<Result<Domain>>);
}

export async function listDomains(
	page = 1,
	pageSize = 20,
): Promise<DomainPage> {
	const res = await apiClient.post({
		url: "/api/v1/domain/list",
		data: { page, page_size: pageSize },
	});
	return unwrap<DomainPage>(res as AxiosResponse<Result<DomainPage>>);
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
	const res = await apiClient.post({
		url: "/api/v1/domain/list-public",
		data,
	});
	return unwrap<DomainPage>(res as AxiosResponse<Result<DomainPage>>);
}

export async function myDomains(): Promise<string[]> {
	const res = await apiClient.post({
		url: "/api/v1/domain/my-domains",
		data: {},
	});
	const data = (res as AxiosResponse<Result<{ domain_uuids: string[] }>>).data
		.data;
	return data?.domain_uuids ?? [];
}

export async function previewDomainInvite(inviteCode: string): Promise<Domain> {
	const res = await apiClient.post({
		url: "/api/v1/domain/preview",
		data: { invite_code: inviteCode },
	});
	return unwrap<Domain>(res as AxiosResponse<Result<Domain>>);
}

export async function joinDomain(inviteCode: string): Promise<Domain> {
	const res = await apiClient.post({
		url: "/api/v1/domain/join",
		data: { invite_code: inviteCode },
	});
	return unwrap<Domain>(res as AxiosResponse<Result<Domain>>);
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
	max_rooms?: number;
}): Promise<Domain> {
	const res = await apiClient.post({
		url: "/api/v1/domain/update",
		data,
	});
	return unwrap<Domain>(res as AxiosResponse<Result<Domain>>);
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
	const res = await apiClient.post({
		url: "/api/v1/domain/members",
		data: { domain_uuid: domainUUID },
	});
	const data = (res as AxiosResponse<Result<{ members: DomainMember[] }>>).data
		.data;
	return data?.members ?? [];
}
