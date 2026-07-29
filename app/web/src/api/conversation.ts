import type { AxiosResponse } from "axios";
import type { Result } from "./apiClient";
import apiClient from "./apiClient";

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

export interface MessageDTO {
	id: string;
	room: string;
	content: string;
	replyTo: string;
	senderIdentity: string;
	senderDisplay: string;
	senderRole: string;
	timestamp: number;
	conversationId: string;
	targetIdentity: string;
}

export interface MessageListResult {
	messages: MessageDTO[];
	nextCursor?: string;
}

export async function listConversations(
	limit?: number,
): Promise<ConversationDTO[]> {
	const res = (await apiClient.post({
		url: "/api/v1/conversation/list",
		data: { limit: limit || 50 },
	})) as AxiosResponse<Result<ConversationDTO[]>>;
	return res.data.data ?? [];
}

export async function getConversationMessages(
	conversationID: string,
	before?: string,
	limit?: number,
): Promise<MessageListResult> {
	const res = (await apiClient.post({
		url: "/api/v1/conversation/messages",
		data: {
			conversation_id: conversationID,
			before: before || "",
			limit: limit || 50,
		},
	})) as AxiosResponse<Result<MessageListResult>>;
	return res.data.data ?? { messages: [] };
}

export async function markConversationRead(
	conversationID: string,
): Promise<void> {
	await apiClient.post({
		url: "/api/v1/conversation/mark-read",
		data: { conversation_id: conversationID },
	});
}
