import apiClient from "./apiClient";

/** 落地页 preview 返回的域摘要（不含邀请码本身）。 */
export interface InvitePreviewDomain {
	uuid: string;
	name: string;
	icon_url: string;
	description: string;
}

export interface InvitePreview {
	domain: InvitePreviewDomain;
	requires_approval: boolean;
	remaining_uses: number;
}

export type InviteJoinStatus = "joined" | "pending";

export interface InviteRegisterInput {
	code: string;
	username: string;
	password: string;
	nickname?: string;
	email?: string;
	email_code?: string;
}

export interface InviteJoinResult {
	access_token?: string;
	refresh_token?: string;
	expires_in?: number;
	user?: {
		id: number;
		uuid: string;
		name: string;
		display_name: string;
		avatar: string;
		role: string;
	};
	domain: InvitePreviewDomain & { is_public?: boolean };
	status: InviteJoinStatus;
}

export interface JoinRequestDTO {
	id: number;
	domain_uuid: string;
	user_uuid: string;
	invite_id: number;
	status: "pending" | "approved" | "rejected";
	reviewed_by: string;
	reviewed_at?: string | null;
	created_at: string;
	updated_at: string;
}

export interface JoinRequestView {
	request: JoinRequestDTO | null;
	status: "" | "pending" | "approved" | "rejected";
	is_member: boolean;
}

export interface JoinRequestListItem extends JoinRequestDTO {
	name: string;
	display_name: string;
}

export interface JoinRequestListResult {
	items: JoinRequestListItem[];
	total: number;
	counts: Record<string, number>;
}

/** 验码并取域信息。邀请码无效是高频可预期错误：页面内就地展示中文提示。 */
export async function previewInvite(code: string): Promise<InvitePreview> {
	const data = await apiClient.post<InvitePreview>({
		url: "/api/v1/invite/preview",
		data: { code },
		skipErrorToast: true,
	});
	if (!data) throw new Error("invite preview is missing");
	return data;
}

/** 内联注册：注册成功即入域或提交申请，并下发 Cookie。 */
export async function registerViaInvite(
	input: InviteRegisterInput,
): Promise<InviteJoinResult> {
	const data = await apiClient.post<InviteJoinResult>({
		url: "/api/v1/invite/register",
		data: {
			code: input.code,
			username: input.username,
			password: input.password,
			nickname: input.nickname ?? "",
			email: input.email ?? "",
			email_code: input.email_code ?? "",
		},
		skipErrorToast: true,
	});
	if (!data) throw new Error("invite register response is missing");
	return data;
}

/** 已登录用户凭邀请码加入。 */
export async function joinByCode(code: string): Promise<InviteJoinResult> {
	const data = await apiClient.post<InviteJoinResult>({
		url: "/api/v1/invite/join",
		data: { code },
		skipErrorToast: true,
	});
	if (!data) throw new Error("invite join response is missing");
	return data;
}

/** 已登录用户免码申请加入公开域（仍受域级审核约束）。 */
export async function joinByDomain(
	domainUUID: string,
): Promise<InviteJoinResult> {
	const data = await apiClient.post<InviteJoinResult>({
		url: "/api/v1/invite/join",
		data: { domain_uuid: domainUUID },
		skipErrorToast: true,
	});
	if (!data) throw new Error("invite join response is missing");
	return data;
}

/** 查询自己在该域的申请状态。 */
export async function getJoinRequest(
	domainUUID: string,
): Promise<JoinRequestView> {
	const data = await apiClient.post<JoinRequestView>({
		url: "/api/v1/invite/my-request",
		data: { domain_uuid: domainUUID },
		skipErrorToast: true,
	});
	if (!data) throw new Error("join request response is missing");
	return data;
}

/** 撤销自己的待审申请。 */
export async function cancelJoinRequest(domainUUID: string): Promise<void> {
	await apiClient.post({
		url: "/api/v1/invite/cancel-request",
		data: { domain_uuid: domainUUID },
	});
}

export interface ListJoinRequestsInput {
	domain_uuid: string;
	status?: "" | "pending" | "approved" | "rejected";
	keyword?: string;
	page?: number;
	page_size?: number;
}

/** 管理端：分页查询域内加入申请。 */
export async function listJoinRequests(
	input: ListJoinRequestsInput,
): Promise<JoinRequestListResult> {
	const data = await apiClient.post<JoinRequestListResult>({
		url: "/api/v1/domain/join-requests/list",
		data: input,
	});
	if (!data) throw new Error("join request list is missing");
	return {
		items: data.items ?? [],
		total: data.total ?? 0,
		counts: data.counts ?? {},
	};
}

/** 管理端：通过或拒绝一条申请。 */
export async function reviewJoinRequest(
	domainUUID: string,
	requestID: number,
	approve: boolean,
): Promise<JoinRequestDTO> {
	const data = await apiClient.post<JoinRequestDTO>({
		url: "/api/v1/domain/join-requests/review",
		data: { domain_uuid: domainUUID, request_id: requestID, approve },
	});
	if (!data) throw new Error("join request review response is missing");
	return data;
}

/** 管理端：读写域级「加入需审核」开关。 */
export async function setJoinApproval(
	domainUUID: string,
	require: boolean,
): Promise<{ uuid: string; require_join_approval: boolean }> {
	const data = await apiClient.post<{
		uuid: string;
		require_join_approval: boolean;
	}>({
		url: "/api/v1/domain/join-approval/config",
		data: { domain_uuid: domainUUID, require_join_approval: require },
	});
	if (!data) throw new Error("join approval config response is missing");
	return data;
}
