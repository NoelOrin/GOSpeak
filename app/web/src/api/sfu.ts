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
};

export interface SFUConfig {
	id: number;
	provider: SFUProvider;
	livekit_host: string;
	livekit_key: string;
	livekit_secret: string;
	agora_app_id: string;
	agora_app_certificate: string;
	agora_host: string;
	agora_customer_id: string;
	agora_customer_secret: string;
	mediasoup_bridge_url: string;
	mediasoup_host: string;
	srs_host: string;
	srs_api_port: string;
	srs_whip_port: string;
	srs_secret: string;
	daily_api_key: string;
	daily_domain: string;
	created_at?: string;
	updated_at?: string;
}

export type UpdateSFUConfigParams = Omit<
	SFUConfig,
	"id" | "created_at" | "updated_at"
>;

export async function getSFUConfig(): Promise<SFUConfig> {
	const res = (await apiClient.post({
		url: "/api/v1/sfu/config",
	})) as AxiosResponse<Result<SFUConfig>>;

	const result = res.data;
	if (result.code !== 0) throw new Error(result.msg);
	if (!result.data) throw new Error("sfu config is missing");
	return result.data;
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

	const result = res.data;
	if (result.code !== 0) throw new Error(result.msg);
	if (!result.data) throw new Error("join token response is missing");
	return result.data;
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

export async function updateSFUConfig(
	params: UpdateSFUConfigParams,
): Promise<SFUConfig> {
	const res = (await apiClient.post({
		url: "/api/v1/sfu/update-config",
		data: params,
	})) as AxiosResponse<Result<SFUConfig>>;

	const result = res.data;
	if (result.code !== 0) throw new Error(result.msg);
	if (!result.data) throw new Error("sfu config is missing");
	return result.data;
}
