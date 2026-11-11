import apiClient from "@/api/apiClient";

export interface InvitePreview {
	domain: {
		uuid: string;
		name: string;
		icon_url?: string;
		description?: string;
	};
	requires_approval: boolean;
	remaining_uses: number;
}

export interface DomainInvite {
	id: number;
	domain_uuid: string;
	label: string;
	code: string;
	expires_at?: string | null;
	max_uses: number;
	used_count: number;
	disabled_at?: string | null;
	created_by: string;
	created_at: string;
}

export interface InviteRedemption {
	id: number;
	invite_id: number;
	domain_uuid: string;
	user_uuid: string;
	code: string;
	success: boolean;
	fail_reason?: string;
	created_at: string;
}

export async function previewInvite(code: string): Promise<InvitePreview> {
	const result = await apiClient.post<InvitePreview>({
		url: "/api/v1/invite/preview",
		data: { code },
		skipErrorToast: true,
	});
	if (!result) throw new Error("invite preview is missing");
	return result;
}

export async function createInvite(params: {
	domain_uuid: string;
	label?: string;
	expires_in_hours?: number;
	max_uses?: number;
}): Promise<DomainInvite> {
	const result = await apiClient.post<DomainInvite>({
		url: "/api/v1/domain/invites/create",
		data: params,
	});
	if (!result) throw new Error("invite create returned no data");
	return result;
}

export async function listInvites(domainUUID: string): Promise<DomainInvite[]> {
	const result = await apiClient.post<{ invites: DomainInvite[] }>({
		url: "/api/v1/domain/invites/list",
		data: { domain_uuid: domainUUID },
	});
	return result?.invites ?? [];
}

export async function disableInvite(
	domainUUID: string,
	inviteID: number,
): Promise<void> {
	await apiClient.post<null>({
		url: "/api/v1/domain/invites/disable",
		data: { domain_uuid: domainUUID, invite_id: inviteID },
	});
}

export async function listRedemptions(
	domainUUID: string,
	inviteID: number,
	page = 1,
	pageSize = 20,
): Promise<{ items: InviteRedemption[]; total: number }> {
	const result = await apiClient.post<{
		items: InviteRedemption[];
		total: number;
	}>({
		url: "/api/v1/domain/invites/redemptions",
		data: {
			domain_uuid: domainUUID,
			invite_id: inviteID,
			page,
			page_size: pageSize,
		},
	});
	return { items: result?.items ?? [], total: result?.total ?? 0 };
}

/** 邀请码状态：生效中 / 已过期 / 已用完 / 已禁用。 */
export function inviteStatus(
	invite: DomainInvite,
): "active" | "expired" | "exhausted" | "disabled" {
	if (invite.disabled_at) return "disabled";
	if (
		invite.expires_at &&
		new Date(invite.expires_at).getTime() <= Date.now()
	) {
		return "expired";
	}
	if (invite.max_uses > 0 && invite.used_count >= invite.max_uses) {
		return "exhausted";
	}
	return "active";
}
