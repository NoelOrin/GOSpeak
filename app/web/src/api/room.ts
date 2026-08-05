import apiClient from "./apiClient";

export interface CreateRoomReq {
	name: string;
	password?: string;
	description: string;
	limit: number;
	audio_only: boolean;
	allow_audience: boolean;
	type?: "text" | "voice";
	domain_uuid: string;
}

export interface RoomRecord {
	id: number;
	uuid: string;
	name: string;
	password?: string;
	description: string;
	limit: number;
	audio_only: boolean;
	allow_audience: boolean;
	type?: "text" | "voice";
	created_by?: string;
	created_at?: string;
	domain_uuid?: string;
}

export interface RoomListResult {
	rooms: RoomRecord[];
	total: number;
	page: number;
	size: number;
}

export interface UpdateRoomReq {
	id: number;
	name?: string;
	description?: string;
	limit?: number;
	audio_only?: boolean;
	allow_audience?: boolean;
}

export async function createRoom(req: CreateRoomReq): Promise<RoomRecord> {
	const data = await apiClient.post<RoomRecord>({
		url: "/api/v1/room/create",
		data: req,
	});

	if (!data) throw new Error("room data is missing");
	return data;
}

export async function listRooms(
	page: number,
	pageSize: number,
	domainUUID?: string,
): Promise<RoomListResult> {
	const data = await apiClient.post<RoomListResult>({
		url: "/api/v1/room/list",
		data: { page, page_size: pageSize, domain_uuid: domainUUID },
	});

	if (!data) throw new Error("room data is missing");
	return data;
}

export async function updateRoom(req: UpdateRoomReq): Promise<RoomRecord> {
	const data = await apiClient.post<RoomRecord>({
		url: "/api/v1/room/update",
		data: req,
	});

	if (!data) throw new Error("room data is missing");
	return data;
}

export async function deleteRoom(id: number): Promise<void> {
	await apiClient.post({ url: "/api/v1/room/delete", data: { id } });
}
