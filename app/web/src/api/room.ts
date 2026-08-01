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
	type?: "text" | "voice";
	domain_uuid?: string;
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
	createdAt?: string;
	updatedAt?: string;
	domain_uuid?: string;
}

export async function createRoom(req: CreateRoomReq): Promise<RoomRecord> {
	const res = (await apiClient.post({
		url: "/api/v1/room/create",
		data: req,
	})) as AxiosResponse<Result<RoomRecord>>;

	if (!(res as any).data.data) throw new Error("room data is missing");
	return (res as any).data.data;
}
