import type { SFUProvider } from "@gospeak/sfu-client/types";
import { toast } from "@gospeak/components";
import { setServerMutedByIdentity } from "@/handler_audio";
import type { AnnouncementDTO } from "@/protocol/announcement";
import type { PrivateMessageDTO } from "@/protocol/conversation";
import { EVENTS } from "@/socket/events";
import type { MessageDTO } from "@/types/message";
import {
	applyTypingEvent,
	type TypingEventPayload,
} from "@/stores/typingStore";
import {
	addCreatedRoom,
	applyMemberJoined,
	applyMemberLeft,
	applyMemberUpdated,
	mergeRoomUpdated,
} from "@/socket/roomState";
import type {
	ActivityEvent,
	MuteEvent,
	RoomInfo,
	RoomPresenceEvent,
	UnmuteEvent,
} from "@/socket/types";

export type SpeechRestrictionInfo = {
	permanent: boolean;
	expires_at: string | null;
	reason: string;
};

export interface ServerEventBinder {
	disconnect(): void;
	onServerEvent(event: string, handler: (data: any) => void): unknown;
}

type Updater<T> = T | ((prev: T) => T);

export interface SocketEventDeps {
	setRooms: (value: Updater<RoomInfo[]>) => void;
	setCurrentRoom: (value: Updater<string | null>) => void;
	setSelectedRoomInfo: (value: Updater<RoomInfo | null>) => void;
	setActiveSFUProvider: (value: Updater<SFUProvider | undefined>) => void;
	setConnecting: (value: Updater<boolean>) => void;
	setConnected: (value: Updater<boolean>) => void;
	setSpeechRestricted: (value: Updater<boolean>) => void;
	setSpeechRestrictionInfo: (
		value: Updater<SpeechRestrictionInfo | null>,
	) => void;
	setSpeakingIdentities: (value: Updater<string[]>) => void;
	currentRoom: () => string | null;
	currentDomainUUID: () => string | null;
	emitActivity: (event: ActivityEvent) => void;
	emitPresence: (event: RoomPresenceEvent) => void;
	emitKicked: () => void;
	handleProviderChanged: (provider?: string) => void;
	handlePrivateNew: (dto: PrivateMessageDTO) => void;
	handleMessageCreated: (dto: MessageDTO) => void;
	handleAnnouncementNew: (dto: AnnouncementDTO) => void;
	currentUserName: () => string;
	currentUserID: () => number | undefined;
}

export function bindServerEvents(
	adapter: ServerEventBinder,
	deps: SocketEventDeps,
): void {
	adapter.onServerEvent(
		EVENTS.ROOM_CREATED,
		(room: RoomInfo & { error?: string }) => {
			console.log("[Socket] room:created", room.name, room.error);
			if (room.error) {
				toast.error(`创建房间失败: ${room.error}`);
				return;
			}
			deps.setRooms((prev) => addCreatedRoom(prev, room));
		},
	);

	adapter.onServerEvent(EVENTS.ROOM_UPDATED, (room: RoomInfo) => {
		if (!room?.name) return;
		console.log("[Socket] room:updated", room.name, "count=", room.count);
		// 仅覆盖服务端实际返回的字段（name/has_password/members/count/created_at），
		// 保留 DB 字段（id/uuid/description/limit/audio_only/allow_audience）不被零值覆盖。
		// members 由 rooms[] 派生，无需单独维护。
		deps.setRooms((prev) => mergeRoomUpdated(prev, room));
	});

	adapter.onServerEvent(
		EVENTS.MEMBER_JOINED as string,
		(data: {
			room: string;
			identity: string;
			id: string;
			stream?: string;
			domain_uuid?: string;
		}) => {
			console.log("[Socket] member:joined", data.identity);
			// 只做计数增量：本地不拼装缺 name/avatar 的空壳成员，
			// 服务端在 member:joined 后紧跟广播 room:updated 全量补齐成员明细。
			deps.setRooms((prev) => applyMemberJoined(prev, data));
			deps.emitActivity({
				type: "member_joined",
				room: data.room,
				domain_uuid: data.domain_uuid,
				identity: data.identity,
				timestamp: Date.now(),
			});
			deps.emitPresence({
				type: "member_joined",
				room: data.room,
				domain_uuid: data.domain_uuid,
				identity: data.identity,
				timestamp: Date.now(),
			});
		},
	);

	adapter.onServerEvent(
		EVENTS.MEMBER_LEFT as string,
		(data: {
			room: string;
			identity: string;
			id: string;
			domain_uuid?: string;
		}) => {
			console.log("[Socket] member:left", data.identity);
			deps.setRooms((prev) => applyMemberLeft(prev, data));
			deps.emitActivity({
				type: "member_left",
				room: data.room,
				domain_uuid: data.domain_uuid,
				identity: data.identity,
				timestamp: Date.now(),
			});
			deps.emitPresence({
				type: "member_left",
				room: data.room,
				domain_uuid: data.domain_uuid,
				identity: data.identity,
				timestamp: Date.now(),
			});
		},
	);

	adapter.onServerEvent(
		EVENTS.MEMBER_UPDATED,
		(data: {
			room: string;
			identity: string;
			is_mic_muted: boolean;
			domain_uuid?: string;
		}) => {
			// members 由 rooms[] 派生，仅更新 rooms[] 即可
			deps.setRooms((prev) => applyMemberUpdated(prev, data));
		},
	);

	adapter.onServerEvent(
		EVENTS.ROOM_LIST_RESULT as string,
		(data: { rooms: RoomInfo[]; count: number }) => {
			console.log("[Socket] room:list:result", data.rooms.length, "rooms");
			deps.setRooms(data.rooms);
		},
	);

	adapter.onServerEvent(
		EVENTS.ROOM_KICKED,
		(data: {
			room: string;
			target_identity: string;
			enforcement?: string;
			domain_uuid?: string;
		}) => {
			console.log(
				"[Socket] room:kicked",
				data.room,
				data.target_identity,
				data.enforcement,
			);
			if (data.target_identity !== deps.currentUserName()) return;
			const kickMsg =
				data.enforcement === "soft"
					? "你已被移出房间（软踢：请断开本地媒体）"
					: data.enforcement === "degraded"
						? "你已被移出房间（降级强制）"
						: "你已被移出房间";
			toast.error(kickMsg);
			deps.setCurrentRoom(null);
			deps.setSelectedRoomInfo(null);
			deps.emitKicked();
		},
	);

	// 用户级禁言事件：允许收听，不允许发布本地音轨
	// 必须按 user_id 过滤，避免全服广播误伤其他客户端
	adapter.onServerEvent(
		EVENTS.USER_MUTED,
		(data: MuteEvent & { enforcement?: string }) => {
			console.log("[Socket] user:muted", data.user_id, data.enforcement);
			if (data.user_id !== deps.currentUserID()) return;
			const mode =
				data.enforcement === "hard"
					? "服务端原生强制停止推流"
					: data.enforcement === "degraded"
						? "服务端降级强制停止推流"
						: "请停止本地推流（软禁言）";
			toast.warning(
				data.permanent
					? `你已被永久禁言，当前为仅收听模式（${mode}）`
					: `你已被禁言，当前为仅收听模式（${mode}）${data.reason ? `，原因: ${data.reason}` : ""}`,
			);
			deps.setSpeechRestricted(true);
			deps.setSpeechRestrictionInfo({
				permanent: data.permanent,
				expires_at: data.expires_at,
				reason: data.reason,
			});
		},
	);

	adapter.onServerEvent(EVENTS.USER_UNMUTED, (data: UnmuteEvent) => {
		console.log("[Socket] user:unmuted", data.user_id);
		if (data.user_id !== deps.currentUserID()) return;
		toast.success("你的禁言已被解除，可以重新发言");
		deps.setSpeechRestricted(false);
		deps.setSpeechRestrictionInfo(null);
	});
	adapter.onServerEvent(
		EVENTS.MEMBER_MUTED as string,
		(data: { identity?: string; muted?: boolean }) => {
			if (data.identity) {
				setServerMutedByIdentity(data.identity, data.muted !== false);
			}
		},
	);
	adapter.onServerEvent(
		EVENTS.MEMBER_UNMUTED as string,
		(data: { identity?: string; muted?: boolean }) => {
			if (data.identity) {
				setServerMutedByIdentity(data.identity, Boolean(data.muted));
			}
		},
	);
	// 发言检测：信令层聚合后广播房间级 active speakers
	adapter.onServerEvent(
		EVENTS.ROOM_ACTIVE_SPEAKERS,
		(event: { room?: string; identities?: string[]; domain_uuid?: string }) => {
			const room = event?.room;
			if (
				room &&
				(room !== deps.currentRoom() ||
					(event.domain_uuid || "") !== (deps.currentDomainUUID() || ""))
			)
				return;
			deps.setSpeakingIdentities(event?.identities ?? []);
		},
	);

	// SFU 热切换：先断 socket；rejoin 优先（保留房间意图，handleProviderChanged
	// 决定原地重连或兜底整页刷新），join 中途同样直接走该流程
	adapter.onServerEvent(
		EVENTS.SFU_PROVIDER_CHANGED,
		(data: { provider?: string }) => {
			try {
				adapter.disconnect();
			} catch {
				// ignore
			}
			deps.setConnecting(false);
			deps.setConnected(false);
			// currentRoom / selectedRoomInfo 是「用户想留在房间」的意图，
			// 不在此清除：providerReload 的 rejoin 路径依赖它派生重连。
			deps.setActiveSFUProvider(undefined);
			deps.handleProviderChanged(data?.provider);
		},
	);

	// 全局私聊新消息监听：即使用户未打开任何会话也能收到通知
	adapter.onServerEvent(EVENTS.PRIVATE_NEW, (dto: PrivateMessageDTO) => {
		deps.handlePrivateNew(dto);
	});

	// 文字房新消息：全局监听（不随房间加入/离开注销），chatStore 负责分流到
	// 当前房间消息或「其它频道未读计数」。
	adapter.onServerEvent(EVENTS.MESSAGE_CREATED, (dto: MessageDTO) => {
		deps.handleMessageCreated(dto);
	});

	// 文字房「正在输入」广播：只记状态，由展示层按当前房间过滤 + TTL 清理
	adapter.onServerEvent(EVENTS.TYPING, (data: TypingEventPayload) => {
		applyTypingEvent(data);
	});

	// 系统公告：平台公告走 namespace 广播，域公告定向投递到 __user 房间
	adapter.onServerEvent(EVENTS.ANNOUNCEMENT_NEW, (dto: AnnouncementDTO) => {
		deps.handleAnnouncementNew(dto);
	});
}
