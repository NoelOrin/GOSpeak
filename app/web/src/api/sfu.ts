import { DEFAULT_SFU_PROVIDER } from "@gospeak/sfu-client";
import type { SFUProvider } from "@gospeak/sfu-client/types";
import type { AxiosResponse } from "axios";
import type { Result } from "./apiClient";
import apiClient from "./apiClient";

export type { SFUProvider } from "@gospeak/sfu-client/types";

export interface SFUProviderCapabilities {
	supportsParticipants: boolean;
	requiresSignalAdapter: boolean;
	kickViaSignal: boolean;
}

export interface GetJoinTokenParams {
	room: string;
	identity: string;
	password?: string;
}

export interface JoinTokenResponse {
	token: string;
	serverUrl: string;
	room: string;
	identity: string;
	provider?: SFUProvider;
	appId?: string;
	bridgeUrl?: string;
	whipUrl?: string;
	dailyDomain?: string;
	stream?: string;
	streamToken?: string;
}

export const SFU_PROVIDER_CAPABILITIES: Record<
	SFUProvider,
	SFUProviderCapabilities
> = {
	livekit: {
		supportsParticipants: true,
		requiresSignalAdapter: false,
		kickViaSignal: true,
	},
	agora: {
		supportsParticipants: true,
		requiresSignalAdapter: false,
		kickViaSignal: true,
	},
	mediasoup: {
		supportsParticipants: false,
		requiresSignalAdapter: true,
		kickViaSignal: true,
	},
	srs: {
		supportsParticipants: false,
		requiresSignalAdapter: false,
		kickViaSignal: false,
	},
	daily: {
		supportsParticipants: true,
		requiresSignalAdapter: false,
		kickViaSignal: true,
	},
	cloudflare: {
		supportsParticipants: false,
		requiresSignalAdapter: false,
		kickViaSignal: true,
	},
};

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
	mediasoup_bridge_url: string;
	mediasoup_host: string;
	srs_host: string;
	srs_api_port: string;
	srs_secret: string;
	srs_secret_set?: boolean;
	srs_whip_url: string;
	srs_public_host: string;
	daily_api_key: string;
	daily_api_key_set?: boolean;
	daily_domain: string;
	cf_app_id: string;
	cf_app_secret: string;
	cf_app_secret_set?: boolean;
	cf_stun_url: string;
	created_at?: string;
	updated_at?: string;
}

export type UpdateSFUConfigParams = Omit<
	SFUConfig,
	| "created_at"
	| "updated_at"
	| "livekit_secret_set"
	| "agora_app_certificate_set"
	| "agora_customer_secret_set"
	| "srs_secret_set"
	| "daily_api_key_set"
	| "cf_app_secret_set"
>;

export interface SFUProvidersListResponse {
	providers: SFUConfig[];
	active: SFUProvider;
}

/** 获取当前激活 provider 的配置 */
export async function getSFUConfig(): Promise<SFUConfig> {
	const res = (await apiClient.post({
		url: "/api/v1/sfu/config",
	})) as AxiosResponse<Result<SFUConfig>>;

	if (!(res as any).data.data) throw new Error("sfu config is missing");
	return (res as any).data.data;
}

/** 获取指定 provider 的配置（新） */
export async function getSFUConfigByProvider(
	provider: SFUProvider,
): Promise<SFUConfig> {
	const res = (await apiClient.post({
		url: `/api/v1/sfu/config/${provider}`,
	})) as AxiosResponse<Result<SFUConfig>>;

	if (!(res as any).data.data) throw new Error("sfu config is missing");
	return (res as any).data.data;
}

/** 更新指定 provider 的配置并激活为当前（语义不变，但后端已改为 per-provider 行） */
export async function updateSFUConfig(
	params: UpdateSFUConfigParams,
): Promise<SFUConfig> {
	const res = (await apiClient.post({
		url: "/api/v1/sfu/update-config",
		data: params,
	})) as AxiosResponse<Result<SFUConfig>>;

	if (!(res as any).data.data) throw new Error("sfu config is missing");
	return (res as any).data.data;
}

/** 切换激活的 provider，不修改配置（新） */
export async function switchSFUProvider(
	provider: SFUProvider,
): Promise<SFUConfig> {
	const res = (await apiClient.post({
		url: "/api/v1/sfu/switch-provider",
		data: { provider },
	})) as AxiosResponse<Result<SFUConfig>>;

	if (!(res as any).data.data) throw new Error("sfu config is missing");
	return (res as any).data.data;
}

/** 获取所有已配置 provider 列表 + 当前激活的 provider（新） */
export async function listSFUProviders(): Promise<SFUProvidersListResponse> {
	const res = (await apiClient.post({
		url: "/api/v1/sfu/providers",
	})) as AxiosResponse<Result<SFUProvidersListResponse>>;

	if (!(res as any).data.data) throw new Error("sfu providers list is missing");
	return (res as any).data.data;
}

export async function getJoinToken(
	params: GetJoinTokenParams,
	signal?: AbortSignal,
): Promise<JoinTokenResponse> {
	const res = (await apiClient.post({
		url: "/api/v1/signal/token",
		data: params,
		signal,
	})) as AxiosResponse<Result<JoinTokenResponse>>;

	if (!(res as any).data.data)
		throw new Error("join token response is missing");
	return (res as any).data.data;
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

export function getSFUProviderCapabilities(
	provider: SFUProvider,
): SFUProviderCapabilities {
	return SFU_PROVIDER_CAPABILITIES[provider];
}
