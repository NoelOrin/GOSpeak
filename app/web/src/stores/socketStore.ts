import type { SFUProvider } from "@gospeak/sfu-client/types";
import { toast } from "@gospeak/components";
import { getWSEndpoint } from "@/api/ws";
import { preloadSfuClient } from "@/components/room/services/loadSfuClient";
import { createWSClient } from "@/socket/wsClient";
import type { WSConnectionState } from "@/socket/wsClient";
import { EVENTS } from "@/socket/events";
import { bindServerEvents } from "@/socket/socketEvents";
import { createProviderReloadHandler } from "@/socket/providerReload";
import { upsertRoomMembersFromAck } from "@/socket/roomState";
import { notificationStore } from "@/stores/notificationStore";
import { createTabLock } from "@/socket/tabLock";
import type { PrivateMessageDTO } from "@/protocol/conversation";
import type { MessageDTO } from "@/types/message";
import { speakingStore } from "@/handler_audio/speakingStore";
import { useUserStore } from "@/stores/userStore";
import { useDomainStore } from "@/stores/domainStore";
import { create } from "zustand";
import type {
	ActivityEvent,
	MemberInfo,
	RoomInfo,
	RoomPresenceEvent,
} from "@/socket/types";

// 私聊事件通过 chatStore 注册回调消费，避免两个 store 模块互相 import 形成环依赖。
let privateNewHandler: ((dto: PrivateMessageDTO) => void) | null = null;
export function setPrivateNewHandler(
	handler: (dto: PrivateMessageDTO) => void,
) {
	privateNewHandler = handler;
}

// 文字房消息同样通过回调注册：全局监听一次，用户不在任何文字房时也要能累计未读。
let messageCreatedHandler: ((dto: MessageDTO) => void) | null = null;
export function setMessageCreatedHandler(handler: (dto: MessageDTO) => void) {
	messageCreatedHandler = handler;
}
export { EVENTS } from "@/socket/events";

export type {
	ActivityEvent,
	MemberInfo,
	RoomInfo,
	RoomPresenceEvent,
} from "@/socket/types";

interface SocketState {
	connected: boolean;
	connecting: boolean;
	socketState: WSConnectionState;
	rooms: RoomInfo[];
	currentRoom: string | null;
	selectedRoomInfo: RoomInfo | null;
	activeSFUProvider: SFUProvider | undefined;
	speechRestricted: boolean;
	speechRestrictionInfo: {
		permanent: boolean;
		expires_at: string | null;
		reason: string;
	} | null;
}

export const useSocketStore = create<SocketState>(() => ({
	connected: false,
	connecting: false,
	socketState: "new",
	rooms: [],
	currentRoom: null,
	selectedRoomInfo: null,
	activeSFUProvider: undefined,
	speechRestricted: false,
	speechRestrictionInfo: null,
}));

const tabLock = createTabLock({
	channelName: "gospeak_socket_tab",
	tabId:
		typeof crypto !== "undefined" && "randomUUID" in crypto
			? crypto.randomUUID()
			: `tab-${Date.now()}-${Math.random().toString(36).slice(2)}`,
	probeTimeoutMs: 150,
});

const handleProviderChanged = createProviderReloadHandler({
	showToast: toast as (msg: string) => void,
	preloadSfuClient,
});

const adapter = createWSClient({
	refreshEndpoint: () => getWSEndpoint(currentDomainUUID() ?? undefined),
});

const resolveUpdater = <T>(v: T | ((prev: T) => T), prev: T): T =>
	typeof v === "function" ? (v as (p: T) => T)(prev) : v;
const setConnected = (v: boolean | ((prev: boolean) => boolean)) =>
	useSocketStore.setState((prev) => ({
		connected: resolveUpdater(v, prev.connected),
	}));
const setConnecting = (v: boolean | ((prev: boolean) => boolean)) =>
	useSocketStore.setState((prev) => ({
		connecting: resolveUpdater(v, prev.connecting),
	}));
const setRooms = (updater: RoomInfo[] | ((prev: RoomInfo[]) => RoomInfo[])) =>
	useSocketStore.setState((prev) => ({
		rooms: resolveUpdater(updater, prev.rooms),
	}));
const setCurrentRoom = (
	room: string | null | ((prev: string | null) => string | null),
) =>
	useSocketStore.setState((prev) => ({
		currentRoom: resolveUpdater(room, prev.currentRoom),
	}));
const setSelectedRoomInfo = (
	info: RoomInfo | null | ((prev: RoomInfo | null) => RoomInfo | null),
) =>
	useSocketStore.setState((prev) => ({
		selectedRoomInfo: resolveUpdater(info, prev.selectedRoomInfo),
	}));
const setActiveSFUProvider = (
	p:
		| SFUProvider
		| undefined
		| ((prev: SFUProvider | undefined) => SFUProvider | undefined),
) =>
	useSocketStore.setState((prev) => ({
		activeSFUProvider: resolveUpdater(p, prev.activeSFUProvider),
	}));
const setSpeechRestricted = (v: boolean | ((prev: boolean) => boolean)) =>
	useSocketStore.setState((prev) => ({
		speechRestricted: resolveUpdater(v, prev.speechRestricted),
	}));
const setSpeechRestrictionInfo = (
	info:
		| SocketState["speechRestrictionInfo"]
		| ((
				prev: SocketState["speechRestrictionInfo"],
		  ) => SocketState["speechRestrictionInfo"]),
) =>
	useSocketStore.setState((prev) => ({
		speechRestrictionInfo: resolveUpdater(info, prev.speechRestrictionInfo),
	}));
const setSocketState = (s: WSConnectionState) =>
	useSocketStore.setState({ socketState: s });

function currentDomainUUID(): string | null {
	return useDomainStore.getState().currentDomainUUID ?? null;
}

function currentRoom(): string | null {
	return useSocketStore.getState().currentRoom;
}

/** members 派生自 rooms[] 中当前房，消除 members/rooms 双源时序竞态 */
export function selectMembers(state: SocketState): MemberInfo[] {
	const domainUUID = currentDomainUUID();
	return (
		state.rooms.find(
			(r) =>
				r.name === state.currentRoom &&
				(r.domain_uuid || "") === (domainUUID || ""),
		)?.members ?? []
	);
}

let pendingTextRejoin: { room: string; domain_uuid?: string } | null = null;
/** 信令断线后待重新注册的语音房间（媒体链路独立，通常无需重建） */
let pendingVoiceRejoin: { room: string; domain_uuid?: string } | null = null;

const activityListeners = new Set<(event: ActivityEvent) => void>();
const presenceListeners = new Set<(event: RoomPresenceEvent) => void>();
const kickedListeners = new Set<() => void>();

function emitActivity(event: ActivityEvent) {
	for (const listener of activityListeners) listener(event);
}

function emitPresence(event: RoomPresenceEvent) {
	for (const listener of presenceListeners) listener(event);
}

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

// 断线提示只在「意外掉线」时弹一次，重连成功后清掉，避免抖动刷屏
let disconnectNoticeId: string | number | null = null;
let intentionalDisconnect = false;

adapter.onConnected(() => {
	setConnecting(false);
	setConnected(true);
	console.log("[Socket] connected");
	if (disconnectNoticeId !== null) {
		toast.dismiss(disconnectNoticeId);
		disconnectNoticeId = null;
		toast.success("已重新连接");
	}
	if (pendingTextRejoin) {
		const rejoin = pendingTextRejoin;
		pendingTextRejoin = null;
		const identity = useUserStore.getState().user?.name ?? "";
		void joinRoom(rejoin.room, identity, undefined, rejoin.domain_uuid)
			.then(() => {
				console.log("[Socket] text room re-joined after worker switch");
			})
			.catch((err: unknown) => {
				console.warn("[Socket] text room re-join failed:", err);
			});
	}
	// WS 不重放断线期间的事件：连接（含重连）成功后必须拉一次未读数兜底。
	void notificationStore.loadUnreadCount();
	// 语音房重连：把信令面的房间注册补回来。
	// 媒体（LiveKit）是独立链路，通常仍连着，所以这里只补信令，不重建会话。
	if (pendingVoiceRejoin) {
		const rejoin = pendingVoiceRejoin;
		pendingVoiceRejoin = null;
		const identity = useUserStore.getState().user?.name ?? "";
		const password = (
			useSocketStore.getState().selectedRoomInfo as {
				_password?: string;
			} | null
		)?._password;
		void joinRoom(rejoin.room, identity, password, rejoin.domain_uuid)
			.then(() => {
				console.log("[Socket] voice room signal re-registered");
			})
			.catch((err: unknown) => {
				console.warn("[Socket] voice room re-register failed:", err);
				toast.error("语音房间连接已断开，请重新进入房间");
				clearSelectedRoom();
			});
	}
	// 原 solid effect：连接成功即拉取当前域房间列表
	listRooms();
});

adapter.onConnectError((err: Error) => {
	setConnecting(false);
	console.error("[Socket] connect_error:", err.message);
	toast.error(`连接服务器失败: ${err.message}`);
});

adapter.onDisconnected((reason: string) => {
	setConnecting(false);
	setConnected(false);
	const state = useSocketStore.getState();
	const prevRoom = state.currentRoom;
	const prevDomain = currentDomainUUID();
	const prevRoomMeta = prevRoom
		? state.rooms.find((r) => r.name === prevRoom)
		: undefined;
	const wasText = prevRoomMeta?.type === "text";
	const wasVoice = !!prevRoom && !wasText;
	// 临时断线：保留 currentRoom / selectedRoomInfo，让语音会话继续存活。
	// 真要离开房间时由 useVoiceSession 的 teardown 走 resetRoomState()。
	resetTransientState();
	// 主动断连（登出 / 让出标签页）不打扰用户，只有意外掉线才提示
	if (!intentionalDisconnect && useUserStore.getState().user) {
		disconnectNoticeId = toast.warning("连接已断开，正在尝试重连…", {
			duration: Number.POSITIVE_INFINITY,
		});
	}
	if (wasText && prevRoom) {
		pendingTextRejoin = {
			room: prevRoom,
			domain_uuid: prevDomain ?? undefined,
		};
	}
	// 语音房记录重连后要重新注册的信令房间（媒体连接若还活着则不用重建）
	if (wasVoice && prevRoom) {
		pendingVoiceRejoin = {
			room: prevRoom,
			domain_uuid: prevDomain ?? undefined,
		};
	}
	console.log("[Socket] disconnected:", reason);
});

let serverEventsBound = false;

async function connectWithLock(url: string): Promise<string> {
	if (adapter.isConnected()) {
		const current = adapter.getCurrentUrl?.();
		if (current === url) return current;
		adapter.disconnect();
	}
	const state = useSocketStore.getState();
	if (state.connecting) {
		throw new Error("socket connection already in progress");
	}
	// 源头禁止多标签页连接：BroadcastChannel 探测，同一浏览器仅允许一个活动标签页持有 socket
	setConnecting(true);
	const ok = await tabLock.claim();
	if (!ok) {
		setConnecting(false);
		toast.error("已在其他标签页连接，请关闭其他标签页后重试");
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
		adapter.connect(url);
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
				handlePrivateNew: (dto) => privateNewHandler?.(dto),
				handleMessageCreated: (dto) => messageCreatedHandler?.(dto),
				handleAnnouncementNew: (dto) => notificationStore.handleIncoming(dto),
				setSpeakingIdentities: (v) =>
					speakingStore.set(
						typeof v === "function" ? v(speakingStore.get()) : v,
					),
				currentUserName: () => useUserStore.getState().user?.name ?? "",
				currentUserID: () => useUserStore.getState().user?.id,
			});
		}
		return url;
	} catch (err) {
		setConnecting(false);
		toast.error(
			`获取连接地址失败: ${err instanceof Error ? err.message : String(err)}`,
		);
		throw err;
	}
}

function connect() {
	if (adapter.isConnected() || useSocketStore.getState().connecting) return;
	if (!useUserStore.getState().user) {
		toast.warning("请先登录");
		return;
	}
	const socketUrl = import.meta.env.VITE_SOCKET_URL || "";
	void connectWithLock(socketUrl).catch(() => {});
}

/** 网络抖动 / 临时断线：只清「服务端派生」状态，保留用户想待的房间意图。
 *  selectedRoomInfo / currentRoom 必须保留 —— 它们是"我要在这个房间"的本地意图，
 *  而 SFU 媒体连接与信令是两条独立的链路，信令抖动不该把语音踢掉。 */
function resetTransientState() {
	useSocketStore.setState({ rooms: [] });
	setSpeechRestricted(false);
	setSpeechRestrictionInfo(null);
	speakingStore.set([]);
}

function resetRoomState() {
	setCurrentRoom(null);
	useSocketStore.setState({ rooms: [] });
	setSelectedRoomInfo(null);
	setSpeechRestricted(false);
	setSpeechRestrictionInfo(null);
	setActiveSFUProvider(undefined);
	// 断连后旧房间的发言高亮不再有效，清空避免重连/换房时残留误亮。
	speakingStore.set([]);
}

function disconnect() {
	intentionalDisconnect = true;
	activityListeners.clear();
	presenceListeners.clear();
	kickedListeners.clear();
	adapter.offAllServerEvents();
	serverEventsBound = false;
	adapter.disconnect();
	tabLock.release();
	notificationStore.reset();
	setConnecting(false);
	setConnected(false);
	pendingTextRejoin = null;
	pendingVoiceRejoin = null;
	resetRoomState();
	intentionalDisconnect = false;
}

function createRoom(name: string, password?: string) {
	const domainUUID = currentDomainUUID();
	if (!domainUUID) {
		toast.warning("请先选择域");
		return;
	}
	const sent = adapter.emitFireAndForget(EVENTS.ROOM_CREATE, {
		room: name,
		password,
		domain_uuid: domainUUID,
	});
	if (!sent) {
		toast.error("连接未就绪，房间创建请求未发送");
	}
}

function signalEmit(
	event: string,
	payload?: Record<string, unknown>,
): Promise<any> {
	return adapter.emitAck(event, payload);
}

function clearCurrentRoom() {
	setCurrentRoom(null);
	// 离开/被踢/切房退出当前会话时清空发言态，防止陈旧 identity 残留高亮。
	speakingStore.set([]);
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
			// 切房时旧房发言态已失效；新房发言列表由 join 回放 / SFU 原生事件随后送达。
			speakingStore.set([]);
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
		useSocketStore.setState({ rooms: [] });
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
		toast.error("连接未就绪，踢出请求未发送");
	}
}

function emitMicState(room: string, identity: string, isMicMuted: boolean) {
	const sent = adapter.emitFireAndForget(EVENTS.MEMBER_MIC_STATE, {
		room,
		identity,
		isMicMuted,
		domain_uuid: currentDomainUUID() ?? undefined,
	});
	if (!sent) {
		toast.error("连接未就绪，麦克风状态未同步");
	}
}

/**
 * 文字房「正在输入」上报。
 * 纯 UI 提示：发不出去就算了，不要弹 toast 打扰用户；服务端会按 2s 节流广播。
 */
function emitTyping(room: string, domainUUID: string | null, typing: boolean) {
	if (!room) return;
	adapter.emitFireAndForget(EVENTS.TYPING, {
		room,
		domain_uuid: domainUUID ?? undefined,
		typing,
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
		if (!adapter.isConnected() && !useSocketStore.getState().connecting) return;
		toast.warning("连接已切换到其他标签页");
		disconnect();
	});
	// 确保 BroadcastChannel 已建立，能响应 probe
	tabLock.ensureListening();
}

// 原 solid effect：当前域变化时刷新房间列表（已连接状态下）
useDomainStore.subscribe(() => {
	if (useSocketStore.getState().connected) {
		listRooms();
	}
});

// 原 solid effect：登出即断连
useUserStore.subscribe((state) => {
	if (!state.user) disconnect();
});

export const socketStore = {
	connected: () => useSocketStore.getState().connected,
	connecting: () => useSocketStore.getState().connecting,
	socketState: () => useSocketStore.getState().socketState,
	rooms: () => useSocketStore.getState().rooms,
	currentRoom: () => useSocketStore.getState().currentRoom,
	currentDomainUUID,
	members: () => selectMembers(useSocketStore.getState()),
	selectedRoomInfo: () => useSocketStore.getState().selectedRoomInfo,
	activeSFUProvider: () => useSocketStore.getState().activeSFUProvider,
	speechRestricted: () => useSocketStore.getState().speechRestricted,
	speechRestrictionInfo: () => useSocketStore.getState().speechRestrictionInfo,
	connect,
	disconnect,
	connectToWorker: (workerUrl: string) => connectWithLock(workerUrl),
	createRoom,
	joinRoom,
	leaveRoom,
	joinRoomSFU,
	clearCurrentRoom,
	onActivity,
	onPresence,
	onRoomKicked,
	getSocket,
	onConnected: (cb: () => void) => adapter.onConnected(cb),
	listRooms,
	kickMember,
	emitMicState,
	emitTyping,
	selectRoom,
	clearSelectedRoom,
	setCurrentSFUProvider,
};

export default socketStore;
