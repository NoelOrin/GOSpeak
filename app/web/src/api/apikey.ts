import apiClient from './apiClient'
import type { Result } from './apiClient'

export interface BotAPIKey {
  id: number
  uuid: string
  name: string
  permissions: string[]
  created_by: string
  expires_at: string
  last_used_at: string | null
  revoked: boolean
  created_at: string
}

export interface CreateBotKeyInput {
  name: string
  permissions: string[]
  expires_in?: string
}

export interface CreateBotKeyResult {
  key: BotAPIKey
  plain_key: string
}

// 可授予 Bot 的权限清单（不含 admin 专属权限）。
export const BOT_PERMISSION_OPTIONS: { code: string; label: string }[] = [
  { code: 'room:read', label: '查看房间' },
  { code: 'room:update', label: '编辑房间' },
  { code: 'user:read', label: '查看用户' },
  { code: 'signal:kick', label: '踢出房间' },
]

export function createBotKey(input: CreateBotKeyInput): Promise<Result<CreateBotKeyResult>> {
  return apiClient.post({ url: "/bot/key/create", data: input }).then((r) => r.data)
}

export function listBotKeys(): Promise<Result<BotAPIKey[]>> {
  return apiClient.post({ url: "/bot/key/list" }).then((r) => r.data)
}

export function revokeBotKey(uuid: string): Promise<Result<null>> {
  return apiClient.post({ url: "/bot/key/revoke", data: { uuid } }).then((r) => r.data)
}
