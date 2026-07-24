import type { AxiosResponse } from "axios";
import type { MessageDTO, MessageListResult } from "@/types/message";
import type { Result } from "./apiClient";
import apiClient from "./apiClient";

export async function listMessages(
	room_uuid: string,
	before?: string,
	limit = 100,
): Promise<MessageListResult> {
	const res = (await apiClient.post({
		url: "/api/v1/room/messages/list",
		data: { room_uuid, before, limit },
	})) as AxiosResponse<Result<MessageListResult>>;

	if (!(res as any).data.data) throw new Error("messages missing");
	return (res as any).data.data as MessageListResult;
}

export async function sendMessage(body: {
	room_uuid: string;
	content: string;
	reply_to?: string;
	mentions?: string[];
	client_nonce?: string;
}): Promise<MessageDTO> {
	const res = (await apiClient.post({
		url: "/api/v1/room/messages/send",
		data: body,
	})) as AxiosResponse<Result<MessageDTO>>;

	if (!(res as any).data.data) throw new Error("send failed");
	return (res as any).data.data as MessageDTO;
}

export async function editMessage(
	room_uuid: string,
	message_uuid: string,
	content: string,
): Promise<MessageDTO> {
	const res = (await apiClient.post({
		url: "/api/v1/room/messages/edit",
		data: { room_uuid, message_uuid, content },
	})) as AxiosResponse<Result<MessageDTO>>;

	if (!(res as any).data.data) throw new Error("edit failed");
	return (res as any).data.data as MessageDTO;
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
	const res = (await apiClient.post({
		url: "/api/v1/room/messages/react",
		data: { room_uuid, message_uuid, emoji },
	})) as AxiosResponse<Result<MessageDTO>>;

	if (!(res as any).data.data) throw new Error("react failed");
	return (res as any).data.data as MessageDTO;
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
