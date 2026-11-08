import type { ConversationDTO, PrivateMessageDTO } from "@/api/conversation";
import type { MessageDTO } from "@/types/message";

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

export function mergePrivateMessages(
	existing: PrivateMessageDTO[],
	incoming: PrivateMessageDTO[],
): PrivateMessageDTO[] {
	const byUUID = new Map<string, PrivateMessageDTO>();
	const byNonce = new Map<string, PrivateMessageDTO>();

	for (const m of existing) {
		byUUID.set(m.uuid, m);
		if (m.client_nonce) byNonce.set(m.client_nonce, m);
	}
	for (const m of incoming) {
		byUUID.set(m.uuid, m);
		if (m.client_nonce) {
			const pending = byNonce.get(m.client_nonce);
			if (pending && pending.uuid !== m.uuid) byUUID.delete(pending.uuid);
			byNonce.set(m.client_nonce, m);
		}
	}

	const result = Array.from(byUUID.values());
	result.sort((a, b) => a.created_at.localeCompare(b.created_at));
	return result;
}

export function remapRecordKey<T>(
	records: Record<string, T>,
	from: string,
	to: string,
): Record<string, T> {
	if (!from || !to || from === to || !(from in records)) return records;
	const next = { ...records };
	next[to] = to in next ? next[to] : next[from];
	delete next[from];
	return next;
}

export function remapPrivateMessages(
	records: Record<string, PrivateMessageDTO[]>,
	from: string,
	to: string,
): Record<string, PrivateMessageDTO[]> {
	if (!from || !to || from === to || !(from in records)) return records;
	const next = { ...records };
	const moved = (next[from] || []).map((m) => ({
		...m,
		conversation_id: to,
	}));
	delete next[from];
	next[to] = mergePrivateMessages(next[to] || [], moved);
	return next;
}

export function remapConversationList(
	conversations: ConversationDTO[],
	from: string,
	to: string,
): ConversationDTO[] {
	if (!from || !to || from === to) return conversations;
	const tempIndex = conversations.findIndex((c) => c.conversation_id === from);
	if (tempIndex === -1) return conversations;
	const realIndex = conversations.findIndex((c) => c.conversation_id === to);
	const temp = conversations[tempIndex];
	const real = realIndex >= 0 ? conversations[realIndex] : undefined;
	const merged: ConversationDTO = {
		...real,
		...temp,
		conversation_id: to,
		other_identity: real?.other_identity || temp.other_identity,
		other_display_name: real?.other_display_name || temp.other_display_name,
		other_avatar: real?.other_avatar || temp.other_avatar,
		last_content: real?.last_content || temp.last_content,
		last_sender_identity:
			real?.last_sender_identity || temp.last_sender_identity,
		last_message_at: real?.last_message_at || temp.last_message_at,
		unread_count: real?.unread_count ?? temp.unread_count,
	};
	const out = [...conversations];
	out.splice(tempIndex, 1);
	const target =
		realIndex === -1
			? Math.min(tempIndex, out.length)
			: realIndex > tempIndex
				? realIndex - 1
				: realIndex;
	out.splice(target, realIndex === -1 ? 0 : 1, merged);
	return out;
}
