import { createSignal } from "solid-js";
import { getProfile, logout as logoutApi } from "@/api/auth";
import { refreshSession } from "@/api/authTransport";

export interface UserInfo {
	id: number;
	uuid: string;
	name: string;
	display_name: string;
	avatar: string;
	role: string;
	is_guest?: boolean;
	permissions?: string[];
}

const STORAGE_USER = "user";

const [user, setUser] = createSignal<UserInfo | null>(null);
const [userStale, setUserStale] = createSignal(false);

let ensureSessionPromise: Promise<boolean> | null = null;

// 只缓存非敏感 user 元数据；access/refresh token 完全由 HttpOnly Cookie 承载。
const cachedUser = localStorage.getItem(STORAGE_USER);
if (cachedUser) {
	try {
		setUser(JSON.parse(cachedUser));
	} catch {
		localStorage.removeItem(STORAGE_USER);
	}
}

async function loginAction(u: UserInfo) {
	localStorage.setItem(STORAGE_USER, JSON.stringify(u));
	setUser(u);
	setUserStale(false);
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
	setUser(null);
	setUserStale(false);
}

async function logoutAction() {
	try {
		await logoutApi();
	} catch {
		// 即使服务端登出失败，也继续清除本地状态
	}
	await clearAuthAction();
}

/**
 * 确保会话可用：
 * - 由 HttpOnly refresh cookie 静默换发 access cookie
 * - 成功后拉取最新 profile；失败则清会话并返回 false
 */
async function ensureSessionAction(): Promise<boolean> {
	if (ensureSessionPromise) {
		return ensureSessionPromise;
	}

	ensureSessionPromise = (async () => {
		try {
			await refreshSession();
			await fetchProfileAction();
			return !!user();
		} catch {
			await clearAuthAction();
			return false;
		} finally {
			ensureSessionPromise = null;
		}
	})();

	return ensureSessionPromise;
}

const userStore = {
	user,
	userStale,
	/** 本地有用户元数据；真实会话有效性以服务端 Cookie 鉴权为准 */
	isLoggedIn: () => !!user(),
	hasSession: () => !!user(),
	ensureSession: ensureSessionAction,
	login: loginAction,
	logout: logoutAction,
	clearAuth: clearAuthAction,
	fetchProfile: fetchProfileAction,
};

export default userStore;
