import axios, {
	type AxiosRequestConfig,
	type AxiosError,
	type AxiosResponse,
	type AxiosInstance,
} from "axios";
import userStore from "@/stores/userStore";
import { refreshToken } from "@/api/auth";

export interface Result<T = any> {
	code: number;
	msg: string;
	data?: T;
}

// Token 相关错误码
const TOKEN_ERROR_CODES = new Set([1001, 1002, 1003, 1014]);

// 刷新状态管理
let isRefreshing = false;
let refreshSubscribers: Array<(token: string) => void> = [];

function subscribeTokenRefresh(cb: (token: string) => void) {
	refreshSubscribers.push(cb);
}

function onTokenRefreshed(newToken: string) {
	refreshSubscribers.forEach((cb) => cb(newToken));
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
			const token = userStore.accessToken();
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
			const originalRequest = error.config as AxiosRequestConfig & {
				_retry?: boolean;
			};

			// 有响应体且是 token 相关错误
			if (
				error.response?.data?.code &&
				TOKEN_ERROR_CODES.has(error.response.data.code)
			) {
				// 有 refresh token 且未重试过，尝试刷新
				if (!originalRequest._retry && userStore.refreshToken()) {
					if (isRefreshing) {
						return new Promise((resolve) => {
							subscribeTokenRefresh((newToken) => {
								if (originalRequest.headers) {
									originalRequest.headers.Authorization =
										`Bearer ${newToken}`;
								}
								resolve(axiosInstance(originalRequest));
							});
						});
					}

					isRefreshing = true;
					originalRequest._retry = true;

					try {
						const newToken = await refreshToken(
							userStore.refreshToken(),
						);
						await userStore.updateAccessToken(newToken);
						onTokenRefreshed(newToken);

						if (originalRequest.headers) {
							originalRequest.headers.Authorization =
								`Bearer ${newToken}`;
						}
						return axiosInstance(originalRequest);
					} catch {
						// refresh 也失败，才跳转登录
						await userStore.logout();
						window.location.href = "/login";
						return Promise.reject(error);
					} finally {
						isRefreshing = false;
					}
				}

				// 没有 refresh token，直接跳转登录
				await userStore.logout();
				window.location.href = "/login";
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
