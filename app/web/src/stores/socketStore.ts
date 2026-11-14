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
import { createTabLock } from "@/socket/tabLock";
import type { PrivateMessageDTO } from "@/protocol/conversation";
import type { AnnouncementDTO } from "@/protocol/announcement";
import type { MessageDTO } from "@/types/message";
import { useUserStore } from "@/stores/userStore";
import { useDomainStore } from "@/stores/domainStore";
import { create } from "zustand";
import type {
	ActivityEvent,
	MemberInfo,
	RoomInfo,
	RoomPresenceEvent,
} from "@/socket/types";

// 私聊/文字房/公告事件通过注册回调消费：全局监听一次，由各 store 自行驱动，
// 避免 socketStore 与业务 store 互相 import 形成环依赖（socketStore 只负责连接）。
let privateNewHandler: ((dto: PrivateMessageDTO) => void) | null = null;
export function setPrivateNewHandler(
	handler: (dto: PrivateMessageDTO) => void,
) {
	privateNewHandler = handler;
}

let messageCreatedHandler: ((dto: MessageDTO) => void) | null = null;
export function setMessageCreatedHandler(handler: (dto: MessageDTO) => void) {
	messageCreatedHandler = handler;
}

let announcementHandler: ((dto: AnnouncementDTO) => void) | null = null;
export function setAnnouncementHandler(
	handler: (dto: AnnouncementDTO) => void,
) {
	announcementHandler = handler;
}

export { EVENTS } from "@/socket/events";

export type {
	ActivityEvent,
	MemberInfo,
	RoomInfo,
	RoomPresenceEvent,
} from "@/socket/types";

/** 用户意图：想留在哪个房间。断线重连时由此派生 rejoin，与服务端派生状态分层。 */
export interface RoomRejoinIntent {
	room: string;
	domain_uuid?: string;
	/** 文字房需完整 re-join；语音房只补信令注册，媒体重建走 voice session 状态机 */
	kind: "text" | "voice";
}

export interface SocketState {
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
	/** 发言高亮 identities（signal 聚合 + SFU 原生事件双源写入），事实源在本 store */
	speakingIdentities: string[];
	/** 用户意图：想留在的房间；重连时由此派生 rejoin */
	rejoinIntent: RoomRejoinIntent | null;
	/** 意外掉线提示 toast id，重连成功后 dismiss */
	disconnectNoticeId: string | number | null;
	/** server 事件绑定状态：一次连接生命周期内只绑定一次 */
	serverEventsBound: boolean;
	/** 主动断连（登出 / 让出标签页）：true 时不弹掉线提示 */
	intentionalDisconnect: boolean;
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
	speakingIdentities: [],
	rejoinIntent: null,
	disconnectNoticeId: null,
	serverEventsBound: false,
	intentionalDisconnect: false,
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
	canRejoin: () =>
		!!useUserStore.getState().user && !!useSocketStore.getState().rejoinIntent,
	attemptRejoin: () => {
		// 语音会话的完整重建交给挂载中的 voice session（拆旧媒体 + refetch token），
		// 清掉语音意图避免重连后 connected 钩子再做一次重复的信令注册。
		const intent = useSocketStore.getState().rejoinIntent;
		if (intent?.kind === "voice") setRejoinIntent(null);
		for (const listener of voiceRebuildListeners) listener();
		connect();
	},
	isConnected: () => adapter.isConnected(),
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
	useSocketStore.setState((prev) => {
		const next = resolveUpdater(room, prev.currentRoom);
		return {
			currentRoom: next,
			// currentRoom 清空 = 用户不再想留在任何房间，rejoin 意图一并撤销
			...(next === null ? { rejoinIntent: null } : {}),
		};
	});
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
const setSpeakingIdentities = (v: string[] | ((prev: string[]) => string[])) =>
	useSocketStore.setState((prev) => ({
		speakingIdentities: resolveUpdater(v, prev.speakingIdentities),
	}));
const setRejoinIntent = (intent: RoomRejoinIntent | null) =>
	useSocketStore.setState({ rejoinIntent: intent });
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

const activityListeners = new Set<(event: ActivityEvent) => void>();
const presenceListeners = new Set<(event: RoomPresenceEvent) => void>();
const kickedListeners = new Set<() => void>();
/** SFU provider 切换时的 voice session 原地重建回调（useVoiceSession 注册） */
const voiceRebuildListeners = new Set<() => void>();

export function onVoiceSessionRebuild(cb: () => void): () => void {
	voiceRebuildListeners.add(cb);
	return () => {
		voiceRebuildListeners.delete(cb);
	};
}

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

adapter.onConnected(() => {
	setConnecting(false);
	setConnected(true);
	console.log("[Socket] connected");
	const noticeId = useSocketStore.getState().disconnectNoticeId;
	if (noticeId !== null) {
		toast.dismiss(noticeId);
		useSocketStore.setState({ disconnectNoticeId: null });
		toast.success("已重新连接");
	}
	// 重连时的 rejoin 由用户意图（rejoinIntent）派生，而非手工维护的 pending 标志
	const intent = useSocketStore.getState().rejoinIntent;
	if (intent?.kind === "text") {
		const identity = useUserStore.getState().user?.name ?? "";
		void joinRoom(intent.room, identity, undefined, intent.domain_uuid)
			.then(() => {
				console.log("[Socket] text room re-joined after reconnect");
			})
			.catch((err: unknown) => {
				// 本轮 rejoin 放弃：清意图，等用户手动进房
				setRejoinIntent(null);
				console.warn("[Socket] text room re-join failed:", err);
			});
	}
	// 语音房：只补信令面的注册（媒体链路独立，通常仍连着，不重建会话）。
	if (intent?.kind === "voice") {
		const identity = useUserStore.getState().user?.name ?? "";
		const password = (
			useSocketStore.getState().selectedRoomInfo as {
				_password?: string;
			} | null
		)?._password;
		void joinRoom(intent.room, identity, password, intent.domain_uuid)
			.then(() => {
				console.log("[Socket] voice room signal re-registered");
			})
			.catch((err: unknown) => {
				// 语音会话 UI 已随失败关闭：清意图，避免下次重连幽灵重注册
				setRejoinIntent(null);
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
	// 临时断线：保留 currentRoom / selectedRoomInfo / rejoinIntent，
	// 让语音会话继续存活。真要离开房间时由 useVoiceSession 的 teardown 走 resetRoomState()。
	resetTransientState();
	// 掉线时以服务端派生的房间元数据修正意图的 kind（join 时房间可能尚未入列表）
	if (prevRoom) {
		setRejoinIntent({
			room: prevRoom,
			domain_uuid: prevDomain ?? undefined,
			kind: wasText ? "text" : "voice",
		});
	}
	// 主动断连（登出 / 让出标签页）不打扰用户，只有意外掉线才提示
	if (!state.intentionalDisconnect && useUserStore.getState().user) {
		const noticeId = toast.warning("连接已断开，正在尝试重连…", {
			duration: Number.POSITIVE_INFINITY,
		});
		useSocketStore.setState({ disconnectNoticeId: noticeId });
	}
	console.log("[Socket] disconnected:", reason);
});

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
	// 源头禁止多标签页连接：Web Locks（降级 BroadcastChannel probe），同一浏览器仅一个活动标签页持有 socket
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
		if (!useSocketStore.getState().serverEventsBound) {
			useSocketStore.setState({ serverEventsBound: true });
			bindServerEvents(adapter, {
				setRooms,
				setCurrentRoom,
				setSelectedRoomInfo,
				setActiveSFUProvider,
				setConnecting,
				setConnected,
				setSpeechRestricted,
				setSpeechRestrictionInfo,
				setSpeakingIdentities,
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
				handleAnnouncementNew: (dto) => announcementHandler?.(dto),
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
 *  selectedRoomInfo / currentRoom / rejoinIntent 必须保留 —— 它们是"我要在这个房间"的
 *  本地意图，而 SFU 媒体连接与信令是两条独立的链路，信令抖动不该把语音踢掉。 */
function resetTransientState() {
	useSocketStore.setState({ rooms: [] });
	setSpeechRestricted(false);
	setSpeechRestrictionInfo(null);
	setSpeakingIdentities([]);
}

function resetRoomState() {
	setCurrentRoom(null);
	useSocketStore.setState({ rooms: [] });
	setSelectedRoomInfo(null);
	setSpeechRestricted(false);
	setSpeechRestrictionInfo(null);
	setActiveSFUProvider(undefined);
	// 断连后旧房间的发言高亮不再有效，清空避免重连/换房时残留误亮。
	setSpeakingIdentities([]);
}

function disconnect() {
	useSocketStore.setState({ intentionalDisconnect: true });
	activityListeners.clear();
	presenceListeners.clear();
	kickedListeners.clear();
	adapter.offAllServerEvents();
	useSocketStore.setState({ serverEventsBound: false });
	adapter.disconnect();
	tabLock.release();
	setConnecting(false);
	setConnected(false);
	const noticeId = useSocketStore.getState().disconnectNoticeId;
	if (noticeId !== null) {
		toast.dismiss(noticeId);
		useSocketStore.setState({ disconnectNoticeId: null });
	}
	resetRoomState();
	useSocketStore.setState({ intentionalDisconnect: false });
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
	// 离开/被踢/切房退出当前会话：currentRoom 置空同时撤销 rejoin 意图，
	// 并清空发言态，防止陈旧 identity 残留高亮。
	setCurrentRoom(null);
	setSpeakingIdentities([]);
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
			const resolvedDomain = domain_uuid ?? currentDomainUUID() ?? undefined;
			setCurrentRoom(data.room);
			// 记录用户意图：想留在这个房间（断线重连时由此派生 rejoin）。
			// kind 以本地 rooms 元数据判别，掉线时会再按最新元数据修正。
			const roomMeta = useSocketStore
				.getState()
				.rooms.find(
					(r) =>
						r.name === data.room &&
						(r.domain_uuid || "") === (resolvedDomain || ""),
				);
			setRejoinIntent({
				room: data.room,
				domain_uuid: resolvedDomain,
				kind: roomMeta?.type === "text" ? "text" : "voice",
			});
			// 切房时旧房发言态已失效；新房发言列表由 join 回放 / SFU 原生事件随后送达。
			setSpeakingIdentities([]);
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
	// 确保监听已建立，能响应他页 probe（Web Locks 模式下为 no-op）
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
	speakingIdentities: () => useSocketStore.getState().speakingIdentities,
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
	onVoiceSessionRebuild,
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
