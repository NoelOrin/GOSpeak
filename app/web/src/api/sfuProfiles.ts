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
