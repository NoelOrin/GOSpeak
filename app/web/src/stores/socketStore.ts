// 1. imports + re-exports + module helpers
import type { SFUProvider } from "@gospeak/sfu-client/types";
import { createEffect, createMemo, createRoot, createSignal } from "solid-js";
import { showToast } from "solid-notifications";
import type { PrivateMessageDTO } from "@/api/conversation";
import { getWSTicket } from "@/api/ws";
import { preloadSfuClient } from "@/components/room/services/loadSfuClient";
// NOTE: store -> audio 写入是已知耦合；房间 UI 直接读 speakingStore。
// 若后续要彻底解耦，改为 onActiveSpeakers 订阅，由 useRoomAudioBridge/voiceChat 写入。
import { setSpeakingIdentities } from "@/handler_audio/speakingStore";
import { createWSClient } from "@/socket/wsClient";
import { EVENTS } from "@/socket/events";
import { createProviderReloadHandler } from "@/socket/providerReload";
import {
	addCreatedRoom,
	applyMemberJoinedShell,
	applyMemberLeft,
	applyMemberUpdated,
	mergeRoomUpdated,
	upsertRoomMembersFromAck,
} from "@/socket/roomState";
import { createTabLock } from "@/socket/tabLock";
import { chatStore } from "@/stores/chatStore";
import userStore from "@/stores/userStore";
import domainStore from "@/stores/domainStore";

export { EVENTS } from "@/socket/events";

// 2. tabLock / providerReload helpers
const tabLock = createTabLock({
	channelName: "gospeak_socket_tab",
	tabId:
		typeof crypto !== "undefined" && "randomUUID" in crypto
			? crypto.randomUUID()
			: `tab-${Date.now()}-${Math.random().toString(36).slice(2)}`,
	probeTimeoutMs: 150,
});

const handleProviderChanged = createProviderReloadHandler({
	showToast,
	preloadSfuClient,
});

export type {
	ActivityEvent,
	MemberInfo,
	MuteEvent,
	RoomInfo,
	RoomPresenceEvent,
	UnmuteEvent,
} from "@/socket/types";

import type {
	ActivityEvent,
	MemberInfo,
	MuteEvent,
	RoomInfo,
	RoomPresenceEvent,
	UnmuteEvent,
} from "@/socket/types";

export const socketStore = createRoot(() => {
	// 3. adapter + signals
	const adapter = createWSClient({ refreshTicket: getWSTicket });

	const [connected, setConnected] = createSignal(false);
	const [rooms, setRooms] = createSignal<RoomInfo[]>([]);
	const [currentRoom, setCurrentRoom] = createSignal<string | null>(null);
	const currentDomainUUID = createMemo<string | null>(
		() => domainStore.state.currentDomainUUID ?? null,
	);
	// members 派生自 rooms[] 中当前房，消除 members/rooms 双源时序竞态
	const members = createMemo<MemberInfo[]>(
		() =>
			rooms().find(
				(r) =>
					r.name === currentRoom() &&
					(r.domain_uuid || "") === (currentDomainUUID() || ""),
			)?.members ?? [],
	);

	createEffect(() => {
		if (connected()) {
			void currentDomainUUID();
			listRooms();
		}
	});
	const [selectedRoomInfo, setSelectedRoomInfo] = createSignal<RoomInfo | null>(
		null,
	);
	const [activeSFUProvider, setActiveSFUProvider] = createSignal<
		SFUProvider | undefined
	>(undefined);
	const [speechRestricted, setSpeechRestricted] = createSignal<boolean>(false);
	const [speechRestrictionInfo, setSpeechRestrictionInfo] = createSignal<{
		permanent: boolean;
		expires_at: string | null;
		reason: string;
	} | null>(null);
	const [connecting, setConnecting] = createSignal(false);

	// 4. listener sets (activity/presence/kicked)
	const activityListeners = new Set<(event: ActivityEvent) => void>();
	const presenceListeners = new Set<(event: RoomPresenceEvent) => void>();
	const kickedListeners = new Set<() => void>();

	function emitActivity(event: ActivityEvent) {
		for (const listener of activityListeners) listener(event);
	}

	function emitPresence(event: RoomPresenceEvent) {
		for (const listener of presenceListeners) listener(event);
	}

	// 5. lifecycle callbacks (onConnected/onDisconnected/onConnectError)
	// 生命周期回调：注册一次，每次 connect/disconnect/connect_error 都会触发
	adapter.onConnected(() => {
		setConnecting(false);
		setConnected(true);
		console.log("[Socket] connected");
	});

	adapter.onConnectError((err: Error) => {
		setConnecting(false);
		console.error("[Socket] connect_error:", err.message);
		showToast(`连接服务器失败: ${err.message}`, { type: "error" });
	});

	adapter.onDisconnected((reason: string) => {
		setConnecting(false);
		setConnected(false);
		console.log("[Socket] disconnected:", reason);
	});

	// 6. connect / bindServerEvents / disconnect
	let serverEventsBound = false;

	function connect() {
		if (adapter.isConnected() || connecting()) return;
		const token = userStore.accessToken();
		if (!token) {
			showToast("请先登录", { type: "warning" });
			return;
		}
		// 源头禁止多标签页连接：BroadcastChannel 探测，同一浏览器仅允许一个活动标签页持有 socket
		setConnecting(true);
		void tabLock.claim().then(async (ok) => {
			if (!ok) {
				setConnecting(false);
				showToast("已在其他标签页连接，请关闭其他标签页后重试", {
					type: "error",
				});
				return;
			}
			if (adapter.isConnected()) {
				setConnecting(false);
				return;
			}
			try {
				const socketUrl = import.meta.env.VITE_SOCKET_URL || "";
				const ticket = await getWSTicket();
				adapter.connect(socketUrl, ticket);
				if (serverEventsBound) return;
				serverEventsBound = true;
				bindServerEvents();
			} catch (err) {
				setConnecting(false);
				showToast(
					`获取连接凭证失败: ${err instanceof Error ? err.message : String(err)}`,
					{ type: "error" },
				);
			}
		});
	}

	function bindServerEvents() {
		adapter.onServerEvent(
			EVENTS.ROOM_CREATED,
			(room: RoomInfo & { error?: string }) => {
				console.log("[Socket] room:created", room.name, room.error);
				if (room.error) {
					showToast(`创建房间失败: ${room.error}`, { type: "error" });
					return;
				}
				setRooms((prev) => addCreatedRoom(prev, room));
			},
		);

		adapter.onServerEvent(EVENTS.ROOM_UPDATED, (room: RoomInfo) => {
			if (!room?.name) return;
			console.log("[Socket] room:updated", room.name, "count=", room.count);
			// 仅覆盖服务端实际返回的字段（name/hasPassword/members/count/createdAt），
			// 保留 DB 字段（id/uuid/description/limit/audioOnly/allowAudience）不被零值覆盖。
			// members 由 rooms[] 派生，无需单独维护。
			setRooms((prev) => mergeRoomUpdated(prev, room));
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
				// 即时追加 shell 成员，富化字段由后续 room:updated 覆盖。
				setRooms((prev) => applyMemberJoinedShell(prev, data));
				emitActivity({
					type: "member_joined",
					room: data.room,
					domain_uuid: data.domain_uuid,
					identity: data.identity,
					timestamp: Date.now(),
				});
				emitPresence({
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
				setRooms((prev) => applyMemberLeft(prev, data));
				emitActivity({
					type: "member_left",
					room: data.room,
					domain_uuid: data.domain_uuid,
					identity: data.identity,
					timestamp: Date.now(),
				});
				emitPresence({
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
				isMicMuted: boolean;
				domain_uuid?: string;
			}) => {
				// members 由 rooms[] 派生，仅更新 rooms[] 即可
				setRooms((prev) => applyMemberUpdated(prev, data));
			},
		);

		adapter.onServerEvent(
			EVENTS.ROOM_LIST_RESULT as string,
			(data: { rooms: RoomInfo[]; count: number }) => {
				console.log("[Socket] room:list:result", data.rooms.length, "rooms");
				setRooms(data.rooms);
			},
		);

		adapter.onServerEvent(
			EVENTS.ROOM_KICKED,
			(data: {
				room: string;
				targetIdentity: string;
				enforcement?: string;
				domain_uuid?: string;
			}) => {
				console.log(
					"[Socket] room:kicked",
					data.room,
					data.targetIdentity,
					data.enforcement,
				);
				if (data.targetIdentity !== userStore.user()?.name) return;
				const kickMsg =
					data.enforcement === "soft"
						? "你已被移出房间（软踢：请断开本地媒体）"
						: data.enforcement === "degraded"
							? "你已被移出房间（降级强制）"
							: "你已被移出房间";
				showToast(kickMsg, { type: "error" });
				setCurrentRoom(null);
				setSelectedRoomInfo(null);
				for (const listener of kickedListeners) listener();
			},
		);

		// 用户级禁言事件：允许收听，不允许发布本地音轨
		// 必须按 user_id 过滤，避免全服广播误伤其他客户端
		adapter.onServerEvent(
			EVENTS.USER_MUTED,
			(data: MuteEvent & { enforcement?: string }) => {
				console.log("[Socket] user:muted", data.user_id, data.enforcement);
				if (data.user_id !== userStore.user()?.id) return;
				const mode =
					data.enforcement === "hard"
						? "服务端原生强制停止推流"
						: data.enforcement === "degraded"
							? "服务端降级强制停止推流"
							: "请停止本地推流（软禁言）";
				showToast(
					data.permanent
						? `你已被永久禁言，当前为仅收听模式（${mode}）`
						: `你已被禁言，当前为仅收听模式（${mode}）${data.reason ? `，原因: ${data.reason}` : ""}`,
					{ type: "warning" },
				);
				setSpeechRestricted(true);
				setSpeechRestrictionInfo({
					permanent: data.permanent,
					expires_at: data.expires_at,
					reason: data.reason,
				});
			},
		);

		adapter.onServerEvent(EVENTS.USER_UNMUTED, (data: UnmuteEvent) => {
			console.log("[Socket] user:unmuted", data.user_id);
			if (data.user_id !== userStore.user()?.id) return;
			showToast("你的禁言已被解除，可以重新发言", { type: "success" });
			setSpeechRestricted(false);
			setSpeechRestrictionInfo(null);
		});
		// 发言检测（SRS / Cloudflare）：信令层聚合后广播房间级 active speakers
		adapter.onServerEvent(
			EVENTS.ROOM_ACTIVE_SPEAKERS,
			(event: {
				room?: string;
				identities?: string[];
				domain_uuid?: string;
			}) => {
				const room = event?.room;
				if (
					room &&
					(room !== currentRoom() ||
						(event.domain_uuid || "") !== (currentDomainUUID() || ""))
				)
					return;
				setSpeakingIdentities(event?.identities ?? []);
			},
		);

		// SFU 热切换：强制断连并 0.5s 后刷新；join 中途同样直接刷新
		adapter.onServerEvent(
			EVENTS.SFU_PROVIDER_CHANGED,
			(data: { provider?: string }) => {
				// 先断 socket，再刷新
				try {
					adapter.disconnect();
				} catch {
					// ignore
				}
				setConnecting(false);
				setConnected(false);
				setCurrentRoom(null);
				setSelectedRoomInfo(null);
				setActiveSFUProvider(undefined);
				handleProviderChanged(data?.provider);
			},
		);

		// 全局私聊新消息监听：即使用户未打开任何会话也能收到通知
		adapter.onServerEvent(EVENTS.PRIVATE_NEW, (dto: PrivateMessageDTO) => {
			chatStore.handlePrivateNew(dto);
		});
	}

	function disconnect() {
		activityListeners.clear();
		presenceListeners.clear();
		kickedListeners.clear();
		adapter.offAllServerEvents();
		serverEventsBound = false;
		adapter.disconnect();
		tabLock.release();
		setConnecting(false);
		setConnected(false);
		setCurrentRoom(null);
		setSpeechRestricted(false);
		setSpeechRestrictionInfo(null);
		setActiveSFUProvider(undefined);
	}

	// 7. room APIs (create/join/leave/list/kick/select)
	function createRoom(name: string, password?: string) {
		adapter.emitFireAndForget(EVENTS.ROOM_CREATE, {
			room: name,
			password,
			domain_uuid: currentDomainUUID() ?? undefined,
		});
	}

	function signalEmit(
		event: string,
		payload?: Record<string, unknown>,
	): Promise<any> {
		return adapter.emitAck(event, payload);
	}

	createEffect(() => {
		if (!userStore.accessToken()) disconnect();
	});

	function clearCurrentRoom() {
		setCurrentRoom(null);
	}

	function waitForConnected(timeoutMs = 8000): Promise<void> {
		if (adapter.isConnected()) return Promise.resolve();
		// 进房时 socket 可能仍在握手；短等连接，避免 emit 丢进未连接队列后永久无 ack。
		return new Promise((resolve, reject) => {
			const timer = setTimeout(() => {
				off();
				reject(new Error("socket connect timeout"));
			}, timeoutMs);
			const off = adapter.onConnected(() => {
				clearTimeout(timer);
				off();
				resolve();
			});
			if (adapter.isConnected()) {
				clearTimeout(timer);
				off();
				resolve();
			}
		});
	}

	function joinRoom(room: string, identity: string, password?: string) {
		return waitForConnected()
			.then(() =>
				signalEmit(EVENTS.ROOM_JOIN, {
					room,
					identity,
					password,
					domain_uuid: currentDomainUUID() ?? undefined,
				}),
			)
			.then((data) => {
				if (data.error) {
					throw new Error(data.error);
				}
				setCurrentRoom(data.room);
				return data;
			});
	}

	function leaveRoom(room: string): Promise<any> {
		// 仅发信令，不清 currentRoom。状态清理由 session 生命周期统一管理
		// （teardownSession / handleManualLeave / room:kicked），避免切房 fire-and-forget 时
		// .then 清状态与新房 joinRoom 设状态竞态。
		// 离开者已不在 WS room，收不到 member:left；namespace 广播
		// room:updated / room:list:result 负责刷新列表。ack 后再拉一次列表兜底。
		return signalEmit(EVENTS.ROOM_LEAVE, {
			room,
			domain_uuid: currentDomainUUID() ?? undefined,
		}).then((data) => {
			listRooms();
			return data;
		});
	}

	function joinRoomSFU(room: string, identity: string, stream?: string) {
		return waitForConnected()
			.then(() =>
				signalEmit(EVENTS.ROOM_JOIN_SFU, {
					room,
					identity,
					stream,
					domain_uuid: currentDomainUUID() ?? undefined,
				}),
			)
			.then((data) => {
				// ack 返回完整成员列表，即时 upsert rooms[]（不等 room:updated）
				if (data.members) {
					const ackMembers: MemberInfo[] = data.members;
					setRooms((prev) =>
						upsertRoomMembersFromAck(
							prev,
							data.room,
							currentDomainUUID() ?? undefined,
							ackMembers,
						),
					);
				}
				emitActivity({
					type: "room_joined",
					room: data.room,
					identity: data.identity,
					timestamp: Date.now(),
				});
				return data;
			});
	}

	function onActivity(cb: (event: ActivityEvent) => void): () => void {
		activityListeners.add(cb);
		return () => {
			activityListeners.delete(cb);
		};
	}

	function onPresence(cb: (event: RoomPresenceEvent) => void): () => void {
		presenceListeners.add(cb);
		return () => {
			presenceListeners.delete(cb);
		};
	}

	function onRoomKicked(cb: () => void): () => void {
		kickedListeners.add(cb);
		return () => {
			kickedListeners.delete(cb);
		};
	}

	function getSocket() {
		return adapter;
	}

	function listRooms() {
		adapter.emitFireAndForget(EVENTS.ROOM_LIST, {
			domain_uuid: currentDomainUUID() ?? undefined,
		});
	}

	function kickMember(room: string, targetIdentity: string) {
		adapter.emitFireAndForget(EVENTS.ROOM_KICK, {
			room,
			targetIdentity,
			domain_uuid: currentDomainUUID() ?? undefined,
		});
	}

	// 9. mic/speaking emits
	function emitMicState(room: string, identity: string, isMicMuted: boolean) {
		adapter.emitFireAndForget(EVENTS.MEMBER_MIC_STATE, {
			room,
			identity,
			isMicMuted,
			domain_uuid: currentDomainUUID() ?? undefined,
		});
	}

	function emitSpeaking(room: string, identity: string, speaking: boolean) {
		adapter.emitFireAndForget(EVENTS.MEMBER_SPEAKING, {
			room,
			identity,
			speaking,
			domain_uuid: currentDomainUUID() ?? undefined,
		});
	}

	function selectRoom(room: RoomInfo) {
		setSelectedRoomInfo(room);
	}

	function clearSelectedRoom() {
		setSelectedRoomInfo(null);
	}

	function setCurrentSFUProvider(provider?: SFUProvider) {
		setActiveSFUProvider(provider);
	}

	// 标签页关闭时释放 owner，允许其他标签页接管
	if (typeof window !== "undefined") {
		window.addEventListener("beforeunload", () => {
			tabLock.release();
		});
		// 若异常出现他页 claimed，本页主动让出
		tabLock.setOnForeignClaim(() => {
			if (!adapter.isConnected() && !connecting()) return;
			showToast("连接已切换到其他标签页", { type: "warning" });
			disconnect();
		});
		// 确保 BroadcastChannel 已建立，能响应 probe
		tabLock.ensureListening();
	}

	// 10. return public API
	return {
		connected,
		connecting,
		rooms,
		currentRoom,
		currentDomainUUID,
		members,
		selectedRoomInfo,
		activeSFUProvider,
		speechRestricted,
		speechRestrictionInfo,
		connect,
		disconnect,
		createRoom,
		joinRoom,
		leaveRoom,
		joinRoomSFU,
		clearCurrentRoom,
		onActivity,
		onPresence,
		onRoomKicked,
		getSocket,
		listRooms,
		kickMember,
		emitMicState,
		emitSpeaking,
		selectRoom,
		clearSelectedRoom,
		setCurrentSFUProvider,
	};
});
