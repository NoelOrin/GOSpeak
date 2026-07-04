import type { AxiosResponse } from "axios";
import type { Result } from "./apiClient";
import apiClient from "./apiClient";

export interface MuteRecord {
	id: number;
	uuid: string;
	user_id: number;
	muter_id: number;
	duration: number;
	permanent: boolean;
	expires_at: string | null;
	reason: string;
	created_at: string;
	updated_at: string;
}

export interface CreateMuteParams {
	user_id: number;
	duration: number;
	permanent: boolean;
	reason: string;
}

export async function createMute(params: CreateMuteParams): Promise<MuteRecord> {
	const res = (await apiClient.post({
		url: "/api/v1/mute/create",
		data: params,
	})) as AxiosResponse<Result<MuteRecord>>;

	const result = res.data;
	if (result.code !== 0) throw new Error(result.msg);
	if (!result.data) throw new Error("mute record is missing");
	return result.data;
}

export async function cancelMute(userId: number): Promise<void> {
	const res = (await apiClient.post({
		url: "/api/v1/mute/cancel",
		data: { user_id: userId },
	})) as AxiosResponse<Result>;

	const result = res.data;
	if (result.code !== 0) throw new Error(result.msg);
}

export async function getMuteStatus(
	userId: number,
): Promise<MuteRecord | null> {
	const res = (await apiClient.post({
		url: "/api/v1/mute/status",
		data: { user_id: userId },
	})) as AxiosResponse<Result<MuteRecord | null>>;

	const result = res.data;
	if (result.code !== 0) throw new Error(result.msg);
	return result.data ?? null;
}

export async function listMutes(): Promise<MuteRecord[]> {
	const res = (await apiClient.post({
		url: "/api/v1/mute/list",
	})) as AxiosResponse<Result<MuteRecord[]>>;

	const result = res.data;
	if (result.code !== 0) throw new Error(result.msg);
	return result.data || [];
}
