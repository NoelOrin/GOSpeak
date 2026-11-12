import type { MessageDTO, MessageListResult } from "@/types/message";
import apiClient from "./apiClient";

export async function listMessages(
	room_uuid: string,
	before?: string,
	limit = 100,
	password?: string,
): Promise<MessageListResult> {
	const data = await apiClient.post<MessageListResult>({
		url: "/api/v1/room/messages/list",
		// 平台级私密房间在服务端 requireRoomAccess 里校验密码，
		// 不带 password 会被 403 拒绝（域内房间走成员校验，不看这个字段）。
		data: { room_uuid, before, limit, password: password || undefined },
		skipErrorToast: true,
	});

	if (!data) throw new Error("messages missing");
	return data;
}

export async function searchMessages(
	room_uuid: string,
	query: string,
	password?: string,
): Promise<MessageDTO[]> {
	const data = await apiClient.post<{ items: MessageDTO[] }>({
		url: "/api/v1/room/messages/search",
		data: { room_uuid, query, password: password || undefined },
		skipErrorToast: true,
	});
	return data?.items ?? [];
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
