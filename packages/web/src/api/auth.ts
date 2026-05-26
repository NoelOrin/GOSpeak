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
