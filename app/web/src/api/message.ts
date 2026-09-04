import type { MessageDTO, MessageListResult } from "@/types/message";
import apiClient from "./apiClient";

export async function listMessages(
	room_uuid: string,
	before?: string,
	limit = 100,
): Promise<MessageListResult> {
	const data = await apiClient.post<MessageListResult>({
		url: "/api/v1/room/messages/list",
		data: { room_uuid, before, limit },
	});

	if (!data) throw new Error("messages missing");
	return data;
}

export async function searchMessages(
	room_uuid: string,
	query: string,
): Promise<MessageDTO[]> {
	const data = await apiClient.post<{ items: MessageDTO[] }>({
		url: "/api/v1/room/messages/search",
		data: { room_uuid, query },
	});
	return data?.items ?? [];
}

export async function sendMessage(body: {
	room_uuid: string;
	content: string;
	reply_to?: string;
	mentions?: string[];
	client_nonce?: string;
}): Promise<MessageDTO> {
	const data = await apiClient.post<MessageDTO>({
		url: "/api/v1/room/messages/send",
		data: body,
	});

	if (!data) throw new Error("send failed");
	return data;
}

export async function editMessage(
	room_uuid: string,
	message_uuid: string,
	content: string,
): Promise<MessageDTO> {
	const data = await apiClient.post<MessageDTO>({
		url: "/api/v1/room/messages/edit",
		data: { room_uuid, message_uuid, content },
	});

	if (!data) throw new Error("edit failed");
	return data;
}

export async function deleteMessage(
	room_uuid: string,
	message_uuid: string,
): Promise<void> {
	await apiClient.post({
		url: "/api/v1/room/messages/delete",
		data: { room_uuid, message_uuid },
	});
}

export async function reactMessage(
	room_uuid: string,
	message_uuid: string,
	emoji: string,
): Promise<MessageDTO> {
	const data = await apiClient.post<MessageDTO>({
		url: "/api/v1/room/messages/react",
		data: { room_uuid, message_uuid, emoji },
	});

	if (!data) throw new Error("react failed");
	return data;
}

export async function unreactMessage(
	room_uuid: string,
	message_uuid: string,
	emoji: string,
): Promise<void> {
	await apiClient.post({
		url: "/api/v1/room/messages/unreact",
		data: { room_uuid, message_uuid, emoji },
	});
}
