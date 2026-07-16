import { del, get, set } from "idb-keyval";
import { createSignal } from "solid-js";
import { bindAPIClientAuth } from "@/api/apiClientAuth";
import { getProfile, logout as logoutApi } from "@/api/auth";
import { requestAccessTokenByRefreshToken } from "@/api/authTransport";

export interface UserInfo {
	id: number;
	uuid: string;
	name: string;
	display_name: string;
	avatar: string;
	role: string;
}

const STORAGE_USER = "user";
const STORAGE_ACCESS_TOKEN = "accessToken";
const DB_REFRESH_TOKEN = "refreshToken";
/** access 剩余不足该时间时主动刷新，避免边界 401 */
const ACCESS_REFRESH_SKEW_MS = 60_000;

const [user, setUser] = createSignal<UserInfo | null>(null);
const [userStale, setUserStale] = createSignal(false);
const [accessToken, setAccessToken] = createSignal("");
const [refreshToken, setRefreshToken] = createSignal("");

let resolveAuthHydrated: () => void = () => {};
const authHydrated = new Promise<void>((resolve) => {
	resolveAuthHydrated = resolve;
});
let ensureSessionPromise: Promise<boolean> | null = null;

// 同步从 localStorage 恢复 user 和 accessToken，保证路由守卫立即可用
const cachedUser = localStorage.getItem(STORAGE_USER);
const cachedToken = localStorage.getItem(STORAGE_ACCESS_TOKEN);
if (cachedUser) {
	try {
		setUser(JSON.parse(cachedUser));
	} catch {
		localStorage.removeItem(STORAGE_USER);
	}
}
if (cachedToken) setAccessToken(cachedToken);

// refreshToken 在 IndexedDB，需等待恢复后再做无感刷新
get<string>(DB_REFRESH_TOKEN)
	.then((rt) => {
		if (rt) setRefreshToken(rt);
	})
	.finally(() => {
		resolveAuthHydrated();
	});

// 解码 JWT payload，返回任意字段；解析失败返回 null
function decodeJWTPayload<T = Record<string, any>>(token: string): T | null {
	try {
		const payload = token.split(".")[1];
		return JSON.parse(atob(payload)) as T;
	} catch {
		return null;
	}
}

function isTokenExpired(token: string, skewMs = 0): boolean {
	const payload = decodeJWTPayload<{ exp?: number }>(token);
	if (!payload || typeof payload.exp !== "number") return true;
	return Date.now() + skewMs >= payload.exp * 1000;
}

async function waitAuthHydrated() {
	await authHydrated;
}

async function loginAction(u: UserInfo, at: string, rt: string) {
	localStorage.setItem(STORAGE_USER, JSON.stringify(u));
	localStorage.setItem(STORAGE_ACCESS_TOKEN, at);
	await set(DB_REFRESH_TOKEN, rt);
	setUser(u);
	setUserStale(false);
	setAccessToken(at);
	setRefreshToken(rt);
}

async function updateAccessTokenAction(token: string) {
	localStorage.setItem(STORAGE_ACCESS_TOKEN, token);
	setAccessToken(token);
}

async function fetchProfileAction() {
	try {
		const profile = await getProfile();
		const updated = { ...user(), ...profile } as UserInfo;
		localStorage.setItem(STORAGE_USER, JSON.stringify(updated));
		setUser(updated);
		setUserStale(false);
	} catch {
		setUserStale(true);
	}
}

async function clearAuthAction() {
	localStorage.removeItem(STORAGE_USER);
	localStorage.removeItem(STORAGE_ACCESS_TOKEN);
	await del(DB_REFRESH_TOKEN);
	setUser(null);
	setUserStale(false);
	setAccessToken("");
	setRefreshToken("");
}

async function logoutAction() {
	try {
		if (accessToken()) {
			await logoutApi(refreshToken() || undefined);
		}
	} catch {
		// 即使服务端登出失败，也继续清除本地状态
	}
	await clearAuthAction();
}

/**
 * 确保会话可用：
 * - access 仍有效：直接通过
 * - access 过期/临近过期且 refresh 可用：静默换发 access
 * - 无法恢复：清会话并返回 false
 */
async function ensureSessionAction(): Promise<boolean> {
	await waitAuthHydrated();

	const at = accessToken();
	const rt = refreshToken();

	// access 仍有效（含 60s 余量）
	if (at && !isTokenExpired(at, ACCESS_REFRESH_SKEW_MS)) {
		return true;
	}

	// 没有 refresh，只能看 access 是否仍在有效期内
	if (!rt) {
		return !!(at && !isTokenExpired(at));
	}

	if (ensureSessionPromise) {
		return ensureSessionPromise;
	}

	ensureSessionPromise = (async () => {
		try {
			const newToken = await requestAccessTokenByRefreshToken(rt);
			await updateAccessTokenAction(newToken);
			return true;
		} catch {
			await clearAuthAction();
			return false;
		} finally {
			ensureSessionPromise = null;
		}
	})();

	return ensureSessionPromise;
}

bindAPIClientAuth({
	getAccessToken: accessToken,
	getRefreshToken: refreshToken,
	updateAccessToken: updateAccessTokenAction,
	clearAuth: clearAuthAction,
	waitAuthHydrated,
});

const userStore = {
	user,
	userStale,
	accessToken,
	refreshToken,
	/** 当前 access 仍有效（不含“可静默恢复”的过期会话） */
	isLoggedIn: () => {
		const token = accessToken();
		return !!token && !isTokenExpired(token);
	},
	/** 本地仍有会话痕迹（access 或 refresh），可供 ensureSession 尝试恢复 */
	hasSession: () => {
		return !!(accessToken() || refreshToken());
	},
	waitAuthHydrated,
	ensureSession: ensureSessionAction,
	login: loginAction,
	logout: logoutAction,
	clearAuth: clearAuthAction,
	updateAccessToken: updateAccessTokenAction,
	fetchProfile: fetchProfileAction,
};

export default userStore;
