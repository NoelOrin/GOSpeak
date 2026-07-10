import apiClient from './apiClient'
import type { Result } from './apiClient'

export interface BotAPIKey {
  id: number
  uuid: string
  name: string
  role: string
  user_uuid: string
  revoked: boolean
  expires_at: string
  created_at: string
  updated_at: string
}

export interface CreateBotKeyInput {
  name: string
  role: string
  expires_in?: string
}

export interface CreateBotKeyResult {
  token: string
  token_uuid: string
  user: {
    id: number
    uuid: string
    name: string
    display_name: string
    role: string
  }
  permanent: boolean
  expires_at?: string
}

export function createBotKey(input: CreateBotKeyInput): Promise<Result<CreateBotKeyResult>> {
  return apiClient.post({ url: "/api/v1/bot/create", data: input }).then((r) => r.data)
}

export function listBotKeys(): Promise<Result<BotAPIKey[]>> {
  return apiClient.post({ url: "/api/v1/bot/list" }).then((r) => r.data)
}

export function revokeBotKey(uuid: string): Promise<Result<null>> {
  return apiClient.post({ url: "/api/v1/bot/revoke", data: { uuid } }).then((r) => r.data)
}
