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

export async function createMute(
	params: CreateMuteParams,
): Promise<MuteRecord> {
	const data = await apiClient.post<MuteRecord>({
		url: "/api/v1/mute/create",
		data: params,
	});

	if (!data) throw new Error("mute record is missing");
	return data;
}

export async function cancelMute(userId: number): Promise<void> {
	await apiClient.post({
		url: "/api/v1/mute/cancel",
		data: { user_id: userId },
	});
}

export async function listMutes(): Promise<MuteRecord[]> {
	const data = await apiClient.post<MuteRecord[]>({
		url: "/api/v1/mute/list",
	});

	return data || [];
}
