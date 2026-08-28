import axios from "axios";

const rawAxios = axios.create({
	timeout: 50000,
	withCredentials: true,
	headers: { "Content-Type": "application/json;charset=utf-8" },
});

let pendingRefresh: Promise<number | null> | null = null;

/** 由 HttpOnly refresh cookie 静默续期 access token，返回服务端下发的 expires_in（秒）。 */
export async function refreshSession(): Promise<number | null> {
	if (!pendingRefresh) {
		pendingRefresh = rawAxios
			.post("/api/v1/auth/refresh_token")
			.then((resp) => {
				const data = resp.data?.data as { expires_in?: number } | undefined;
				return typeof data?.expires_in === "number" ? data.expires_in : null;
			})
			.finally(() => {
				pendingRefresh = null;
			});
	}
	return pendingRefresh;
}
