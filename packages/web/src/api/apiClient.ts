import axios, {
	type AxiosRequestConfig,
	type AxiosError,
	type AxiosResponse,
	type AxiosInstance,
} from "axios";

export interface Result<T = any> {
	code: number;
	message: string;
	data?: T;
	msg: string;
	state: number;
	toast?: string;
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
			return config;
		},
		(error) => {
			// 请求错误时做些什么
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