import { DEFAULT_SFU_PROVIDER } from "@gospeak/sfu-client";
import type { SFUProvider } from "@gospeak/sfu-client/types";
import type { AxiosResponse } from "axios";
import type { Result } from "./apiClient";
import apiClient from "./apiClient";

export type { SFUProvider } from "@gospeak/sfu-client/types";

export interface SFUProviderCapabilities {
	/** @deprecated use server-side media capabilities instead */
	supportsParticipants: boolean;
	/** @deprecated MediaSoup-only frontend adapter flag */
	requiresSignalAdapter: boolean;
	/** @deprecated kick always goes through signal; use serverKick for media force */
	kickViaSignal: boolean;
	// Media-layer hard-enforcement (from backend sfu.Capabilities)
	serverMute?: boolean;
	serverKick?: boolean;
	deleteRoom?: boolean;
	adminToken?: boolean;
	listRooms?: boolean;
	listMembers?: boolean;
}

export type SFUEnforcementLevel = "hard" | "degraded" | "soft" | "none";

export interface SFUMediaCapabilities {
	serverMute: boolean;
	serverKick: boolean;
	deleteRoom: boolean;
	adminToken: boolean;
	listRooms: boolean;
	listMembers: boolean;
	muteLevel?: SFUEnforcementLevel;
	kickLevel?: SFUEnforcementLevel;
	deleteLevel?: SFUEnforcementLevel;
	listLevel?: SFUEnforcementLevel;
	adminLevel?: SFUEnforcementLevel;
}

/** hard = media forced; degraded = forced via substitute API; soft = signal/policy only */

export interface SFUCapabilityDetail {
	key:
		| "serverMute"
		| "serverKick"
		| "deleteRoom"
		| "listMembers"
		| "listRooms"
		| "adminToken";
	label: string;
	level: SFUEnforcementLevel;
	/** short implementation note shown on selected-provider card */
	impl: string;
	/** soft path that always exists underneath */
	fallback: string;
}

export interface SFUEnforcementProfile {
	provider: SFUProvider;
	summary: string;
	details: SFUCapabilityDetail[];
}

export const SFU_ENFORCEMENT_PROFILES: Record<
	SFUProvider,
	SFUEnforcementProfile
> = {
	livekit: {
		provider: "livekit",
		summary: "原生能力最完整：静音/踢人/删房均可媒体层强制。",
		details: [
			{
				key: "serverMute",
				label: "服务端静音",
				level: "hard",
				impl: "MutePublishedTrack 按 track/identity 静音",
				fallback: "信令 user:muted + 前端停麦",
			},
			{
				key: "serverKick",
				label: "服务端踢人",
				level: "hard",
				impl: "RoomService.RemoveParticipant 断开会话",
				fallback: "信令 room:kicked + 成员表移除",
			},
			{
				key: "deleteRoom",
				label: "删除房间",
				level: "hard",
				impl: "DeleteRoom 删除媒体房间",
				fallback: "信令清房间并广播列表",
			},
			{
				key: "listMembers",
				label: "成员列表",
				level: "hard",
				impl: "ListParticipants 查媒体态成员",
				fallback: "不与 WS 在线表互相伪装",
			},
			{
				key: "listRooms",
				label: "房间列表",
				level: "hard",
				impl: "ListRooms 查活跃媒体房间",
				fallback: "不与信令房间表互相伪装",
			},
			{
				key: "adminToken",
				label: "管理 Token",
				level: "hard",
				impl: "签发 RoomCreate/RoomList admin JWT",
				fallback: "无则不伪造 admin token",
			},
		],
	},
	mediasoup: {
		provider: "mediasoup",
		summary: "经 bridge 强制 pause/close；需专属 WebSocket 信令适配。",
		details: [
			{
				key: "serverMute",
				label: "服务端静音",
				level: "hard",
				impl: "PauseProducer / PauseParticipant",
				fallback: "信令 user:muted + 前端停麦",
			},
			{
				key: "serverKick",
				label: "服务端踢人",
				level: "hard",
				impl: "CloseParticipant 关闭 transport",
				fallback: "信令 room:kicked + 成员表移除",
			},
			{
				key: "deleteRoom",
				label: "删除房间",
				level: "hard",
				impl: "bridge 清理 router/room",
				fallback: "信令清房间",
			},
			{
				key: "listMembers",
				label: "成员列表",
				level: "hard",
				impl: "bridge ListParticipants",
				fallback: "不与 WS 在线表互相伪装",
			},
			{
				key: "listRooms",
				label: "房间列表",
				level: "hard",
				impl: "bridge ListRouters",
				fallback: "不与信令房间表互相伪装",
			},
			{
				key: "adminToken",
				label: "管理 Token",
				level: "none",
				impl: "无 admin join token，bridge 自身鉴权",
				fallback: "返回 not supported",
			},
		],
	},
	agora: {
		provider: "agora",
		summary: "无原生 mute/kick REST；用 kicking-rule 做降级 hard。",
		details: [
			{
				key: "serverMute",
				label: "服务端静音",
				level: "degraded",
				impl: "kicking-rule 撤销 publish_audio/video，保留在频道",
				fallback: "信令禁言 + 前端 speechRestricted 停推",
			},
			{
				key: "serverKick",
				label: "服务端踢人",
				level: "degraded",
				impl: "短时 kicking-rule(join_channel, 60s) 强制离会",
				fallback: "信令 room:kicked；短暂挡重进≠永久 ban",
			},
			{
				key: "deleteRoom",
				label: "删除房间",
				level: "hard",
				impl: "DeleteChannel 删除频道",
				fallback: "信令清房间",
			},
			{
				key: "listMembers",
				label: "成员列表",
				level: "hard",
				impl: "channel user REST 列表",
				fallback: "不与 WS 在线表互相伪装",
			},
			{
				key: "listRooms",
				label: "房间列表",
				level: "hard",
				impl: "channel list REST",
				fallback: "不与信令房间表互相伪装",
			},
			{
				key: "adminToken",
				label: "管理 Token",
				level: "none",
				impl: "管理面使用 Customer ID/Secret，无 admin RTC token",
				fallback: "返回 not supported",
			},
		],
	},
	srs: {
		provider: "srs",
		summary: "WHIP/WHEP 流模型；静音靠强制停推，解禁需前端重推。",
		details: [
			{
				key: "serverMute",
				label: "服务端静音",
				level: "degraded",
				impl: "KickByStreams 踢掉 WHIP 推流客户端（强制 unpublish）",
				fallback: "信令禁言；unmute 后前端重新 WHIP",
			},
			{
				key: "serverKick",
				label: "服务端踢人",
				level: "hard",
				impl: "按 stream 踢 client，断开媒体",
				fallback: "信令 room:kicked + 成员表移除",
			},
			{
				key: "deleteRoom",
				label: "删除房间",
				level: "hard",
				impl: "按 room streams 批量踢流 + 清理 registry",
				fallback: "信令清房间",
			},
			{
				key: "listMembers",
				label: "成员列表",
				level: "degraded",
				impl: "RoomRegistry + SRS clients 聚合 identity",
				fallback: "无 registry 时能力受限",
			},
			{
				key: "listRooms",
				label: "房间列表",
				level: "degraded",
				impl: "RoomRegistry 聚合活跃 room",
				fallback: "不直接拿信令房间伪装媒体房间",
			},
			{
				key: "adminToken",
				label: "管理 Token",
				level: "hard",
				impl: "HMAC/JWT stream admin token",
				fallback: "无 secret 时拒签",
			},
		],
	},
	daily: {
		provider: "daily",
		summary: "踢人 REST 可用；静音无可靠服务端 track mute，走 soft。",
		details: [
			{
				key: "serverMute",
				label: "服务端静音",
				level: "soft",
				impl: "无服务端轨道静音 API",
				fallback: "信令 user:muted + 前端 setMicEnabled(false)",
			},
			{
				key: "serverKick",
				label: "服务端踢人",
				level: "hard",
				impl: "presence 查 session id → remove participant",
				fallback: "信令 room:kicked",
			},
			{
				key: "deleteRoom",
				label: "删除房间",
				level: "hard",
				impl: "DELETE /rooms/{name}",
				fallback: "信令清房间",
			},
			{
				key: "listMembers",
				label: "成员列表",
				level: "hard",
				impl: "rooms/{name}/presence",
				fallback: "不与 WS 在线表互相伪装",
			},
			{
				key: "listRooms",
				label: "房间列表",
				level: "hard",
				impl: "GET /rooms",
				fallback: "不与信令房间表互相伪装",
			},
			{
				key: "adminToken",
				label: "管理 Token",
				level: "none",
				impl: "管理使用 API Key，非 meeting token",
				fallback: "返回 not supported",
			},
		],
	},
	cloudflare: {
		provider: "cloudflare",
		summary: "无原生房间/静音；踢人靠删 session，列表仅进程内缓存。",
		details: [
			{
				key: "serverMute",
				label: "服务端静音",
				level: "soft",
				impl: "无 session 级 mute；关 track 不足以稳定替代",
				fallback: "信令 user:muted + 前端停推",
			},
			{
				key: "serverKick",
				label: "服务端踢人",
				level: "hard",
				impl: "DeleteSession 终止该 identity 的 session",
				fallback: "信令 room:kicked",
			},
			{
				key: "deleteRoom",
				label: "删除房间",
				level: "degraded",
				impl: "批量 DeleteSession（进程内 session map）",
				fallback: "多实例/重启后 map 不完整",
			},
			{
				key: "listMembers",
				label: "成员列表",
				level: "degraded",
				impl: "本地 sessions map 的 identity 集合",
				fallback: "非跨实例权威",
			},
			{
				key: "listRooms",
				label: "房间列表",
				level: "degraded",
				impl: "本地 sessions map 的 room 键",
				fallback: "非跨实例权威",
			},
			{
				key: "adminToken",
				label: "管理 Token",
				level: "none",
				impl: "使用 App Secret 调 REST，无 admin join token",
				fallback: "返回 not supported",
			},
		],
	},
};

export function getSFUEnforcementProfile(
	provider: SFUProvider,
	override?: SFUMediaCapabilities | null,
): SFUEnforcementProfile {
	const base = SFU_ENFORCEMENT_PROFILES[provider];
	if (!override) return base;

	const levelFromBool = (
		enabled: boolean | undefined,
		preferred?: SFUEnforcementLevel,
		fallback: SFUEnforcementLevel = "soft",
	): SFUEnforcementLevel => {
		if (preferred) return preferred;
		if (enabled) return fallback === "none" ? "hard" : fallback;
		return fallback === "hard" || fallback === "degraded" ? "soft" : fallback;
	};

	const map: Record<SFUCapabilityDetail["key"], SFUEnforcementLevel> = {
		serverMute: levelFromBool(
			override.serverMute,
			override.muteLevel,
			base.details.find((d) => d.key === "serverMute")?.level ?? "soft",
		),
		serverKick: levelFromBool(
			override.serverKick,
			override.kickLevel,
			base.details.find((d) => d.key === "serverKick")?.level ?? "soft",
		),
		deleteRoom: levelFromBool(
			override.deleteRoom,
			override.deleteLevel,
			base.details.find((d) => d.key === "deleteRoom")?.level ?? "soft",
		),
		listMembers: levelFromBool(
			override.listMembers,
			override.listLevel,
			base.details.find((d) => d.key === "listMembers")?.level ?? "soft",
		),
		listRooms: levelFromBool(
			override.listRooms,
			override.listLevel,
			base.details.find((d) => d.key === "listRooms")?.level ?? "soft",
		),
		adminToken: levelFromBool(
			override.adminToken,
			override.adminLevel,
			base.details.find((d) => d.key === "adminToken")?.level ?? "none",
		),
	};

	return {
		...base,
		details: base.details.map((d) => ({
			...d,
			level: map[d.key] ?? d.level,
		})),
	};
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
	capabilities?: SFUMediaCapabilities;
}

/** Frontend adapter flags + defaults for media capabilities. */
export const SFU_PROVIDER_CAPABILITIES: Record<
	SFUProvider,
	SFUProviderCapabilities
> = {
	livekit: {
		supportsParticipants: true,
		requiresSignalAdapter: false,
		kickViaSignal: true,
		serverMute: true,
		serverKick: true,
		deleteRoom: true,
		adminToken: true,
		listRooms: true,
		listMembers: true,
	},
	agora: {
		supportsParticipants: true,
		requiresSignalAdapter: false,
		kickViaSignal: true,
		serverMute: true,
		serverKick: true,
		deleteRoom: true,
		adminToken: false,
		listRooms: true,
		listMembers: true,
	},
	mediasoup: {
		supportsParticipants: true,
		requiresSignalAdapter: true,
		kickViaSignal: true,
		serverMute: true,
		serverKick: true,
		deleteRoom: true,
		adminToken: false,
		listRooms: true,
		listMembers: true,
	},
	srs: {
		supportsParticipants: true,
		requiresSignalAdapter: false,
		kickViaSignal: true,
		serverMute: true,
		serverKick: true,
		deleteRoom: true,
		adminToken: true,
		listRooms: true,
		listMembers: true,
	},
	daily: {
		supportsParticipants: true,
		requiresSignalAdapter: false,
		kickViaSignal: true,
		serverMute: false,
		serverKick: true,
		deleteRoom: true,
		adminToken: false,
		listRooms: true,
		listMembers: true,
	},
	cloudflare: {
		supportsParticipants: true,
		requiresSignalAdapter: false,
		kickViaSignal: true,
		serverMute: false,
		serverKick: true,
		deleteRoom: true,
		adminToken: false,
		listRooms: true,
		listMembers: true,
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
		| "daily_api_key_set"
		| "cf_app_secret_set"
	>
>;

export interface SFUProvidersListResponse {
	providers: SFUConfig[];
	active: SFUProvider;
	capabilities?: Partial<Record<SFUProvider, SFUMediaCapabilities>>;
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
	override?: SFUMediaCapabilities | null,
): SFUProviderCapabilities {
	const base = SFU_PROVIDER_CAPABILITIES[provider];
	if (!override) return base;
	return {
		...base,
		serverMute: override.serverMute,
		serverKick: override.serverKick,
		deleteRoom: override.deleteRoom,
		adminToken: override.adminToken,
		listRooms: override.listRooms,
		listMembers: override.listMembers,
		supportsParticipants: override.listMembers,
	};
}
