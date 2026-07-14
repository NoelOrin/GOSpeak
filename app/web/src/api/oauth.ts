import type { Result } from "./apiClient";
import apiClient from "./apiClient";

export interface OAuthProvider {
	id: number;
	name: string;
	display_name: string;
	icon_url: string;
	client_id: string;
	/** 管理端读取时始终为空；提交时留空表示保留旧值 */
	client_secret: string;
	client_secret_set?: boolean;
	auth_url: string;
	token_url: string;
	userinfo_url: string;
	redirect_url: string;
	scopes: string;
	uid_field: string;
	username_field: string;
	avatar_field: string;
	email_field: string;
	enabled: boolean;
	created_at: string;
	updated_at: string;
}

export interface EnabledProvider {
	name: string;
	display_name: string;
	icon_url: string;
}

export interface CreateProviderInput {
	name: string;
	display_name?: string;
	icon_url?: string;
	client_id?: string;
	client_secret?: string;
	auth_url?: string;
	token_url?: string;
	user_info_url?: string;
	redirect_url?: string;
	scopes?: string;
	uid_field?: string;
	username_field?: string;
	avatar_field?: string;
	email_field?: string;
	enabled?: boolean;
}

export interface UpdateProviderInput {
	id: number;
	name?: string;
	display_name?: string;
	icon_url?: string;
	client_id?: string;
	client_secret?: string;
	auth_url?: string;
	token_url?: string;
	user_info_url?: string;
	redirect_url?: string;
	scopes?: string;
	uid_field?: string;
	username_field?: string;
	avatar_field?: string;
	email_field?: string;
	enabled?: boolean;
}

export function listProviders(): Promise<Result<OAuthProvider[]>> {
	return apiClient
		.get({ url: "/api/v1/oauth/admin/providers" })
		.then((r) => r.data);
}

export function listEnabledProviders(): Promise<Result<EnabledProvider[]>> {
	return apiClient.get({ url: "/api/v1/oauth/providers" }).then((r) => r.data);
}

export function createProvider(
	input: CreateProviderInput,
): Promise<Result<OAuthProvider>> {
	return apiClient
		.post({ url: "/api/v1/oauth/admin/providers", data: input })
		.then((r) => r.data);
}

export function updateProvider(
	input: UpdateProviderInput,
): Promise<Result<OAuthProvider>> {
	return apiClient
		.put({ url: "/api/v1/oauth/admin/providers", data: input })
		.then((r) => r.data);
}

export function deleteProvider(id: number): Promise<Result<null>> {
	return apiClient
		.delete({ url: `/api/v1/oauth/admin/providers/${id}` })
		.then((r) => r.data);
}

/** 公开：获取已启用 OAuth 提供商（登录页使用，无需登录）。 */
export async function getEnabledProviders(): Promise<EnabledProvider[]> {
	const res = await listEnabledProviders();
	if (res.code !== 0) {
		throw new Error(res.msg || "failed to load oauth providers");
	}
	return res.data ?? [];
}

/** 浏览器跳转 OAuth 授权页；state 由服务端 cookie 生成/校验。 */
export function getOAuthLoginURL(provider: string): string {
	const name = encodeURIComponent(provider);
	return `/api/v1/oauth/login/${name}`;
}
