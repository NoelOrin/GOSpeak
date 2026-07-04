import axios, {
	type AxiosError,
	type AxiosInstance,
	type AxiosRequestConfig,
	type AxiosResponse,
} from "axios";
import { showToast } from "solid-notifications";
import { getAPIClientAuth } from "@/api/apiClientAuth";
import { requestAccessTokenByRefreshToken } from "@/api/authTransport";

export interface Result<T = any> {
	code: number;
	msg: string;
	data?: T;
}

// Token 相关错误码
const TOKEN_ERROR_CODES = new Set([1001, 1002, 1003, 1014]);
// 直接登出的错误码（不尝试刷新）
const FORCE_LOGOUT_CODES = new Set([1015]);
// 禁言提示但不登出的错误码
const MUTE_ERROR_CODES = new Set([1016]);

// 刷新状态管理
let isRefreshing = false;
let refreshSubscribers: Array<(token: string) => void> = [];

function subscribeTokenRefresh(cb: (token: string) => void) {
	refreshSubscribers.push(cb);
}

function onTokenRefreshed(newToken: string) {
	refreshSubscribers.forEach((cb) => {
		cb(newToken);
	});
	refreshSubscribers = [];
}

const createInstance = (baseURL?: string) => {
	const axiosInstance = axios.create({
		baseURL: baseURL,
		timeout: 50000,
		headers: { "Content-Type": "application/json;charset=utf-8" },
	});

	// 请求拦截
	axiosInstance.interceptors.request.use(
		(config) => {
			const token = getAPIClientAuth().getAccessToken();
			if (token) {
				config.headers.Authorization = `Bearer ${token}`;
			}
			return config;
		},
		(error) => {
			return Promise.reject(error);
		},
	);

	// 响应拦截
	axiosInstance.interceptors.response.use(
		(res: AxiosResponse<Result>) => {
			return res;
		},
		async (error: AxiosError<Result>) => {
			// 请求被主动取消（如切换房间中断旧请求），静默忽略，不弹 toast
			if (error.code === "ERR_CANCELED" || error.name === "CanceledError") {
				return Promise.reject(error);
			}
			const auth = getAPIClientAuth();
			const originalRequest = error.config as AxiosRequestConfig & {
				_retry?: boolean;
			};

			// 被封禁等需要强制登出的错误
			if (
				error.response?.data?.code &&
				FORCE_LOGOUT_CODES.has(error.response.data.code)
			) {
				await auth.clearAuth();
				window.location.href = "/login?banned=1";
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
				// 有 refresh token 且未重试过，尝试刷新
				const refreshToken = auth.getRefreshToken();
				if (!originalRequest._retry && refreshToken) {
					if (isRefreshing) {
						return new Promise((resolve) => {
							subscribeTokenRefresh((newToken) => {
								if (originalRequest.headers) {
									originalRequest.headers.Authorization = `Bearer ${newToken}`;
								}
								resolve(axiosInstance(originalRequest));
							});
						});
					}

					isRefreshing = true;
					originalRequest._retry = true;

					try {
						const newToken =
							await requestAccessTokenByRefreshToken(refreshToken);
						await auth.updateAccessToken(newToken);
						onTokenRefreshed(newToken);

						if (originalRequest.headers) {
							originalRequest.headers.Authorization = `Bearer ${newToken}`;
						}
						return axiosInstance(originalRequest);
					} catch {
						// refresh 失败，清空等待队列再跳登录
						refreshSubscribers = [];
						await auth.clearAuth();
						window.location.href = "/login";
						return Promise.reject(error);
					} finally {
						isRefreshing = false;
					}
				}

				// 没有 refresh token，直接跳转登录
				await auth.clearAuth();
				window.location.href = "/login";
				return Promise.reject(error);
			}

			// 服务器错误或网络错误，通过 toast 提示
			const status = error.response?.status;
			if (!error.response) {
				showToast("网络连接失败，请检查网络", { type: "error" });
			} else if (status && status >= 500) {
				showToast(error.response.data?.msg || "服务器错误，请稍后重试", {
					type: "error",
				});
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
				.request<any, AxiosResponse<Result>>(config)
				.then((res: AxiosResponse<Result>) => {
					resolve(res as unknown as Promise<T>);
				})
				.catch((e: Error | AxiosError) => {
					reject(e);
				});
		});
	}
}

// export default new APIClient(import.meta.env.VITE_API_URL);
export default new APIClient("/");
