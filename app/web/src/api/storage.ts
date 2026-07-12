import type { AxiosResponse } from 'axios'
import apiClient from './apiClient'
import type { Result } from './apiClient'

// ===== 类型定义 =====

export interface StorageConfigView {
  provider_type: 's3' | 'local'
  endpoint: string
  bucket: string
  region: string
  access_key: string   // 脱敏 "AK***XYZ"
  secret_key: string   // 永远为空
  public_base_url: string
  path_prefix: string
  max_file_size: number
  allowed_types: string
}

export interface StorageConfigInput {
  provider_type: 's3' | 'local'
  endpoint?: string
  bucket?: string
  region?: string
  access_key?: string
  secret_key?: string
  public_base_url?: string
  path_prefix?: string
  max_file_size?: number
  allowed_types?: string
}

export interface PresignResult {
  provider_type: 's3' | 'local'
  upload_url: string | null
  object_key: string
  public_url?: string
}

// ===== API 函数 =====

/** 获取预签名上传 URL */
export async function presignUpload(params: {
  file_name: string
  content_type: string
  file_size: number
  category: string
}): Promise<PresignResult> {
  const res = (await apiClient.post({
    url: '/api/v1/storage/presign',
    data: params,
  })) as AxiosResponse<Result<PresignResult>>

  return (res as any).data.data
}

/** 确认 S3 上传完成 */
export async function confirmUpload(objectKey: string): Promise<{ public_url: string }> {
  const res = (await apiClient.post({
    url: '/api/v1/storage/confirm',
    data: { object_key: objectKey },
  })) as AxiosResponse<Result<{ public_url: string }>>

  return (res as any).data.data
}

/** 本地模式中转上传 */
export async function uploadFile(file: File, objectKey: string): Promise<{ public_url: string }> {
  const formData = new FormData()
  formData.append('file', file)
  formData.append('object_key', objectKey)

  const res = (await apiClient.post({
    url: '/api/v1/storage/upload',
    data: formData,
    headers: { 'Content-Type': 'multipart/form-data' },
  })) as AxiosResponse<Result<{ public_url: string }>>

  return (res as any).data.data
}

/** 获取存储配置（管理员） */
export async function getStorageConfig(): Promise<StorageConfigView> {
  const res = (await apiClient.post({
    url: '/api/v1/storage/config',
  })) as AxiosResponse<Result<StorageConfigView>>

  return (res as any).data.data
}

/** 更新存储配置（管理员） */
export async function updateStorageConfig(config: StorageConfigInput): Promise<StorageConfigView> {
  const res = (await apiClient.post({
    url: '/api/v1/storage/update-config',
    data: config,
  })) as AxiosResponse<Result<StorageConfigView>>

  return (res as any).data.data
}
