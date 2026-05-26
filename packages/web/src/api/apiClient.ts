import axios, {
	type AxiosRequestConfig,
	type AxiosError,
	type AxiosResponse,
	type AxiosInstance,
} from "axios";
import userStore from "@/stores/userStore";

export interface Result<T = any> {
	code: number;
	msg: string;
	data?: T;
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
		(error: AxiosError<Result>) => {
			console.error(error.message, "--");
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