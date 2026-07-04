import type { AxiosResponse } from "axios";
import type { Result } from "./apiClient";
import apiClient from "./apiClient";

export interface CreateRoomReq {
	name: string;
	password?: string;
	description: string;
	limit: number;
	audio_only: boolean;
	allow_audience: boolean;
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
	created_by?: string;
	createdAt?: string;
	updatedAt?: string;
}

export async function createRoom(req: CreateRoomReq): Promise<RoomRecord> {
	const res = (await apiClient.post({
		url: "/api/v1/room/create",
		data: req,
	})) as AxiosResponse<Result<RoomRecord>>;

	const result = res.data;
	if (result.code !== 0) throw new Error(result.msg);
	if (!result.data) throw new Error("room data is missing");
	return result.data;
}
