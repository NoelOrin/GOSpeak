import { batch, createRoot, createSignal } from "solid-js";
import type { ConversationDTO, MessageDTO } from "@/api/conversation";
import {
	getConversationMessages,
	listConversations,
	markConversationRead,
} from "@/api/conversation";
import { EVENTS } from "@/socket/events";
import userStore from "@/stores/userStore";
import {
	type CachedConversation,
	type CachedMessage,
	chatCache,
} from "@/utils/idb-cache";

const MAX_MESSAGES = 200;

function convToCache(c: ConversationDTO): CachedConversation {
	return {
		conversation_id: c.conversation_id,
		other_identity: c.other_identity,
		other_display_name: c.other_display_name,
		other_avatar: c.other_avatar,
		last_content: c.last_content,
		last_sender_identity: c.last_sender_identity,
		last_message_at: c.last_message_at,
		unread_count: c.unread_count,
		cached_at: Date.now(),
	};
}

function cacheToDTO(c: CachedConversation): ConversationDTO {
	return {
		conversation_id: c.conversation_id,
		other_identity: c.other_identity,
		other_display_name: c.other_display_name,
		other_avatar: c.other_avatar,
		last_content: c.last_content,
		last_sender_identity: c.last_sender_identity,
		last_message_at: c.last_message_at,
		unread_count: c.unread_count,
	};
}

function cachedMsgToDTO(m: CachedMessage): MessageDTO {
	return {
		id: m.id,
		room: "",
		content: m.content,
		replyTo: m.reply_to,
		senderIdentity: m.sender_identity,
		senderDisplay: m.sender_display,
		senderRole: m.sender_role,
		timestamp: m.timestamp,
		conversationId: m.conversation_id,
		targetIdentity: m.target_identity,
	};
}

function msgToCached(convID: string, m: MessageDTO): CachedMessage {
	return {
		id: m.id,
		conversation_id: convID,
		content: m.content,
		sender_identity: m.senderIdentity,
		sender_display: m.senderDisplay,
		sender_role: m.senderRole,
		timestamp: m.timestamp,
		reply_to: m.replyTo,
		target_identity: m.targetIdentity,
	};
}

export const chatStore = createRoot(() => {
	const [conversations, setConversations] = createSignal<ConversationDTO[]>([]);
	const [activeConversationID, setActiveConversationID] = createSignal<
		string | null
	>(null);
	const [messages, setMessages] = createSignal<Record<string, MessageDTO[]>>(
		{},
	);
	const [hasMore, setHasMore] = createSignal<Record<string, boolean>>({});
	const [loadingList, setLoadingList] = createSignal(false);
	const [loadingMessages, setLoadingMessages] = createSignal<
		Record<string, boolean>
	>({});

	async function loadConversations(): Promise<void> {
		try {
			const cached = await chatCache.getConversations();
			if (cached.length > 0) {
				setConversations(cached.map(cacheToDTO));
			}
		} catch {
			// IDB unavailable or empty
		}

		setLoadingList(true);
		try {
			const list = await listConversations();
			setConversations(list);
			chatCache.setConversations(list.map(convToCache)).catch(() => {});
		} catch (err) {
			console.error("[Chat] loadConversations failed:", err);
		} finally {
			setLoadingList(false);
		}
	}

	async function selectConversation(id: string): Promise<void> {
		setActiveConversationID(id);

		// Show cached messages first
		try {
			const cached = await chatCache.getMessages(id);
			if (cached.length > 0) {
				setMessages((prev) => ({
					...prev,
					[id]: cached.map(cachedMsgToDTO),
				}));
			}
		} catch {
			// no-op
		}

		// Reset unread
		const conv = conversations().find((c) => c.conversation_id === id);
		if (conv && conv.unread_count > 0) {
			markConversationRead(id).catch(() => {});
			setConversations((prev) =>
				prev.map((c) =>
					c.conversation_id === id ? { ...c, unread_count: 0 } : c,
				),
			);
		}

		setLoadingMessages((prev) => ({ ...prev, [id]: true }));
		try {
			const result = await getConversationMessages(id, undefined, MAX_MESSAGES);
			setMessages((prev) => {
				const existing = prev[id] || [];
				const apiIds = new Set(result.messages.map((m) => m.id));
				const wsOnly = existing.filter((m) => !apiIds.has(m.id));
				return { ...prev, [id]: [...result.messages, ...wsOnly] };
			});
			setHasMore((prev) => ({ ...prev, [id]: !!result.nextCursor }));
			// Write to IDB cache
			const cached = result.messages.map((m) => msgToCached(id, m));
			chatCache.appendMessages(id, cached).catch(() => {});
		} catch (err) {
			console.error("[Chat] loadMessages failed:", err);
		} finally {
			setLoadingMessages((prev) => ({ ...prev, [id]: false }));
		}
	}

	async function loadMoreMessages(convID: string): Promise<void> {
		const existing = messages()[convID] || [];
		if (existing.length === 0 || !hasMore()[convID]) return;
		const oldest = existing[0];
		setLoadingMessages((prev) => ({ ...prev, [convID]: true }));
		try {
			const result = await getConversationMessages(
				convID,
				oldest.id,
				MAX_MESSAGES,
			);
			batch(() => {
				setMessages((prev) => ({
					...prev,
					[convID]: [...result.messages, ...existing],
				}));
				setHasMore((prev) => ({ ...prev, [convID]: !!result.nextCursor }));
			});
			chatCache
				.prependMessages(
					convID,
					result.messages.map((m) => msgToCached(convID, m)),
				)
				.catch(() => {});
		} catch (err) {
			console.error("[Chat] loadMoreMessages failed:", err);
		} finally {
			setLoadingMessages((prev) => ({ ...prev, [convID]: false }));
		}
	}

	function sendMessage(convID: string, content: string): void {
		const conv = conversations().find((c) => c.conversation_id === convID);
		if (!conv) return;
		// WS emit: private:send — the socket store handles the event binding
		// We use the global socket adapter from the namespace
		import("@/socket/client").then(({ createSocketClient }) => {
			const adapter = createSocketClient();
			adapter.emitFireAndForget(EVENTS.PRIVATE_SEND, {
				target_identity: conv.other_identity,
				content,
			});
		});
	}

	// Start a new conversation with a given identity (open chat window)
	async function startConversation(identity: string): Promise<void> {
		// Check if an existing conversation exists
		const existing = conversations().find((c) => c.other_identity === identity);
		if (existing) {
			selectConversation(existing.conversation_id);
			return;
		}
		// No existing conversation — send a message to create one
		// We'll create a temporary convID and let the WS event fill it in
		const tempID = `new:${identity}`;
		setActiveConversationID(null);
		// The user will need to send a message to start the conversation
		// For now just set a marker
	}

	function handlePrivateNew(dto: MessageDTO): void {
		const convID = dto.conversationId;
		if (!convID) return;
		const currentMsgs = messages();
		const convMsgs = currentMsgs[convID] || [];

		// De-duplicate
		if (convMsgs.some((m) => m.id === dto.id)) return;

		batch(() => {
			setMessages((prev) => ({
				...prev,
				[convID]: [...(prev[convID] || []), dto],
			}));
			setConversations((prev) => {
				const existing = prev.find((c) => c.conversation_id === convID);
				const self = userStore.user()?.name || "";
				if (existing) {
					return prev.map((c) =>
						c.conversation_id === convID
							? {
									...c,
									last_content: dto.content,
									last_sender_identity: dto.senderIdentity,
									last_message_at: dto.timestamp,
									unread_count:
										convID === activeConversationID() ? 0 : c.unread_count + 1,
								}
							: c,
					);
				}
				// New conversation — prepend to list
				return [
					{
						conversation_id: convID,
						other_identity:
							dto.senderIdentity === self
								? dto.targetIdentity
								: dto.senderIdentity,
						other_display_name:
							dto.senderIdentity === self ? "" : dto.senderDisplay,
						other_avatar: "",
						last_content: dto.content,
						last_sender_identity: dto.senderIdentity,
						last_message_at: dto.timestamp,
						unread_count: convID === activeConversationID() ? 0 : 1,
					},
					...prev,
				];
			});
			// Also cache the message
			chatCache
				.appendMessages(convID, [msgToCached(convID, dto)])
				.catch(() => {});
		});
	}

	function markRead(convID: string): void {
		setConversations((prev) =>
			prev.map((c) =>
				c.conversation_id === convID ? { ...c, unread_count: 0 } : c,
			),
		);
		markConversationRead(convID).catch(() => {});
	}

	function totalUnread(): number {
		return conversations().reduce((sum, c) => sum + c.unread_count, 0);
	}

	return {
		conversations,
		activeConversationID,
		messages,
		hasMore,
		loadingList,
		loadingMessages,
		loadConversations,
		selectConversation,
		loadMoreMessages,
		sendMessage,
		startConversation,
		handlePrivateNew,
		markRead,
		totalUnread,
	};
});
