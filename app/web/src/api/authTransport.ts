import axios from "axios";

const rawAxios = axios.create({
	timeout: 50000,
	withCredentials: true,
	headers: { "Content-Type": "application/json;charset=utf-8" },
});

let pendingRefresh: Promise<void> | null = null;

/** 由 HttpOnly refresh cookie 静默续期 access token，浏览器侧无需持有任何 token。 */
export async function refreshSession(): Promise<void> {
	if (!pendingRefresh) {
		pendingRefresh = rawAxios
			.post("/api/v1/auth/refresh_token")
			.then(() => undefined)
			.finally(() => {
				pendingRefresh = null;
			});
	}
	await pendingRefresh;
}
