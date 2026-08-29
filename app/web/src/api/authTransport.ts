import axios from "axios";

const rawAxios = axios.create({
	timeout: 50000,
	withCredentials: true,
	headers: { "Content-Type": "application/json;charset=utf-8" },
});

const SESSION_EXPIRES_KEY = "gospeak_session_expires_at";

export function readSessionExpiry(): number | null {
	const raw = localStorage.getItem(SESSION_EXPIRES_KEY);
	if (!raw) return null;
	const n = Number(raw);
	return Number.isFinite(n) && n > 0 ? n : null;
}

export function recordSessionExpiry(expiresIn: number | null | undefined) {
	if (typeof expiresIn === "number" && expiresIn > 0) {
		localStorage.setItem(
			SESSION_EXPIRES_KEY,
			String(Date.now() + expiresIn * 1000),
		);
		return;
	}
	localStorage.removeItem(SESSION_EXPIRES_KEY);
}

let pendingRefresh: Promise<number | null> | null = null;

/**
 * 由 HttpOnly refresh cookie 静默续期 access token，返回服务端下发的 expires_in（秒）。
 * 无论是拦截器还是 ensureSession 触发，过期时间都在这里统一持久化——
 * 401 被拦截器消化时 ensureSession 感知不到 refresh，只能靠这个入口记录。
 */
export async function refreshSession(): Promise<number | null> {
	if (!pendingRefresh) {
		pendingRefresh = rawAxios
			.post("/api/v1/auth/refresh_token")
			.then((resp) => {
				const data = resp.data?.data as { expires_in?: number } | undefined;
				const expiresIn =
					typeof data?.expires_in === "number" ? data.expires_in : null;
				recordSessionExpiry(expiresIn);
				return expiresIn;
			})
			.finally(() => {
				pendingRefresh = null;
			});
	}
	return pendingRefresh;
}
