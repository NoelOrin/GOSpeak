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

/** 私聊消息 DTO — 与后端 service.MessageDTO 对齐 */
export interface PrivateMessageDTO {
	uuid: string;
	author_id: string;
	content: string;
	reply_to?: string;
	edited_at?: string | null;
	deleted: boolean;
	created_at: string;
	conversation_id?: string;
	target_identity?: string;
	client_nonce?: string;
}

export interface PrivateMessageListResult {
	messages: PrivateMessageDTO[];
	next_cursor?: string;
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
): Promise<PrivateMessageListResult> {
	const res = (await apiClient.post({
		url: "/api/v1/conversation/messages",
		data: {
			conversation_id: conversationID,
			before: before || "",
			limit: limit || 50,
		},
	})) as AxiosResponse<Result<PrivateMessageListResult>>;
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

export async function sendDirectMessage(body: {
	target_identity: string;
	content: string;
	client_nonce?: string;
}): Promise<PrivateMessageDTO> {
	const res = (await apiClient.post({
		url: "/api/v1/conversation/send",
		data: body,
	})) as AxiosResponse<Result<PrivateMessageDTO>>;
	if (!res.data.data) throw new Error("send failed");
	return res.data.data;
}
