import type { AxiosResponse } from "axios";
import axios from "axios";
import type { Result } from "./apiClient";

const rawAxios = axios.create({
	timeout: 50000,
	headers: { "Content-Type": "application/json;charset=utf-8" },
});

export async function requestAccessTokenByRefreshToken(
	refreshToken: string,
): Promise<string> {
	const res = (await rawAxios.post("/api/v1/auth/refresh_token", {
		refresh_token: refreshToken,
	})) as AxiosResponse<Result<{ access_token: string }>>;

	const result = res.data;
	if (result.code !== 0) throw new Error(result.msg);
	if (!result.data?.access_token) throw new Error("access_token is missing");
	return result.data.access_token;
}
