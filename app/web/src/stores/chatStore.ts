import { toast } from "@gospeak/components";
import { create } from "zustand";
import {
	type ConversationDTO,
	getConversationMessages,
	listConversations,
	markConversationRead,
	type PrivateMessageDTO,
} from "@/api/conversation";
import { listMessages } from "@/api/message";
import { EVENTS } from "@/socket/events";
import {
	socketStore,
	setPrivateNewHandler,
	setMessageCreatedHandler,
} from "@/stores/socketStore";
import { useUserStore } from "@/stores/userStore";
import { useDomainStore } from "@/stores/domainStore";
import type { MessageDTO } from "@/types/message";
import {
	isRoomPasswordError,
	localizedApiMessage,
} from "@/utils/errorMessages";
import { chatCache } from "@/utils/idb-cache";

import {
	mergeMessages,
	mergePrivateMessages,
	remapConversationList,
	remapPrivateMessages,
	remapRecordKey,
} from "@/utils/chat-merge";

export {
	mergeMessages,
	mergePrivateMessages,
	remapConversationList,
	remapPrivateMessages,
	remapRecordKey,
};

interface ChatState {
	textRoom: string | null;
	textRoomName: string | null;
	textRoomDomainUUID: string | null;
	/** 进入私密文字房时用户输入的密码：历史消息接口也要用它，否则会被服务端 403 */
	textRoomPassword: string | null;
	/** 文字房加载失败的原因（空串表示正常）；密码错时 UI 需要给"重新输入密码"入口 */
	textRoomError: string;
	textRoomNeedsPassword: boolean;
	messages: MessageDTO[];
	hasMore: boolean;
	nextBefore: string | null;
	isAtBottom: boolean;
	loading: boolean;
	reactions: Record<string, Record<string, string[]>>;
	conversations: ConversationDTO[];
	activeConversationID: string | null;
	pmMessages: Record<string, PrivateMessageDTO[]>;
	pmHasMore: Record<string, boolean>;
	pmNextCursor: Record<string, string | null>;
	loadingList: boolean;
	loadingMessages: Record<string, boolean>;
	/** 文字频道未读数：key = room_uuid，进入频道清零，本地持久化以跨刷新保留 */
	unreadByRoom: Record<string, number>;
}

const UNREAD_STORAGE_KEY = "gospeak_text_unread";

function loadUnread(): Record<string, number> {
	try {
		const raw = localStorage.getItem(UNREAD_STORAGE_KEY);
		if (!raw) return {};
		const parsed: unknown = JSON.parse(raw);
		if (!parsed || typeof parsed !== "object") return {};
		const result: Record<string, number> = {};
		for (const [key, value] of Object.entries(parsed)) {
			if (typeof value === "number" && Number.isFinite(value) && value > 0) {
				result[key] = Math.floor(value);
			}
		}
		return result;
	} catch {
		return {};
	}
}

function persistUnread(map: Record<string, number>) {
	try {
		localStorage.setItem(UNREAD_STORAGE_KEY, JSON.stringify(map));
	} catch {
		// 隐私模式 / 存储配额不足：未读只保留在内存里，不影响聊天
	}
}

export const useChatStore = create<ChatState>(() => ({
	textRoom: null,
	textRoomName: null,
	textRoomDomainUUID: null,
	textRoomPassword: null,
	textRoomError: "",
	textRoomNeedsPassword: false,
	messages: [],
	hasMore: false,
	nextBefore: null,
	isAtBottom: true,
	loading: false,
	reactions: {},
	conversations: [],
	activeConversationID: null,
	pmMessages: {},
	pmHasMore: {},
	pmNextCursor: {},
	loadingList: false,
	loadingMessages: {},
	unreadByRoom: loadUnread(),
}));

const getState = () => useChatStore.getState();

// Nonces waiting for server ack / created event.
// 状态机：pending -> sent/failed；created 事件按 nonce 幂等替换，ACK 丢失也不会重复插入。
const pendingNonces = new Map<string, "pending" | "sent" | "failed">();

// Track pending nonces for private messages
const pmPendingNonces = new Map<string, "pending" | "sent" | "failed">();

// Socket listener cleanup handles
let socketCleanups: Array<() => void> = [];

function removeListeners() {
	for (const fn of socketCleanups) fn();
	socketCleanups = [];
}

function addListeners() {
	removeListeners();
	const socket = socketStore.getSocket();
	if (!socket) return;

	socketCleanups = [
		// MESSAGE_CREATED 由 socketEvents 全局注册（离开房间后也要累计未读），
		// 这里只绑定与「当前房间」强相关的事件。
		socket.onServerEvent(EVENTS.MESSAGE_UPDATED, handleUpdated),
		socket.onServerEvent(EVENTS.MESSAGE_DELETED, handleDeleted),
		socket.onServerEvent(EVENTS.MESSAGE_REACTION, handleReaction),
		socket.onServerEvent(EVENTS.MESSAGE_ERROR, handleError),
	];
}

function handleCreated(dto: MessageDTO) {
	const current = getState().textRoom;
	// 不在任何文字房，或消息属于其它频道：只累计未读，绝不塞进当前消息列表
	if (!current || (dto.room_uuid && dto.room_uuid !== current)) {
		incrementUnread(dto);
		return;
	}
	applyCreated(dto);
}

function incrementUnread(dto: MessageDTO) {
	const roomUUID = dto.room_uuid;
	if (!roomUUID) return;
	const me = useUserStore.getState().user;
	if (me && (dto.author_id === me.name || dto.author_id === me.display_name)) {
		return;
	}
	useChatStore.setState((prev) => {
		const unreadByRoom = {
			...prev.unreadByRoom,
			[roomUUID]: (prev.unreadByRoom[roomUUID] ?? 0) + 1,
		};
		persistUnread(unreadByRoom);
		return { unreadByRoom };
	});
}

/** 进入频道 / 手动查看时清零该频道未读。 */
function markRoomRead(roomUUID: string) {
	if (!roomUUID) return;
	useChatStore.setState((prev) => {
		if (!prev.unreadByRoom[roomUUID]) return prev;
		const unreadByRoom = { ...prev.unreadByRoom };
		delete unreadByRoom[roomUUID];
		persistUnread(unreadByRoom);
		return { unreadByRoom };
	});
}

function handleUpdated(dto: MessageDTO) {
	applyUpdated(dto);
}

function handleDeleted(dto: MessageDTO) {
	applyDeleted(dto);
}

function handleReaction(dto: {
	action: "added" | "removed";
	message_uuid: string;
	user_id: string;
	emoji: string;
}) {
	applyReaction(dto);
}

function handleError(data: { client_nonce?: string; error?: string }) {
	if (data.client_nonce) {
		pendingNonces.set(data.client_nonce, "failed");
	}
	if (data.error) {
		toast.error(data.error);
	} else if (data.client_nonce) {
		toast.error("消息发送失败");
	}
}

async function joinTextRoom(room: {
	uuid: string;
	name: string;
	password?: string;
	domain_uuid?: string;
}) {
	useChatStore.setState({
		textRoom: room.uuid,
		textRoomName: room.name,
		textRoomDomainUUID:
			room.domain_uuid ?? socketStore.currentDomainUUID() ?? null,
		textRoomPassword: room.password ?? null,
		textRoomError: "",
		textRoomNeedsPassword: false,
		messages: [],
		hasMore: false,
		nextBefore: null,
		loading: true,
	});
	markRoomRead(room.uuid);
	addListeners();
	try {
		// room:join 走 ack（不能 fire-and-forget）：服务端只在带 id 的请求上回包，
		// 密码错 / 房间满 / 被踢冷却这些拒绝在 fire-and-forget 下会被彻底吞掉。
		// 注意：socket 还没连上时不进入 ack 等待（会白等 8s 握手超时），
		// 直接给出"未连接"的明确失败态。
		const socket = socketStore.getSocket();
		if (!socket?.isConnected()) {
			useChatStore.setState({
				textRoomError: "尚未连接服务器，请稍后重试",
				textRoomNeedsPassword: false,
				loading: false,
			});
			return;
		}
		try {
			await socketStore.joinRoom(
				room.name,
				useUserStore.getState().user?.name ?? "",
				room.password,
				room.domain_uuid ?? socketStore.currentDomainUUID() ?? undefined,
			);
		} catch (error) {
			if (!isRoomPasswordError(error)) {
				// 非密码类失败（房间满 / 被踢）：直接呈现失败态，不再去拉历史
				const message = localizedApiMessage(error, "进入文字频道失败");
				useChatStore.setState({
					textRoomError: message,
					textRoomNeedsPassword: false,
					loading: false,
				});
				toast.error(message);
				return;
			}
			// 密码问题：继续走 loadInitial，由它把"需要密码"的状态和数据一起给出
		}
		await loadInitial();
	} finally {
		useChatStore.setState({ loading: false });
	}
}

function leaveTextRoom() {
	const { textRoomName: roomName, textRoomDomainUUID: roomDomainUUID } =
		getState();
	const socket = socketStore.getSocket();
	if (roomName && socket?.isConnected()) {
		socket.emitFireAndForget(EVENTS.ROOM_LEAVE, {
			room: roomName,
			domain_uuid: roomDomainUUID ?? undefined,
		});
	}
	removeListeners();
	useChatStore.setState({
		textRoom: null,
		textRoomName: null,
		textRoomDomainUUID: null,
		textRoomPassword: null,
		textRoomError: "",
		textRoomNeedsPassword: false,
		messages: [],
		hasMore: false,
		nextBefore: null,
	});
	pendingNonces.clear();
}

/**
 * 私密文字房密码错 / 缺失时，允许用户重新输入密码后重试。
 * 只更新密码并重新拉历史，不改房间本身。
 */
async function retryTextRoomWithPassword(password: string) {
	const { textRoomName, textRoom, textRoomDomainUUID } = getState();
	if (!textRoom || !textRoomName) return;
	useChatStore.setState({ textRoomPassword: password, textRoomError: "" });
	// 重新注册房间（服务端按新密码校验）；socket 没连上时不进入 ack 等待
	if (socketStore.getSocket()?.isConnected()) {
		try {
			await socketStore.joinRoom(
				textRoomName,
				useUserStore.getState().user?.name ?? "",
				password,
				textRoomDomainUUID ?? undefined,
			);
		} catch {
			// 密码仍然错时下面 loadInitial 会再次报错，此处不重复提示
		}
	}
	await loadInitial();
}

async function loadInitial() {
	const room = getState().textRoom;
	if (!room) return;
	useChatStore.setState({ loading: true });
	try {
		const result = await listMessages(
			room,
			undefined,
			100,
			getState().textRoomPassword ?? undefined,
		);
		useChatStore.setState({
			messages: mergeMessages([], result.items),
			hasMore: result.has_more,
			nextBefore: result.has_more ? result.next_before : null,
			textRoomError: "",
			textRoomNeedsPassword: false,
		});
	} catch (error) {
		// 失败必须让用户看见：原先只有 finally，密码错/网络错都退化成"空房间"
		useChatStore.setState({
			messages: [],
			hasMore: false,
			nextBefore: null,
			textRoomError: localizedApiMessage(error, "消息加载失败，请稍后重试"),
			textRoomNeedsPassword: isRoomPasswordError(error),
		});
	} finally {
		useChatStore.setState({ loading: false });
	}
}

async function loadMore() {
	const state = getState();
	const room = state.textRoom;
	if (!room || !state.hasMore || state.loading) return;
	useChatStore.setState({ loading: true });
	try {
		const result = await listMessages(
			room,
			state.nextBefore ?? undefined,
			100,
			state.textRoomPassword ?? undefined,
		);
		useChatStore.setState((prev) => {
			const merged = mergeMessages(prev.messages, result.items);
			// Buffer cap: if length > 1000 and hasMore, drop oldest 200 to stay at ~800
			const capped =
				merged.length > 1000 && result.has_more ? merged.slice(200) : merged;
			return {
				messages: capped,
				hasMore: result.has_more,
				nextBefore: result.has_more ? result.next_before : null,
				textRoomError: "",
			};
		});
	} catch (error) {
		useChatStore.setState({
			textRoomError: localizedApiMessage(error, "加载更早的消息失败"),
			textRoomNeedsPassword: isRoomPasswordError(error),
		});
	} finally {
		useChatStore.setState({ loading: false });
	}
}

function send(
	content: string,
	opts?: { reply_to?: string; mentions?: string[] },
) {
	const state = getState();
	const room = state.textRoom;
	const roomName = state.textRoomName;
	if (!room || !roomName) return;

	const client_nonce = crypto.randomUUID();
	const me = useUserStore.getState().user;

	// Optimistic insert：带上自己的作者信息，否则 pending 期间会渲染成 "?" 头像/名字
	const pending: MessageDTO = {
		uuid: client_nonce,
		room_uuid: room,
		author_id: me?.name ?? "",
		author_name: me?.display_name || me?.name || "",
		author_avatar: me?.avatar || "",
		content,
		reply_to: opts?.reply_to,
		mentions: opts?.mentions,
		deleted: false,
		created_at: new Date().toISOString(),
		client_nonce,
		_status: "pending",
	};

	pendingNonces.set(client_nonce, "pending");
	useChatStore.setState((prev) => ({
		messages: mergeMessages(prev.messages, [pending]),
	}));

	// 发不出去要当场说清楚，不能让占位消息永远停在"发送中"
	const socket = socketStore.getSocket();
	if (!socket?.isConnected()) {
		markMessageFailed(client_nonce, "未连接到服务器");
		return;
	}

	void socket
		.emitAck(EVENTS.MESSAGE_SEND, {
			room: roomName,
			domain_uuid: getState().textRoomDomainUUID ?? undefined,
			content,
			reply_to: opts?.reply_to ?? "",
			mentions: opts?.mentions ?? [],
			client_nonce,
		})
		.then((resp: any) => {
			if (resp?.error) {
				markMessageFailed(client_nonce, serverAckError(resp.error));
				return;
			}
			// ACK 成功不代表落库广播已完成；保持 pending/sent 直到 message:created 按 nonce 幂等替换。
			pendingNonces.set(client_nonce, "sent");
		})
		.catch((err: unknown) => {
			markMessageFailed(
				client_nonce,
				localizedApiMessage(err, "消息发送失败，请检查网络"),
			);
		});
}

/** ack 里的错误是裸字符串（如 "room is full"），统一走一遍中文映射 */
function serverAckError(raw: string): string {
	return localizedApiMessage(
		{ response: { data: { msg: raw } } },
		"消息发送失败",
	);
}

/**
 * 标记发送失败并**保留**该条消息。
 * 旧行为是 15s 后静默删掉，用户以为发出去了——现在改成失败态留存，由用户决定重试或删除。
 */
function markMessageFailed(client_nonce: string, error: string) {
	pendingNonces.set(client_nonce, "failed");
	useChatStore.setState((prev) => ({
		messages: prev.messages.map((m) =>
			m.client_nonce === client_nonce
				? { ...m, _status: "failed", _error: error }
				: m,
		),
	}));
}

/** 重试一条失败的乐观消息：复用原 nonce，服务端按 nonce 幂等，不会重复落库。 */
function retryMessage(uuid: string) {
	const state = getState();
	const msg = state.messages.find((m) => m.uuid === uuid);
	if (msg?._status !== "failed") return;
	const socket = socketStore.getSocket();
	const nonce = msg.client_nonce ?? uuid;
	if (!socket?.isConnected()) {
		markMessageFailed(nonce, "未连接到服务器");
		return;
	}
	pendingNonces.set(nonce, "pending");
	useChatStore.setState((prev) => ({
		messages: prev.messages.map((m) =>
			m.uuid === uuid ? { ...m, _status: "pending", _error: undefined } : m,
		),
	}));
	void socket
		.emitAck(EVENTS.MESSAGE_SEND, {
			room: state.textRoomName,
			domain_uuid: state.textRoomDomainUUID ?? undefined,
			content: msg.content,
			reply_to: msg.reply_to ?? "",
			mentions: msg.mentions ?? [],
			client_nonce: nonce,
		})
		.then((resp: any) => {
			if (resp?.error) {
				markMessageFailed(nonce, serverAckError(resp.error));
				return;
			}
			pendingNonces.set(nonce, "sent");
		})
		.catch((err: unknown) =>
			markMessageFailed(
				nonce,
				localizedApiMessage(err, "消息发送失败，请检查网络"),
			),
		);
}

function applyCreated(dto: MessageDTO) {
	useChatStore.setState((prev) => {
		// 切频道过程中的迟到广播不能串进新频道的消息列表
		if (dto.room_uuid && prev.textRoom && dto.room_uuid !== prev.textRoom) {
			return prev;
		}
		// If this message matches a pending nonce, replace the pending entry
		if (dto.client_nonce && pendingNonces.has(dto.client_nonce)) {
			pendingNonces.delete(dto.client_nonce);
			return {
				messages: prev.messages
					.filter((m) => m.client_nonce !== dto.client_nonce)
					.concat(dto)
					.sort((a, b) => a.created_at.localeCompare(b.created_at)),
			};
		}
		// Otherwise it's a brand-new message from someone else
		return { messages: mergeMessages(prev.messages, [dto]) };
	});
}

function applyUpdated(dto: MessageDTO) {
	useChatStore.setState((prev) => ({
		messages: prev.messages.map((m) => (m.uuid === dto.uuid ? dto : m)),
	}));
}

function applyDeleted(dto: MessageDTO) {
	useChatStore.setState((prev) => ({
		messages: prev.messages.map((m) =>
			m.uuid === dto.uuid ? { ...m, deleted: true } : m,
		),
	}));
}

function applyReaction(dto: {
	action: "added" | "removed";
	message_uuid: string;
	user_id: string;
	emoji: string;
}) {
	useChatStore.setState((prev) => {
		const next = { ...prev.reactions };
		if (!next[dto.message_uuid]) {
			next[dto.message_uuid] = {};
		}
		const msgReactions = { ...next[dto.message_uuid] };
		const users = [...(msgReactions[dto.emoji] || [])];
		if (dto.action === "added") {
			if (!users.includes(dto.user_id)) {
				users.push(dto.user_id);
			}
		} else {
			const idx = users.indexOf(dto.user_id);
			if (idx !== -1) users.splice(idx, 1);
		}
		if (users.length > 0) {
			msgReactions[dto.emoji] = users;
		} else {
			delete msgReactions[dto.emoji];
		}
		next[dto.message_uuid] = msgReactions;
		return { reactions: next };
	});
}

function deleteMessage(messageUUID: string) {
	const roomName = getState().textRoomName;
	if (!roomName) return;
	// Optimistic: mark as deleted locally
	applyDeleted({ uuid: messageUUID } as MessageDTO);
	const socket = socketStore.getSocket();
	if (socket?.isConnected()) {
		socket.emitFireAndForget(EVENTS.MESSAGE_DELETE, {
			room: roomName,
			domain_uuid: getState().textRoomDomainUUID ?? undefined,
			message_uuid: messageUUID,
		});
	}
}

// ------------------------------------------------------------------
// Private chat (私聊)
// ------------------------------------------------------------------

function tempConversationID(identity: string) {
	return `pm_${identity}`;
}

function replaceConversationID(from: string, to: string) {
	if (!from || !to || from === to) return;
	useChatStore.setState((prev) => ({
		conversations: remapConversationList(prev.conversations, from, to),
		pmMessages: remapPrivateMessages(prev.pmMessages, from, to),
		pmHasMore: remapRecordKey(prev.pmHasMore, from, to),
		pmNextCursor: remapRecordKey(prev.pmNextCursor, from, to),
		loadingMessages: remapRecordKey(prev.loadingMessages, from, to),
		activeConversationID:
			prev.activeConversationID === from ? to : prev.activeConversationID,
	}));
	void chatCache.renameConversation(from, to).catch(() => undefined);
}

function resolvePrivateConversationID(dto: PrivateMessageDTO) {
	const realID = dto.conversation_id;
	if (!realID) return null;
	const me = useUserStore.getState().user?.name ?? "";
	const other = me === dto.author_id ? dto.target_identity : dto.author_id;
	if (other) {
		const tempID = tempConversationID(other);
		if (
			tempID !== realID &&
			getState().conversations.some((c) => c.conversation_id === tempID)
		) {
			replaceConversationID(tempID, realID);
		}
	}
	return realID;
}

function upsertPrivateConversation(dto: PrivateMessageDTO) {
	const convID = dto.conversation_id;
	if (!convID) return;
	useChatStore.setState((prev) => {
		const me = useUserStore.getState().user?.name ?? "";
		const other = me === dto.author_id ? dto.target_identity : dto.author_id;
		const existing = prev.conversations.find(
			(c) => c.conversation_id === convID,
		);
		const base = existing || {
			conversation_id: convID,
			other_identity: other || "",
			other_display_name: other || "",
			other_avatar: "",
			last_content: "",
			last_sender_identity: "",
			last_message_at: 0,
			unread_count: 0,
		};
		const next = {
			...base,
			other_identity: other || base.other_identity,
			other_display_name: other || base.other_display_name,
			last_content: dto.content,
			last_sender_identity: dto.author_id,
			last_message_at: new Date(dto.created_at).getTime(),
			unread_count:
				prev.activeConversationID === convID || me === dto.author_id
					? 0
					: (existing?.unread_count || 0) + 1,
		};
		return {
			conversations: existing
				? prev.conversations.map((c) =>
						c.conversation_id === convID ? next : c,
					)
				: [next, ...prev.conversations],
		};
	});
}

function appendPrivateMessage(convID: string, dto: PrivateMessageDTO) {
	useChatStore.setState((prev) => {
		const existing = prev.pmMessages[convID] || [];
		if (dto.client_nonce && pmPendingNonces.has(dto.client_nonce)) {
			pmPendingNonces.delete(dto.client_nonce);
			const filtered = existing.filter(
				(m) => m.client_nonce !== dto.client_nonce,
			);
			return {
				pmMessages: {
					...prev.pmMessages,
					[convID]: mergePrivateMessages(filtered, [dto]),
				},
			};
		}
		if (existing.some((m) => m.uuid === dto.uuid)) return prev;
		return {
			pmMessages: {
				...prev.pmMessages,
				[convID]: mergePrivateMessages(existing, [dto]),
			},
		};
	});
}

function acceptPrivateMessage(dto: PrivateMessageDTO) {
	const convID = resolvePrivateConversationID(dto);
	if (!convID) {
		void loadConversations();
		return;
	}
	appendPrivateMessage(convID, dto);
	upsertPrivateConversation(dto);
	void loadConversations();
	if (getState().activeConversationID === convID) {
		void markRead(convID);
	}
}

function handlePrivateNew(dto: PrivateMessageDTO) {
	acceptPrivateMessage(dto);
}

async function loadConversations() {
	useChatStore.setState({ loadingList: true });
	try {
		const cached = await chatCache.getConversations();
		if (cached.length > 0) {
			useChatStore.setState({
				conversations: cached.map((c) => ({
					conversation_id: c.conversation_id,
					other_identity: c.other_identity,
					other_display_name: c.other_display_name,
					other_avatar: c.other_avatar,
					last_content: c.last_content,
					last_sender_identity: c.last_sender_identity,
					last_message_at: c.last_message_at,
					unread_count: c.unread_count,
				})),
			});
		}
		const list = await listConversations();
		useChatStore.setState({ conversations: list });
		void chatCache.setConversations(
			list.map((c) => ({ ...c, cached_at: Date.now() })),
		);
	} finally {
		useChatStore.setState({ loadingList: false });
	}
}

async function selectConversation(convID: string) {
	useChatStore.setState((prev) => ({
		activeConversationID: convID,
		loadingMessages: { ...prev.loadingMessages, [convID]: true },
	}));
	try {
		const cached = await chatCache.getMessages(convID);
		if (cached.length > 0) {
			useChatStore.setState((prev) => ({
				pmMessages: {
					...prev.pmMessages,
					[convID]: cached.map((c) => ({
						uuid: c.id,
						author_id: c.sender_identity,
						content: c.content,
						created_at: new Date(c.timestamp).toISOString(),
						deleted: false,
						reply_to: c.reply_to || undefined,
						conversation_id: convID,
					})),
				},
			}));
		}
		const result = await getConversationMessages(convID);
		useChatStore.setState((prev) => ({
			pmMessages: { ...prev.pmMessages, [convID]: result.messages },
			pmHasMore: { ...prev.pmHasMore, [convID]: !!result.next_cursor },
			pmNextCursor: {
				...prev.pmNextCursor,
				[convID]: result.next_cursor || null,
			},
		}));
		void chatCache.appendMessages(
			convID,
			result.messages.map((m) => ({
				id: m.uuid,
				conversation_id: convID,
				content: m.content,
				sender_identity: m.author_id,
				sender_display: "",
				sender_role: "",
				timestamp: new Date(m.created_at).getTime(),
				reply_to: m.reply_to || "",
				target_identity: m.target_identity || "",
			})),
		);
		void markRead(convID);
	} finally {
		useChatStore.setState((prev) => ({
			loadingMessages: { ...prev.loadingMessages, [convID]: false },
		}));
	}
}

async function loadMoreMessages(convID: string) {
	const state = getState();
	if (!state.pmHasMore[convID] || state.loadingMessages[convID]) return;
	useChatStore.setState((prev) => ({
		loadingMessages: { ...prev.loadingMessages, [convID]: true },
	}));
	try {
		const cursor = getState().pmNextCursor[convID];
		const result = await getConversationMessages(convID, cursor || undefined);
		useChatStore.setState((prev) => {
			const existing = prev.pmMessages[convID] || [];
			const map = new Map<string, PrivateMessageDTO>();
			for (const m of result.messages) map.set(m.uuid, m);
			for (const m of existing) {
				if (!map.has(m.uuid)) map.set(m.uuid, m);
			}
			return {
				pmMessages: { ...prev.pmMessages, [convID]: Array.from(map.values()) },
				pmHasMore: { ...prev.pmHasMore, [convID]: !!result.next_cursor },
				pmNextCursor: {
					...prev.pmNextCursor,
					[convID]: result.next_cursor || null,
				},
			};
		});
	} finally {
		useChatStore.setState((prev) => ({
			loadingMessages: { ...prev.loadingMessages, [convID]: false },
		}));
	}
}

function sendDirect(convID: string, content: string) {
	if (!content.trim()) return;
	const conv = getState().conversations.find(
		(c) => c.conversation_id === convID,
	);
	if (!conv) return;
	const targetIdentity = conv.other_identity;
	const clientNonce = crypto.randomUUID();

	const pending: PrivateMessageDTO = {
		uuid: clientNonce,
		author_id: useUserStore.getState().user?.name ?? "",
		content,
		deleted: false,
		created_at: new Date().toISOString(),
		conversation_id: convID,
		target_identity: targetIdentity,
		client_nonce: clientNonce,
	};
	pmPendingNonces.set(clientNonce, "pending");
	useChatStore.setState((prev) => {
		const existing = prev.pmMessages[convID] || [];
		return {
			pmMessages: { ...prev.pmMessages, [convID]: [...existing, pending] },
		};
	});

	const socket = socketStore.getSocket();
	if (socket?.isConnected()) {
		const markFailed = () => {
			pmPendingNonces.set(clientNonce, "failed");
			setTimeout(() => {
				if (pmPendingNonces.get(clientNonce) === "failed") {
					useChatStore.setState((prev) => {
						const existing = prev.pmMessages[convID] || [];
						return {
							pmMessages: {
								...prev.pmMessages,
								[convID]: existing.filter(
									(m) => m.client_nonce !== clientNonce,
								),
							},
						};
					});
					pmPendingNonces.delete(clientNonce);
				}
			}, 15000);
		};
		void socket
			.emitAck(EVENTS.PRIVATE_SEND, {
				target_identity: targetIdentity,
				content,
				client_nonce: clientNonce,
			})
			.then((resp: any) => {
				if (resp?.error) {
					markFailed();
					return;
				}
				const dto = resp as PrivateMessageDTO;
				if (!dto?.conversation_id) {
					markFailed();
					return;
				}
				pmPendingNonces.set(clientNonce, "sent");
				acceptPrivateMessage(dto);
			})
			.catch(markFailed);
	}
}

async function startConversation(identity: string) {
	const existing = getState().conversations.find(
		(c) => c.other_identity === identity,
	);
	if (existing) {
		await selectConversation(existing.conversation_id);
		return existing.conversation_id;
	}
	// Temporary placeholder conversation
	const convID = `pm_${identity}`;
	useChatStore.setState((prev) => {
		if (prev.conversations.some((c) => c.other_identity === identity)) {
			return prev;
		}
		return {
			conversations: [
				...prev.conversations,
				{
					conversation_id: convID,
					other_identity: identity,
					other_display_name: identity,
					other_avatar: "",
					last_content: "",
					last_sender_identity: "",
					last_message_at: 0,
					unread_count: 0,
				},
			],
		};
	});
	useChatStore.setState({ activeConversationID: convID });
	return convID;
}

async function markRead(convID: string) {
	try {
		await markConversationRead(convID);
		useChatStore.setState((prev) => ({
			conversations: prev.conversations.map((c) =>
				c.conversation_id === convID ? { ...c, unread_count: 0 } : c,
			),
		}));
	} catch {
		// Best-effort
	}
}

setPrivateNewHandler(handlePrivateNew);
setMessageCreatedHandler(handleCreated);

// 原 solid effect：text room 归属域与当前域不一致时自动离开。
// socketStore 与 chatStore 存在模块循环引用，加载期可能尚未初始化完成；
// currentDomainUUID 缺失时跳过，避免模块加载崩溃。
useDomainStore.subscribe(() => {
	const state = getState();
	const domain = useDomainStore.getState().currentDomainUUID ?? null;
	const roomDomain = state.textRoomDomainUUID;
	if (roomDomain && roomDomain !== (domain ?? undefined)) {
		leaveTextRoom();
	}
});

export const chatStore = {
	textRoom: () => getState().textRoom,
	textRoomName: () => getState().textRoomName,
	textRoomDomainUUID: () => getState().textRoomDomainUUID,
	messages: () => getState().messages,
	hasMore: () => getState().hasMore,
	nextBefore: () => getState().nextBefore,
	isAtBottom: () => getState().isAtBottom,
	setIsAtBottom: (v: boolean) => useChatStore.setState({ isAtBottom: v }),
	loading: () => getState().loading,
	reactions: () => getState().reactions,
	joinTextRoom,
	leaveTextRoom,
	markRoomRead,
	unreadByRoom: () => getState().unreadByRoom,
	retryTextRoomWithPassword,
	loadInitial,
	loadMore,
	send,
	retryMessage,
	applyCreated,
	applyUpdated,
	applyDeleted,
	applyReaction,
	deleteMessage,
	// Private chat
	conversations: () => getState().conversations,
	activeConversationID: () => getState().activeConversationID,
	pmMessages: () => getState().pmMessages,
	pmHasMore: () => getState().pmHasMore,
	loadingList: () => getState().loadingList,
	loadingMessages: () => getState().loadingMessages,
	loadConversations,
	selectConversation,
	loadMoreMessages,
	sendDirect,
	startConversation,
	markRead,
	handlePrivateNew,
};

export default chatStore;
