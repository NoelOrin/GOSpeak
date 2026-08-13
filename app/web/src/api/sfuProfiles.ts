import type { SFUProvider } from "@gospeak/sfu-client/types";

export interface SFUProviderCapabilities {
	/** @deprecated use server-side media capabilities instead */
	supportsParticipants: boolean;
	/** @deprecated kick always goes through signal; use serverKick for media force */
	kickViaSignal: boolean;
	// Media-layer hard-enforcement (from backend sfu.Capabilities)
	serverMute?: boolean;
	serverKick?: boolean;
	deleteRoom?: boolean;
	listRooms?: boolean;
	listMembers?: boolean;
}

export type SFUEnforcementLevel = "hard" | "degraded" | "soft" | "none";

export interface SFUMediaCapabilities {
	serverMute: boolean;
	serverKick: boolean;
	deleteRoom: boolean;
	listRooms: boolean;
	listMembers: boolean;
	muteLevel?: SFUEnforcementLevel;
	kickLevel?: SFUEnforcementLevel;
	deleteLevel?: SFUEnforcementLevel;
	listLevel?: SFUEnforcementLevel;
}

/** hard = media forced; degraded = forced via substitute API; soft = signal/policy only */

/** hard = media forced; degraded = forced via substitute API; soft = signal/policy only */

export interface SFUCapabilityDetail {
	key: "serverMute" | "serverKick" | "deleteRoom" | "listMembers" | "listRooms";
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
				impl: "kicking-rule 撤销 publish_audio/video，保留在房间",
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
				impl: "DeleteChannel 删除房间",
				fallback: "信令清房间",
			},
			{
				key: "listMembers",
				label: "成员列表",
				level: "hard",
				impl: "Agora channel user REST 列表",
				fallback: "不与 WS 在线表互相伪装",
			},
			{
				key: "listRooms",
				label: "房间列表",
				level: "hard",
				impl: "Agora channel list REST",
				fallback: "不与信令房间表互相伪装",
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
				impl: "写禁推黑名单 + 订阅端静音，断流后禁止重推",
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
				level: "hard",
				impl: "SRS /api/v1/streams 直查 + stream→room 反查聚合 identity",
				fallback: "无 registry 时能力受限",
			},
			{
				key: "listRooms",
				label: "房间列表",
				level: "hard",
				impl: "SRS /api/v1/streams 直查 + stream→room 反查聚合 room",
				fallback: "不直接拿信令房间伪装媒体房间",
			},
		],
	},
	cloudflare: {
		provider: "cloudflare",
		summary:
			"无原生房间；禁言靠 CloseTracks 关轨，踢人靠删 session，列表仅进程内缓存。",
		details: [
			{
				key: "serverMute",
				label: "服务端静音",
				level: "degraded",
				impl: "CloseTracks 关闭本地发布轨道（保留订阅）",
				fallback: "unmute 媒体层 no-op，客户端重新发布",
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
	};

	return {
		...base,
		details: base.details.map((d) => ({
			...d,
			level: map[d.key] ?? d.level,
		})),
	};
}

/** Frontend adapter flags + defaults for media capabilities. */
export const SFU_PROVIDER_CAPABILITIES: Record<
	SFUProvider,
	SFUProviderCapabilities
> = {
	livekit: {
		supportsParticipants: true,
		kickViaSignal: true,
		serverMute: true,
		serverKick: true,
		deleteRoom: true,
		listRooms: true,
		listMembers: true,
	},
	agora: {
		supportsParticipants: true,
		kickViaSignal: true,
		serverMute: true,
		serverKick: true,
		deleteRoom: true,
		listRooms: true,
		listMembers: true,
	},
	srs: {
		supportsParticipants: true,
		kickViaSignal: true,
		serverMute: true,
		serverKick: true,
		deleteRoom: true,
		listRooms: true,
		listMembers: true,
	},
	cloudflare: {
		supportsParticipants: true,
		kickViaSignal: true,
		serverMute: true,
		serverKick: true,
		deleteRoom: true,
		listRooms: true,
		listMembers: true,
	},
};

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
		listRooms: override.listRooms,
		listMembers: override.listMembers,
		supportsParticipants: override.listMembers,
	};
}
