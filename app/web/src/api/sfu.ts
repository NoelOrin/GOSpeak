import { DEFAULT_SFU_PROVIDER } from "@gospeak/sfu-client";
import type { SFUProvider } from "@gospeak/sfu-client/types";
import apiClient from "./apiClient";

import type { SFUMediaCapabilities } from "./sfuProfiles";
export type { SFUProvider } from "@gospeak/sfu-client/types";

export * from "./sfuProfiles";

export interface GetJoinTokenParams {
	room: string;
	identity: string;
	password?: string;
	domain_uuid?: string;
}

export interface JoinTokenResponse {
	token: string;
	serverUrl: string;
	workerUrl?: string;
	room: string;
	sfuRoom?: string;
	domain_uuid?: string;
	identity: string;
	provider?: SFUProvider;
	appId?: string;
	whipUrl?: string;
	stream?: string;
	streamToken?: string;
	capabilities?: SFUMediaCapabilities;
}

/** Frontend adapter flags + defaults for media capabilities. */

export interface SFUConfig {
	provider: SFUProvider;
	livekit_host: string;
	livekit_key: string;
	/** 管理端读取时始终为空；提交时留空表示保留旧值 */
	livekit_secret: string;
	livekit_secret_set?: boolean;
	agora_app_id: string;
	agora_app_certificate: string;
	agora_app_certificate_set?: boolean;
	agora_host: string;
	agora_customer_id: string;
	agora_customer_secret: string;
	agora_customer_secret_set?: boolean;
	srs_host: string;
	srs_api_port: string;
	srs_secret: string;
	srs_secret_set?: boolean;
	srs_whip_url: string;
	srs_public_host: string;
	cf_app_id: string;
	cf_app_secret: string;
	cf_app_secret_set?: boolean;
	cf_stun_url: string;
	created_at?: string;
	updated_at?: string;
}

/** 更新请求只要求 provider；其余字段按当前 SFU 提供商局部提交。 */

/** 更新请求只要求 provider；其余字段按当前 SFU 提供商局部提交。 */
export type UpdateSFUConfigParams = {
	provider: SFUProvider;
} & Partial<
	Omit<
		SFUConfig,
		| "provider"
		| "created_at"
		| "updated_at"
		| "livekit_secret_set"
		| "agora_app_certificate_set"
		| "agora_customer_secret_set"
		| "srs_secret_set"
		| "cf_app_secret_set"
	>
>;

export interface SFUProvidersListResponse {
	providers: SFUConfig[];
	active: SFUProvider;
	capabilities?: Partial<Record<SFUProvider, SFUMediaCapabilities>>;
}

/** 获取当前激活 provider 的配置 */

/** 获取当前激活 provider 的配置 */
export async function getSFUConfig(): Promise<SFUConfig> {
	const data = await apiClient.post<SFUConfig>({
		url: "/api/v1/sfu/config",
	});

	if (!data) throw new Error("sfu config is missing");
	return data;
}

/** 获取指定 provider 的配置（新） */

/** 获取指定 provider 的配置（新） */
export async function getSFUConfigByProvider(
	provider: SFUProvider,
): Promise<SFUConfig> {
	const data = await apiClient.post<SFUConfig>({
		url: `/api/v1/sfu/config/${provider}`,
	});

	if (!data) throw new Error("sfu config is missing");
	return data;
}

/** 更新指定 provider 的配置并激活为当前（语义不变，但后端已改为 per-provider 行） */

/** 更新指定 provider 的配置并激活为当前（语义不变，但后端已改为 per-provider 行） */
export async function updateSFUConfig(
	params: UpdateSFUConfigParams,
): Promise<SFUConfig> {
	const data = await apiClient.post<SFUConfig>({
		url: "/api/v1/sfu/update-config",
		data: params,
	});

	if (!data) throw new Error("sfu config is missing");
	return data;
}

/** 切换激活的 provider，不修改配置（新） */

/** 切换激活的 provider，不修改配置（新） */
export async function switchSFUProvider(
	provider: SFUProvider,
): Promise<SFUConfig> {
	const data = await apiClient.post<SFUConfig>({
		url: "/api/v1/sfu/switch-provider",
		data: { provider },
	});

	if (!data) throw new Error("sfu config is missing");
	return data;
}

/** 获取所有已配置 provider 列表 + 当前激活的 provider（新） */

/** 获取所有已配置 provider 列表 + 当前激活的 provider（新） */
export async function listSFUProviders(): Promise<SFUProvidersListResponse> {
	const data = await apiClient.post<SFUProvidersListResponse>({
		url: "/api/v1/sfu/providers",
	});

	if (!data) throw new Error("sfu providers list is missing");
	return data;
}

export async function getJoinToken(
	params: GetJoinTokenParams,
	signal?: AbortSignal,
): Promise<JoinTokenResponse> {
	const data = await apiClient.post<JoinTokenResponse>({
		url: "/api/v1/signal/token",
		data: params,
		signal,
	});

	if (!data) throw new Error("join token response is missing");
	return data;
}

export function resolveSFUProvider(
	response: Pick<JoinTokenResponse, "provider">,
): SFUProvider {
	return (
		response.provider ||
		import.meta.env.VITE_SFU_PROVIDER ||
		DEFAULT_SFU_PROVIDER
	);
}
