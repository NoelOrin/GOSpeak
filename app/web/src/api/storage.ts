import apiClient from "./apiClient";

// ===== 类型定义 =====

export interface StorageConfigView {
	provider_type: "s3" | "local";
	endpoint: string;
	bucket: string;
	region: string;
	/** 管理端读取时始终为空；提交留空表示保留旧值 */
	access_key: string;
	access_key_set?: boolean;
	secret_key: string;
	secret_key_set?: boolean;
	public_base_url: string;
	path_prefix: string;
	max_file_size: number;
	allowed_types: string;
}

export interface StorageConfigInput {
	provider_type: "s3" | "local";
	endpoint?: string;
	bucket?: string;
	region?: string;
	access_key?: string;
	secret_key?: string;
	public_base_url?: string;
	path_prefix?: string;
	max_file_size?: number;
	allowed_types?: string;
}

export interface PresignResult {
	provider_type: "s3" | "local";
	upload_url: string | null;
	object_key: string;
	public_url?: string;
}

// ===== API 函数 =====

/** 获取预签名上传 URL */
export async function presignUpload(params: {
	file_name: string;
	content_type: string;
	file_size: number;
	category: string;
}): Promise<PresignResult> {
	const data = await apiClient.post<PresignResult>({
		url: "/api/v1/storage/presign",
		data: params,
	});

	return data;
}

/** 确认 S3 上传完成 */
export async function confirmUpload(
	objectKey: string,
): Promise<{ public_url: string }> {
	const data = await apiClient.post<{ public_url: string }>({
		url: "/api/v1/storage/confirm",
		data: { object_key: objectKey },
	});

	return data;
}

/** 本地模式中转上传 */
export async function uploadFile(
	file: File,
	objectKey: string,
): Promise<{ public_url: string }> {
	const formData = new FormData();
	formData.append("file", file);
	formData.append("object_key", objectKey);

	const data = await apiClient.post<{ public_url: string }>({
		url: "/api/v1/storage/upload",
		data: formData,
		headers: { "Content-Type": "multipart/form-data" },
	});

	return data;
}

/** 获取存储配置（管理员） */
export async function getStorageConfig(): Promise<StorageConfigView> {
	const data = await apiClient.post<StorageConfigView>({
		url: "/api/v1/storage/config",
	});

	return data;
}

/** 更新存储配置（管理员） */
export async function updateStorageConfig(
	config: StorageConfigInput,
): Promise<StorageConfigView> {
	const data = await apiClient.post<StorageConfigView>({
		url: "/api/v1/storage/update-config",
		data: config,
	});

	return data;
}

/** 测试存储配置连接，不保存配置 */
export async function testStorageConfig(
	config: StorageConfigInput,
): Promise<{ ok: boolean }> {
	const data = await apiClient.post<{ ok: boolean }>({
		url: "/api/v1/storage/test-config",
		data: config,
	});

	return data ?? { ok: true };
}
