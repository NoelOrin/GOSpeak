import type { AxiosResponse } from 'axios'
import apiClient from './apiClient'
import type { Result } from './apiClient'

export interface LoginReq {
  username: string
  password: string
}

export interface BackendUser {
  id: number
  uuid: string
  name: string
  role: string
}

export interface LoginData {
  access_token: string
  refresh_token: string
  user: BackendUser
  need_change_password: boolean
}

export async function login(req: LoginReq): Promise<LoginData> {
  const res = (await apiClient.post({
    url: '/api/v1/auth/login',
    data: req,
  })) as AxiosResponse<Result<LoginData>>

  const result = res.data
  if (result.code !== 0) throw new Error(result.msg)
  return result.data!
}

export async function refreshToken(refreshToken: string): Promise<string> {
  const res = (await apiClient.post({
    url: '/api/v1/auth/refresh_token',
    data: { refresh_token: refreshToken },
  })) as AxiosResponse<Result<{ access_token: string }>>

  const result = res.data
  if (result.code !== 0) throw new Error(result.msg)
  return result.data!.access_token
}

export async function logout(): Promise<void> {
  await apiClient.post({ url: '/api/v1/auth/logout' })
}

export async function changePassword(oldPassword: string, newPassword: string): Promise<void> {
  const res = (await apiClient.post({
    url: '/api/v1/auth/change_password',
    data: { old_password: oldPassword, new_password: newPassword },
  })) as AxiosResponse<Result>

  const result = res.data
  if (result.code !== 0) throw new Error(result.msg)
}

export async function firstChangePassword(newPassword: string, name?: string): Promise<BackendUser> {
  const res = (await apiClient.post({
    url: '/api/v1/auth/first_change_password',
    data: { new_password: newPassword, name: name || undefined },
  })) as AxiosResponse<Result<BackendUser>>

  const result = res.data
  if (result.code !== 0) throw new Error(result.msg)
  return result.data!
}

export async function resetPassword(username: string, newPassword: string): Promise<void> {
  const res = (await apiClient.post({
    url: '/api/v1/auth/reset_password',
    data: { username, new_password: newPassword },
  })) as AxiosResponse<Result>

  const result = res.data
  if (result.code !== 0) throw new Error(result.msg)
}
