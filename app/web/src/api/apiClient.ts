import axios, {
	type AxiosError,
	type AxiosInstance,
	type AxiosRequestConfig,
	type AxiosResponse,
} from "axios";
import { showToast } from "solid-notifications";
import { refreshSession } from "@/api/authTransport";

export interface Result<T = any> {
	code: number;
	msg: string;
	data?: T;
}

// Token 相关错误码
const TOKEN_ERROR_CODES = new Set([1001, 1002, 1003, 1014]);
const RATE_LIMITED_CODE = 1017;
// 直接登出的错误码（不尝试刷新）
const FORCE_LOGOUT_CODES = new Set([1015]);
// 禁言提示但不登出的错误码
const MUTE_ERROR_CODES = new Set([1016]);

// 已在登录/注册页时不再硬跳，避免与 ensureSession 的路由跳转互相触发整页刷新循环
const redirectToLogin = (banned = false) => {
	if (
		window.location.pathname === "/login" ||
		window.location.pathname === "/register"
	) {
		return;
	}
	window.location.href = banned ? "/login?banned=1" : "/login";
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
				showToast(res.data.msg, { type: "error" });
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

			// 被封禁等需要强制登出的错误
			if (
				error.response?.data?.code &&
				FORCE_LOGOUT_CODES.has(error.response.data.code)
			) {
				redirectToLogin(true);
				return Promise.reject(error);
			}

			// 被禁言显示提示但不登出
			if (
				error.response?.data?.code &&
				MUTE_ERROR_CODES.has(error.response.data.code)
			) {
				showToast(error.response.data?.msg || "你已被禁言", {
					type: "warning",
				});
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
			if (!error.response) {
				showToast("网络连接失败，请检查网络", { type: "error" });
			} else if (status && status >= 500) {
				showToast(error.response.data?.msg || "服务器错误，请稍后重试", {
					type: "error",
				});
			} else if (status && status >= 400) {
				showToast(error.response.data?.msg || "请求失败", { type: "error" });
			}

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
						reject(new Error(res.data.msg));
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
