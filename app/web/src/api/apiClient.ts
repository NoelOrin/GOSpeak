import axios, {
	type AxiosError,
	type AxiosInstance,
	type AxiosRequestConfig,
	type AxiosResponse,
} from "axios";
import { toast } from "@gospeak/components";
import { refreshSession } from "@/api/authTransport";

export interface Result<T = any> {
	code: number;
	msg: string;
	data?: T;
}

declare module "axios" {
	interface AxiosRequestConfig<D = any> {
		/**
		 * 为 true 时拦截器不弹统一错误提示，由调用方自己展示文案。
		 * 用于后端 msg 是英文/不友好、需要前端定制中文提示的接口。
		 */
		skipErrorToast?: boolean;
	}
}

/** 拦截器已提示过的错误都会打上这个标记，避免调用方再弹一次（重复 toast）。 */
type ToastMarked = { __toastShown?: boolean };

const markToasted = (error: unknown) => {
	if (error && typeof error === "object") {
		(error as ToastMarked).__toastShown = true;
	}
};

/** 拦截器主动闭嘴的请求：显式标记为「未提示」，调用方仍需自己弹。 */
const markSilent = (error: unknown) => {
	if (error && typeof error === "object") {
		(error as ToastMarked).__toastShown = false;
	}
};

// Token 相关错误码
const TOKEN_ERROR_CODES = new Set([1001, 1002, 1003, 1014]);
const RATE_LIMITED_CODE = 1017;
// 直接登出的错误码（不尝试刷新）
const FORCE_LOGOUT_CODES = new Set([1015]);
// 禁言提示但不登出的错误码
const MUTE_ERROR_CODES = new Set([1016]);

/**
 * 这些路径上的 401 属于「尚未登录」的正常状态，不能由拦截器整页硬跳登录：
 * - login / register：避免与 ensureSession 的路由跳转互相触发整页刷新循环
 * - guest / invite：对外公开入口，路由 beforeLoad 会带着邀请码决定去向；
 *   拦截器硬跳 /login 会吞掉邀请码，让所有分享链接失效
 */
export function shouldSkipLoginRedirect(path: string): boolean {
	return (
		path === "/login" ||
		path === "/register" ||
		path === "/guest" ||
		path.startsWith("/invite/")
	);
}

const redirectToLogin = (banned = false) => {
	const path = window.location.pathname;
	// 封禁是强制登出信号，公开入口也要走；但已在登录页时不要重复刷新
	if (banned) {
		if (path !== "/login") window.location.href = "/login?banned=1";
		return;
	}
	if (shouldSkipLoginRedirect(path)) return;
	// 记录来源：登录后 resolveLoginRedirect 会把用户带回原来的页面，
	// 而不是一律落到 / 再被送去 /discover。
	try {
		sessionStorage.setItem("gospeak_redirect", path + window.location.search);
	} catch {
		// 隐私模式等场景下 sessionStorage 可能不可写，不影响跳登录
	}
	window.location.href = "/login";
};

const createInstance = (baseURL?: string) => {
	const axiosInstance = axios.create({
		baseURL: baseURL,
		timeout: 50000,
		withCredentials: true,
		headers: { "Content-Type": "application/json;charset=utf-8" },
	});

	// 响应拦截 — 统一处理接口错误 toast，组件不再重复处理
	axiosInstance.interceptors.response.use(
		(res: AxiosResponse<Result>) => {
			// 业务失败码也统一 toast（当前后端走 HTTP 4xx/5xx，此分支保底）
			if (res.data && res.data.code !== 0) {
				const silent = res.config?.skipErrorToast === true;
				if (!silent) {
					toast.error(res.data.msg);
				}
				// 记录给 APIClient.request 的 reject 用，保证调用方能判断是否已提示
				(res.config as ToastMarked).__toastShown = !silent;
			}
			return res;
		},
		async (error: AxiosError<Result>) => {
			// 请求被主动取消，静默忽略
			if (error.code === "ERR_CANCELED" || error.name === "CanceledError") {
				return Promise.reject(error);
			}
			const originalRequest = error.config as AxiosRequestConfig & {
				_retry?: boolean;
			};
			// 调用方要自己展示文案的接口，拦截器全程闭嘴
			const silent = originalRequest?.skipErrorToast === true;
			if (silent) markSilent(error);

			// 被封禁等需要强制登出的错误
			if (
				error.response?.data?.code &&
				FORCE_LOGOUT_CODES.has(error.response.data.code)
			) {
				// 跳登录页本身就是提示，标记为已提示避免调用方再弹一条
				markToasted(error);
				redirectToLogin(true);
				return Promise.reject(error);
			}

			// 被禁言显示提示但不登出
			if (
				error.response?.data?.code &&
				MUTE_ERROR_CODES.has(error.response.data.code)
			) {
				if (!silent) {
					toast.warning(error.response.data?.msg || "你已被禁言");
					markToasted(error);
				}
				return Promise.reject(error);
			}

			// 有响应体且是 token 相关错误
			if (
				error.response?.data?.code &&
				TOKEN_ERROR_CODES.has(error.response.data.code)
			) {
				if (!originalRequest._retry) {
					originalRequest._retry = true;
					try {
						await refreshSession();
						return axiosInstance(originalRequest);
					} catch (e) {
						const code = (e as AxiosError<Result>)?.response?.data?.code;
						// refresh 被限流(1017)：沿用现有会话重试原调用方自行处理，不跳登录
						if (code === RATE_LIMITED_CODE) return Promise.reject(error);
						redirectToLogin();
						return Promise.reject(error);
					}
				}
				redirectToLogin();
				return Promise.reject(error);
			}

			// 其余 HTTP/网络错误，统一 toast
			const status = error.response?.status;
			if (silent) return Promise.reject(error);
			if (!error.response) {
				toast.error("网络连接失败，请检查网络");
			} else if (status && status >= 500) {
				toast.error(error.response.data?.msg || "服务器错误，请稍后重试");
			} else if (status && status >= 400) {
				toast.error(error.response.data?.msg || "请求失败");
			} else {
				return Promise.reject(error);
			}
			markToasted(error);

			return Promise.reject(error);
		},
	);
	return axiosInstance;
};

export class APIClient {
	private axiosInstance: AxiosInstance;
	constructor(baseUrl?: string) {
		this.axiosInstance = createInstance(baseUrl);
	}

	get<T = any, U = any>(config: AxiosRequestConfig<U>): Promise<T> {
		return this.request({ ...config, method: "GET" });
	}

	post<T = any, U = any>(config: AxiosRequestConfig<U>): Promise<T> {
		return this.request({ ...config, method: "POST" });
	}

	put<T = any, U = any>(config: AxiosRequestConfig<U>): Promise<T> {
		return this.request({ ...config, method: "PUT" });
	}

	delete<T = any, U = any>(config: AxiosRequestConfig<U>): Promise<T> {
		return this.request({ ...config, method: "DELETE" });
	}

	request<T = any, U = any>(config: AxiosRequestConfig<U>): Promise<T> {
		return new Promise((resolve, reject) => {
			this.axiosInstance
				.request<any, AxiosResponse<Result<T>>>(config)
				.then((res: AxiosResponse<Result<T>>) => {
					if (res.data && res.data.code !== 0) {
						// 拦截器已经弹过提示（或调用方声明跳过），把标记带给调用方
						const err = new Error(res.data.msg) as Error & ToastMarked;
						err.__toastShown =
							(res.config as ToastMarked)?.__toastShown === true;
						reject(err);
						return;
					}
					resolve(res.data?.data as T);
				})
				.catch((e: Error | AxiosError) => {
					reject(e);
				});
		});
	}
}

export default new APIClient("/");
