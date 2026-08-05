// 1. imports + re-exports + module helpers
import type { SFUProvider } from "@gospeak/sfu-client/types";
import { createEffect, createMemo, createRoot, createSignal } from "solid-js";
import { showToast } from "solid-notifications";
import { getWSTicket } from "@/api/ws";
import { preloadSfuClient } from "@/components/room/services/loadSfuClient";
import { createWSClient } from "@/socket/wsClient";
import type { WSConnectionState } from "@/socket/wsClient";
import { EVENTS } from "@/socket/events";
import { bindServerEvents } from "@/socket/socketEvents";
import { createProviderReloadHandler } from "@/socket/providerReload";
import { upsertRoomMembersFromAck } from "@/socket/roomState";
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
	RoomInfo,
	RoomPresenceEvent,
} from "@/socket/types";

export const socketStore = createRoot(() => {
	// 3. adapter + signals
	const adapter = createWSClient({ refreshTicket: getWSTicket });

	const [connected, setConnected] = createSignal(false);
	const [socketState, setSocketState] = createSignal<WSConnectionState>(
		adapter.getState(),
	);
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
	adapter.onStateChange((_prev, next) => {
		setSocketState(next);
		if (next === "open") {
			setConnecting(false);
			setConnected(true);
		} else if (next === "connecting") {
			setConnecting(true);
			setConnected(false);
		} else {
			setConnecting(false);
			setConnected(false);
		}
	});

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
		// 被动断线同样清理房间/禁言状态，避免重连窗口显示假在线/假禁言。
		resetRoomState();
		console.log("[Socket] disconnected:", reason);
	});

	// 6. connect / bindServerEvents / disconnect
	let serverEventsBound = false;

	async function connectWithLock(url: string): Promise<string> {
		if (adapter.isConnected()) {
			const current = adapter.getCurrentUrl?.();
			if (current === url) return current;
			adapter.disconnect();
		}
		if (connecting()) {
			throw new Error("socket connection already in progress");
		}
		// 源头禁止多标签页连接：BroadcastChannel 探测，同一浏览器仅允许一个活动标签页持有 socket
		setConnecting(true);
		const ok = await tabLock.claim();
		if (!ok) {
			setConnecting(false);
			showToast("已在其他标签页连接，请关闭其他标签页后重试", {
				type: "error",
			});
			throw new Error("已在其他标签页连接，请关闭其他标签页后重试");
		}
		if (adapter.isConnected()) {
			const current = adapter.getCurrentUrl?.();
			if (current === url) {
				setConnecting(false);
				return current;
			}
			adapter.disconnect();
		}
		try {
			const ticket = await getWSTicket();
			adapter.connect(url, ticket);
			if (!serverEventsBound) {
				serverEventsBound = true;
				bindServerEvents(adapter, {
					setRooms,
					setCurrentRoom,
					setSelectedRoomInfo,
					setActiveSFUProvider,
					setConnecting,
					setConnected,
					setSpeechRestricted,
					setSpeechRestrictionInfo,
					currentRoom,
					currentDomainUUID,
					emitActivity,
					emitPresence,
					emitKicked: () => {
						for (const listener of kickedListeners) listener();
					},
					handleProviderChanged,
					handlePrivateNew: (dto) => chatStore.handlePrivateNew(dto),
					currentUserName: () => userStore.user()?.name ?? "",
					currentUserID: () => userStore.user()?.id,
				});
			}
			return url;
		} catch (err) {
			setConnecting(false);
			showToast(
				`获取连接凭证失败: ${err instanceof Error ? err.message : String(err)}`,
				{ type: "error" },
			);
			throw err;
		}
	}

	function connect() {
		if (adapter.isConnected() || connecting()) return;
		const token = userStore.accessToken();
		if (!token) {
			showToast("请先登录", { type: "warning" });
			return;
		}
		const socketUrl = import.meta.env.VITE_SOCKET_URL || "";
		void connectWithLock(socketUrl).catch(() => {});
	}

	async function connectToWorker(workerUrl: string): Promise<string> {
		return connectWithLock(workerUrl);
	}

	function resetRoomState() {
		setCurrentRoom(null);
		setRooms([]);
		setSelectedRoomInfo(null);
		setSpeechRestricted(false);
		setSpeechRestrictionInfo(null);
		setActiveSFUProvider(undefined);
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
		resetRoomState();
	}

	// 7. room APIs (create/join/leave/list/kick/select)
	function createRoom(name: string, password?: string) {
		const domainUUID = currentDomainUUID();
		if (!domainUUID) {
			showToast("请先选择域", { type: "warning" });
			return;
		}
		const sent = adapter.emitFireAndForget(EVENTS.ROOM_CREATE, {
			room: name,
			password,
			domain_uuid: domainUUID,
		});
		if (!sent) {
			showToast("连接未就绪，房间创建请求未发送", { type: "error" });
		}
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

	function joinRoom(
		room: string,
		identity: string,
		password?: string,
		domain_uuid?: string,
	) {
		return waitForConnected()
			.then(() =>
				signalEmit(EVENTS.ROOM_JOIN, {
					room,
					identity,
					password,
					domain_uuid: domain_uuid ?? currentDomainUUID() ?? undefined,
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

	function leaveRoom(room: string, domain_uuid?: string): Promise<any> {
		// 仅发信令，不清 currentRoom。状态清理由 session 生命周期统一管理
		// （teardownSession / handleManualLeave / room:kicked），避免切房 fire-and-forget 时
		// .then 清状态与新房 joinRoom 设状态竞态。
		// 离开者已不在 WS room，收不到 member:left；namespace 广播
		// room:updated / room:list:result 负责刷新列表。ack 后再拉一次列表兜底。
		return signalEmit(EVENTS.ROOM_LEAVE, {
			room,
			domain_uuid: domain_uuid ?? currentDomainUUID() ?? undefined,
		}).then((data) => {
			listRooms();
			return data;
		});
	}

	function joinRoomSFU(
		room: string,
		identity: string,
		stream?: string,
		domain_uuid?: string,
	) {
		return waitForConnected()
			.then(() =>
				signalEmit(EVENTS.ROOM_JOIN_SFU, {
					room,
					identity,
					stream,
					domain_uuid: domain_uuid ?? currentDomainUUID() ?? undefined,
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
							domain_uuid ?? currentDomainUUID() ?? undefined,
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
		const domainUUID = currentDomainUUID();
		if (!domainUUID) {
			setRooms([]);
			return;
		}
		adapter.emitFireAndForget(EVENTS.ROOM_LIST, { domain_uuid: domainUUID });
	}

	function kickMember(room: string, targetIdentity: string) {
		const sent = adapter.emitFireAndForget(EVENTS.ROOM_KICK, {
			room,
			targetIdentity,
			domain_uuid: currentDomainUUID() ?? undefined,
		});
		if (!sent) {
			showToast("连接未就绪，踢出请求未发送", { type: "error" });
		}
	}

	// 9. mic/speaking emits
	function emitMicState(room: string, identity: string, isMicMuted: boolean) {
		const sent = adapter.emitFireAndForget(EVENTS.MEMBER_MIC_STATE, {
			room,
			identity,
			isMicMuted,
			domain_uuid: currentDomainUUID() ?? undefined,
		});
		if (!sent) {
			showToast("连接未就绪，麦克风状态未同步", { type: "error" });
		}
	}

	function emitSpeaking(room: string, identity: string, speaking: boolean) {
		const sent = adapter.emitFireAndForget(EVENTS.MEMBER_SPEAKING, {
			room,
			identity,
			speaking,
			domain_uuid: currentDomainUUID() ?? undefined,
		});
		if (!sent) {
			showToast("连接未就绪，发言状态未同步", { type: "error" });
		}
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
		socketState,
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
		connectToWorker,
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
