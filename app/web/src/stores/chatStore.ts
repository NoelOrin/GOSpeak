import { createRoot, createSignal } from "solid-js";
import { listMessages } from "@/api/message";
import { EVENTS } from "@/socket/events";
import { socketStore } from "@/stores/socketStore";
import type { MessageDTO } from "@/types/message";

// ---------------------------------------------------------------------------
// Pure util — exported for testing
// ---------------------------------------------------------------------------

export function mergeMessages(
	existing: MessageDTO[],
	incoming: MessageDTO[],
): MessageDTO[] {
	const map = new Map<string, MessageDTO>();
	for (const m of existing) map.set(m.uuid, m);
	for (const m of incoming) map.set(m.uuid, m);
	const result = Array.from(map.values());
	result.sort((a, b) => a.created_at.localeCompare(b.created_at));
	return result;
}

// ---------------------------------------------------------------------------
// Chat store
// ---------------------------------------------------------------------------

export const chatStore = createRoot(() => {
	const [textRoom, setTextRoom] = createSignal<string | null>(null);
	const [textRoomName, setTextRoomName] = createSignal<string | null>(null);
	const [messages, setMessages] = createSignal<MessageDTO[]>([]);
	const [hasMore, setHasMore] = createSignal(false);
	const [nextBefore, setNextBefore] = createSignal<string | null>(null);
	const [isAtBottom, setIsAtBottom] = createSignal(true);
	const [loading, setLoading] = createSignal(false);

	// Nonces currently waiting for a server ack / created event
	let pendingNonces = new Set<string>();

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

		socket.on(EVENTS.MESSAGE_CREATED, handleCreated);
		socket.on(EVENTS.MESSAGE_UPDATED, handleUpdated);
		socket.on(EVENTS.MESSAGE_DELETED, handleDeleted);
		socket.on(EVENTS.MESSAGE_REACTION, handleReaction);
		socket.on(EVENTS.MESSAGE_ERROR, handleError);

		socketCleanups = [
			() => socket.off(EVENTS.MESSAGE_CREATED, handleCreated),
			() => socket.off(EVENTS.MESSAGE_UPDATED, handleUpdated),
			() => socket.off(EVENTS.MESSAGE_DELETED, handleDeleted),
			() => socket.off(EVENTS.MESSAGE_REACTION, handleReaction),
			() => socket.off(EVENTS.MESSAGE_ERROR, handleError),
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
			setMessages((prev) =>
				prev.filter((m) => m.client_nonce !== data.client_nonce),
			);
			pendingNonces.delete(data.client_nonce);
		}
	}

	// ------------------------------------------------------------------
	// Public API
	// ------------------------------------------------------------------

	async function joinTextRoom(room: {
		uuid: string;
		name: string;
		password?: string;
	}) {
		setTextRoom(room.uuid);
		setTextRoomName(room.name);
		setMessages([]);
		setHasMore(false);
		setNextBefore(null);
		setLoading(true);
		addListeners();
		try {
			// Emit room:join so the Hub registers this socket in connSlots
			const socket = socketStore.getSocket();
			if (socket?.connected) {
				socket.emit(
					EVENTS.ROOM_JOIN,
					JSON.stringify({
						room: room.name,
						...(room.password ? { password: room.password } : {}),
					}),
				);
			}
			await loadInitial();
		} finally {
			setLoading(false);
		}
	}

	function leaveTextRoom() {
		removeListeners();
		setTextRoom(null);
		setTextRoomName(null);
		setMessages([]);
		setHasMore(false);
		setNextBefore(null);
		pendingNonces = new Set();
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

		pendingNonces.add(client_nonce);
		setMessages((prev) => mergeMessages(prev, [pending]));

		// Emit via socket (server expects a JSON string payload)
		const socket = socketStore.getSocket();
		if (socket?.connected) {
			socket.emit(
				EVENTS.MESSAGE_SEND,
				JSON.stringify({
					room: roomName,
					content,
					reply_to: opts?.reply_to ?? "",
					mentions: opts?.mentions ?? [],
					client_nonce,
				}),
				(ack: string) => {
					try {
						const resp = typeof ack === "string" ? JSON.parse(ack) : ack;
						if (resp?.error) {
							// Remove pending on failure
							setMessages((prev) =>
								prev.filter((m) => m.client_nonce !== client_nonce),
							);
							pendingNonces.delete(client_nonce);
						}
					} catch {
						setMessages((prev) =>
							prev.filter((m) => m.client_nonce !== client_nonce),
						);
						pendingNonces.delete(client_nonce);
					}
				},
			);
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
		if (socket?.connected) {
			socket.emit(
				EVENTS.MESSAGE_DELETE,
				JSON.stringify({ room: roomName, message_uuid: messageUUID }),
			);
		}
	}

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
	};
});
