import { createEffect, createRoot, createSignal } from "solid-js";
import { showToast } from "solid-notifications";
import {
	type ConversationDTO,
	getConversationMessages,
	listConversations,
	markConversationRead,
	type PrivateMessageDTO,
} from "@/api/conversation";
import { listMessages } from "@/api/message";
import { EVENTS } from "@/socket/events";
import { socketStore } from "@/stores/socketStore";
import userStore from "@/stores/userStore";
import { setPrivateNewHandler } from "@/stores/socketStore";
import type { MessageDTO } from "@/types/message";
import { chatCache } from "@/utils/idb-cache";

// ---------------------------------------------------------------------------

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

// Chat store
// ---------------------------------------------------------------------------

export const chatStore = createRoot(() => {
	const [textRoom, setTextRoom] = createSignal<string | null>(null);
	const [textRoomName, setTextRoomName] = createSignal<string | null>(null);
	const [textRoomDomainUUID, setTextRoomDomainUUID] = createSignal<
		string | null
	>(null);
	const [messages, setMessages] = createSignal<MessageDTO[]>([]);
	const [hasMore, setHasMore] = createSignal(false);
	const [nextBefore, setNextBefore] = createSignal<string | null>(null);
	const [isAtBottom, setIsAtBottom] = createSignal(true);
	const [loading, setLoading] = createSignal(false);

	createEffect(() => {
		// socketStore 与 chatStore 存在模块循环引用，加载期可能尚未初始化完成；
		// currentDomainUUID 缺失时跳过，避免模块加载崩溃。
		const store = socketStore as
			| { currentDomainUUID?: () => string | null }
			| undefined;
		const domain = store?.currentDomainUUID?.() ?? null;
		const roomDomain = textRoomDomainUUID();
		if (roomDomain && roomDomain !== (domain ?? undefined)) {
			leaveTextRoom();
		}
	});

	// Nonces waiting for server ack / created event.
	// 状态机：pending -> sent/failed；created 事件按 nonce 幂等替换，ACK 丢失也不会重复插入。
	const pendingNonces = new Map<string, "pending" | "sent" | "failed">();

	// Reactions: message_uuid -> { emoji -> user_id[] }
	const [reactions, setReactions] = createSignal<
		Record<string, Record<string, string[]>>
	>({});

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
			socket.onServerEvent(EVENTS.MESSAGE_CREATED, handleCreated),
			socket.onServerEvent(EVENTS.MESSAGE_UPDATED, handleUpdated),
			socket.onServerEvent(EVENTS.MESSAGE_DELETED, handleDeleted),
			socket.onServerEvent(EVENTS.MESSAGE_REACTION, handleReaction),
			socket.onServerEvent(EVENTS.MESSAGE_ERROR, handleError),
		];
	}

	// ------------------------------------------------------------------
	// Event handlers
	// ------------------------------------------------------------------

	function handleCreated(dto: MessageDTO) {
		applyCreated(dto);
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
			showToast(data.error, { type: "error" });
		} else if (data.client_nonce) {
			showToast("消息发送失败", { type: "error" });
		}
	}

	// ------------------------------------------------------------------
	// Public API
	// ------------------------------------------------------------------

	async function joinTextRoom(room: {
		uuid: string;
		name: string;
		password?: string;
		domain_uuid?: string;
	}) {
		setTextRoom(room.uuid);
		setTextRoomName(room.name);
		setTextRoomDomainUUID(
			room.domain_uuid ?? socketStore.currentDomainUUID() ?? null,
		);
		setMessages([]);
		setHasMore(false);
		setNextBefore(null);
		setLoading(true);
		addListeners();
		try {
			// Emit room:join so the Hub registers this socket in connSlots
			const socket = socketStore.getSocket();
			if (socket?.isConnected()) {
				socket.emitFireAndForget(EVENTS.ROOM_JOIN, {
					room: room.name,
					domain_uuid:
						room.domain_uuid ?? socketStore.currentDomainUUID() ?? undefined,
					...(room.password ? { password: room.password } : {}),
				});
			}
			await loadInitial();
		} finally {
			setLoading(false);
		}
	}

	function leaveTextRoom() {
		const roomName = textRoomName();
		const roomDomainUUID = textRoomDomainUUID();
		const socket = socketStore.getSocket();
		if (roomName && socket?.isConnected()) {
			socket.emitFireAndForget(EVENTS.ROOM_LEAVE, {
				room: roomName,
				domain_uuid: roomDomainUUID ?? undefined,
			});
		}
		removeListeners();
		setTextRoom(null);
		setTextRoomName(null);
		setTextRoomDomainUUID(null);
		setMessages([]);
		setHasMore(false);
		setNextBefore(null);
		pendingNonces.clear();
	}

	async function loadInitial() {
		const room = textRoom();
		if (!room) return;
		setLoading(true);
		try {
			const result = await listMessages(room);
			setMessages(mergeMessages([], result.items));
			setHasMore(result.has_more);
			setNextBefore(result.has_more ? result.next_before : null);
		} finally {
			setLoading(false);
		}
	}

	async function loadMore() {
		const room = textRoom();
		if (!room || !hasMore() || loading()) return;
		setLoading(true);
		try {
			const result = await listMessages(room, nextBefore() ?? undefined);
			setMessages((prev) => {
				const merged = mergeMessages(prev, result.items);
				// Buffer cap: if length > 1000 and hasMore, drop oldest 200 to stay at ~800
				if (merged.length > 1000 && result.has_more) {
					return merged.slice(200);
				}
				return merged;
			});
			setHasMore(result.has_more);
			setNextBefore(result.has_more ? result.next_before : null);
		} finally {
			setLoading(false);
		}
	}

	function send(
		content: string,
		opts?: { reply_to?: string; mentions?: string[] },
	) {
		const room = textRoom();
		const roomName = textRoomName();
		if (!room || !roomName) return;

		const client_nonce = crypto.randomUUID();

		// Optimistic insert
		const pending: MessageDTO = {
			uuid: client_nonce,
			room_uuid: room,
			author_id: "",
			content,
			reply_to: opts?.reply_to,
			mentions: opts?.mentions,
			deleted: false,
			created_at: new Date().toISOString(),
			client_nonce,
		};

		pendingNonces.set(client_nonce, "pending");
		setMessages((prev) => mergeMessages(prev, [pending]));

		// Emit via socket (server expects a JSON string payload)
		const socket = socketStore.getSocket();
		if (socket?.isConnected()) {
			const markFailed = () => {
				pendingNonces.set(client_nonce, "failed");
				// 保留乐观消息等待可能的 message:created；超时后再清理，避免 ACK 丢失重复插入。
				setTimeout(() => {
					if (pendingNonces.get(client_nonce) === "failed") {
						setMessages((prev) =>
							prev.filter((m) => m.client_nonce !== client_nonce),
						);
						pendingNonces.delete(client_nonce);
					}
				}, 15000);
			};
			void socket
				.emitAck(EVENTS.MESSAGE_SEND, {
					room: roomName,
					domain_uuid: textRoomDomainUUID() ?? undefined,
					content,
					reply_to: opts?.reply_to ?? "",
					mentions: opts?.mentions ?? [],
					client_nonce,
				})
				.then((resp: any) => {
					if (resp?.error) {
						markFailed();
						return;
					}
					// ACK 成功不代表落库广播已完成；保持 pending/sent 直到 message:created 按 nonce 幂等替换。
					pendingNonces.set(client_nonce, "sent");
				})
				.catch(markFailed);
		}
	}

	function applyCreated(dto: MessageDTO) {
		setMessages((prev) => {
			// If this message matches a pending nonce, replace the pending entry
			if (dto.client_nonce && pendingNonces.has(dto.client_nonce)) {
				pendingNonces.delete(dto.client_nonce);
				return prev
					.filter((m) => m.client_nonce !== dto.client_nonce)
					.concat(dto)
					.sort((a, b) => a.created_at.localeCompare(b.created_at));
			}
			// Otherwise it's a brand-new message from someone else
			return mergeMessages(prev, [dto]);
		});
	}

	function applyUpdated(dto: MessageDTO) {
		setMessages((prev) => prev.map((m) => (m.uuid === dto.uuid ? dto : m)));
	}

	function applyDeleted(dto: MessageDTO) {
		setMessages((prev) =>
			prev.map((m) => (m.uuid === dto.uuid ? { ...m, deleted: true } : m)),
		);
	}

	function applyReaction(dto: {
		action: "added" | "removed";
		message_uuid: string;
		user_id: string;
		emoji: string;
	}) {
		setReactions((prev) => {
			const next = { ...prev };
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
			return next;
		});
	}

	function deleteMessage(messageUUID: string) {
		const roomName = textRoomName();
		if (!roomName) return;
		// Optimistic: mark as deleted locally
		applyDeleted({ uuid: messageUUID } as MessageDTO);
		const socket = socketStore.getSocket();
		if (socket?.isConnected()) {
			socket.emitFireAndForget(EVENTS.MESSAGE_DELETE, {
				room: roomName,
				domain_uuid: textRoomDomainUUID() ?? undefined,
				message_uuid: messageUUID,
			});
		}
	}

	// ------------------------------------------------------------------
	// Private chat (私聊) state + methods
	// ------------------------------------------------------------------

	const [conversations, setConversations] = createSignal<ConversationDTO[]>([]);
	const [activeConversationID, setActiveConversationID] = createSignal<
		string | null
	>(null);
	const [pmMessages, setPmMessages] = createSignal<
		Record<string, PrivateMessageDTO[]>
	>({});
	const [pmHasMore, setPmHasMore] = createSignal<Record<string, boolean>>({});
	const [pmNextCursor, setPmNextCursor] = createSignal<
		Record<string, string | null>
	>({});
	const [loadingList, setLoadingList] = createSignal(false);
	const [loadingMessages, setLoadingMessages] = createSignal<
		Record<string, boolean>
	>({});

	// Track pending nonces for private messages
	const pmPendingNonces = new Map<string, "pending" | "sent" | "failed">();

	function tempConversationID(identity: string) {
		return `pm_${identity}`;
	}

	function replaceConversationID(from: string, to: string) {
		if (!from || !to || from === to) return;
		setConversations((prev) => remapConversationList(prev, from, to));
		setPmMessages((prev) => remapPrivateMessages(prev, from, to));
		setPmHasMore((prev) => remapRecordKey(prev, from, to));
		setPmNextCursor((prev) => remapRecordKey(prev, from, to));
		setLoadingMessages((prev) => remapRecordKey(prev, from, to));
		setActiveConversationID((prev) => (prev === from ? to : prev));
		void chatCache.renameConversation(from, to).catch(() => undefined);
	}

	function resolvePrivateConversationID(dto: PrivateMessageDTO) {
		const realID = dto.conversation_id;
		if (!realID) return null;
		const me = userStore.user()?.name ?? "";
		const other = me === dto.author_id ? dto.target_identity : dto.author_id;
		if (other) {
			const tempID = tempConversationID(other);
			if (
				tempID !== realID &&
				conversations().some((c) => c.conversation_id === tempID)
			) {
				replaceConversationID(tempID, realID);
			}
		}
		return realID;
	}
	function upsertPrivateConversation(dto: PrivateMessageDTO) {
		const convID = dto.conversation_id;
		if (!convID) return;
		setConversations((prev) => {
			const me = userStore.user()?.name ?? "";
			const other = me === dto.author_id ? dto.target_identity : dto.author_id;
			const existing = prev.find((c) => c.conversation_id === convID);
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
					activeConversationID() === convID || me === dto.author_id
						? 0
						: (existing?.unread_count || 0) + 1,
			};
			return existing
				? prev.map((c) => (c.conversation_id === convID ? next : c))
				: [next, ...prev];
		});
	}

	function appendPrivateMessage(convID: string, dto: PrivateMessageDTO) {
		setPmMessages((prev) => {
			const existing = prev[convID] || [];
			if (dto.client_nonce && pmPendingNonces.has(dto.client_nonce)) {
				pmPendingNonces.delete(dto.client_nonce);
				const filtered = existing.filter(
					(m) => m.client_nonce !== dto.client_nonce,
				);
				return {
					...prev,
					[convID]: mergePrivateMessages(filtered, [dto]),
				};
			}
			if (existing.some((m) => m.uuid === dto.uuid)) return prev;
			return {
				...prev,
				[convID]: mergePrivateMessages(existing, [dto]),
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
		if (activeConversationID() === convID) {
			void markRead(convID);
		}
	}

	function handlePrivateNew(dto: PrivateMessageDTO) {
		acceptPrivateMessage(dto);
	}

	async function loadConversations() {
		setLoadingList(true);
		try {
			const cached = await chatCache.getConversations();
			if (cached.length > 0) {
				setConversations(
					cached.map((c) => ({
						conversation_id: c.conversation_id,
						other_identity: c.other_identity,
						other_display_name: c.other_display_name,
						other_avatar: c.other_avatar,
						last_content: c.last_content,
						last_sender_identity: c.last_sender_identity,
						last_message_at: c.last_message_at,
						unread_count: c.unread_count,
					})),
				);
			}
			const list = await listConversations();
			setConversations(list);
			void chatCache.setConversations(
				list.map((c) => ({ ...c, cached_at: Date.now() })),
			);
		} finally {
			setLoadingList(false);
		}
	}

	async function selectConversation(convID: string) {
		setActiveConversationID(convID);
		setLoadingMessages((prev) => ({ ...prev, [convID]: true }));
		try {
			const cached = await chatCache.getMessages(convID);
			if (cached.length > 0) {
				setPmMessages((prev) => ({
					...prev,
					[convID]: cached.map((c) => ({
						uuid: c.id,
						author_id: c.sender_identity,
						content: c.content,
						created_at: new Date(c.timestamp).toISOString(),
						deleted: false,
						reply_to: c.reply_to || undefined,
						conversation_id: convID,
					})),
				}));
			}
			const result = await getConversationMessages(convID);
			setPmMessages((prev) => ({ ...prev, [convID]: result.messages }));
			setPmHasMore((prev) => ({
				...prev,
				[convID]: !!result.next_cursor,
			}));
			setPmNextCursor((prev) => ({
				...prev,
				[convID]: result.next_cursor || null,
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
			setLoadingMessages((prev) => ({ ...prev, [convID]: false }));
		}
	}

	async function loadMoreMessages(convID: string) {
		if (!pmHasMore()[convID] || loadingMessages()[convID]) return;
		setLoadingMessages((prev) => ({ ...prev, [convID]: true }));
		try {
			const cursor = pmNextCursor()[convID];
			const result = await getConversationMessages(convID, cursor || undefined);
			setPmMessages((prev) => {
				const existing = prev[convID] || [];
				const map = new Map<string, PrivateMessageDTO>();
				for (const m of result.messages) map.set(m.uuid, m);
				for (const m of existing) {
					if (!map.has(m.uuid)) map.set(m.uuid, m);
				}
				return { ...prev, [convID]: Array.from(map.values()) };
			});
			setPmHasMore((prev) => ({
				...prev,
				[convID]: !!result.next_cursor,
			}));
			setPmNextCursor((prev) => ({
				...prev,
				[convID]: result.next_cursor || null,
			}));
		} finally {
			setLoadingMessages((prev) => ({ ...prev, [convID]: false }));
		}
	}

	function sendDirect(convID: string, content: string) {
		if (!content.trim()) return;
		const conv = conversations().find((c) => c.conversation_id === convID);
		if (!conv) return;
		const targetIdentity = conv.other_identity;
		const clientNonce = crypto.randomUUID();

		const pending: PrivateMessageDTO = {
			uuid: clientNonce,
			author_id: userStore.user()?.name ?? "",
			content,
			deleted: false,
			created_at: new Date().toISOString(),
			conversation_id: convID,
			target_identity: targetIdentity,
			client_nonce: clientNonce,
		};
		pmPendingNonces.set(clientNonce, "pending");
		setPmMessages((prev) => {
			const existing = prev[convID] || [];
			return { ...prev, [convID]: [...existing, pending] };
		});

		const socket = socketStore.getSocket();
		if (socket?.isConnected()) {
			const markFailed = () => {
				pmPendingNonces.set(clientNonce, "failed");
				setTimeout(() => {
					if (pmPendingNonces.get(clientNonce) === "failed") {
						setPmMessages((prev) => {
							const existing = prev[convID] || [];
							return {
								...prev,
								[convID]: existing.filter(
									(m) => m.client_nonce !== clientNonce,
								),
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
		const existing = conversations().find((c) => c.other_identity === identity);
		if (existing) {
			await selectConversation(existing.conversation_id);
			return existing.conversation_id;
		}
		// Temporary placeholder conversation
		const convID = `pm_${identity}`;
		setConversations((prev) => {
			if (prev.some((c) => c.other_identity === identity)) return prev;
			return [
				...prev,
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
			];
		});
		setActiveConversationID(convID);
		return convID;
	}

	async function markRead(convID: string) {
		try {
			await markConversationRead(convID);
			setConversations((prev) =>
				prev.map((c) =>
					c.conversation_id === convID ? { ...c, unread_count: 0 } : c,
				),
			);
		} catch {
			// Best-effort
		}
	}

	setPrivateNewHandler(handlePrivateNew);

	return {
		textRoom,
		textRoomName,
		messages,
		hasMore,
		isAtBottom,
		setIsAtBottom,
		loading,
		reactions,
		joinTextRoom,
		leaveTextRoom,
		loadInitial,
		loadMore,
		send,
		applyCreated,
		applyUpdated,
		applyDeleted,
		applyReaction,
		deleteMessage,
		// Private chat
		conversations,
		activeConversationID,
		pmMessages,
		pmHasMore,
		loadingList,
		loadingMessages,
		loadConversations,
		selectConversation,
		loadMoreMessages,
		sendDirect,
		startConversation,
		markRead,
		handlePrivateNew,
	};
});
