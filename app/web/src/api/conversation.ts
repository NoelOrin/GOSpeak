import apiClient from "./apiClient";
import type { PrivateMessageDTO } from "@/protocol/conversation";

export interface ConversationDTO {
	conversation_id: string;
	other_identity: string;
	other_display_name: string;
	other_avatar: string;
	last_content: string;
	last_sender_identity: string;
	last_message_at: number;
	unread_count: number;
}

export type { PrivateMessageDTO };

export interface PrivateMessageListResult {
	messages: PrivateMessageDTO[];
	next_cursor?: string;
}

export async function listConversations(
	limit?: number,
): Promise<ConversationDTO[]> {
	const data = await apiClient.post<ConversationDTO[]>({
		url: "/api/v1/conversation/list",
		data: { limit: limit || 50 },
	});
	return data ?? [];
}

export async function getConversationMessages(
	conversationID: string,
	before?: string,
	limit?: number,
): Promise<PrivateMessageListResult> {
	const data = await apiClient.post<PrivateMessageListResult>({
		url: "/api/v1/conversation/messages",
		data: {
			conversation_id: conversationID,
			before: before || "",
			limit: limit || 50,
		},
	});
	return data ?? { messages: [] };
}

export async function markConversationRead(
	conversationID: string,
): Promise<void> {
	await apiClient.post({
		url: "/api/v1/conversation/mark-read",
		data: { conversation_id: conversationID },
	});
}

export async function sendDirectMessage(body: {
	target_identity: string;
	content: string;
	client_nonce?: string;
}): Promise<PrivateMessageDTO> {
	const data = await apiClient.post<PrivateMessageDTO>({
		url: "/api/v1/conversation/send",
		data: body,
	});
	if (!data) throw new Error("send failed");
	return data;
}
