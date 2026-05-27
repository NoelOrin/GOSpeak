import { createSignal } from 'solid-js'
import { get, set, del } from 'idb-keyval'
import { logout as logoutApi } from '@/api/auth'

export interface UserInfo {
  id: number
  uuid: string
  name: string
  role: string
}

const STORAGE_USER = 'user'
const STORAGE_ACCESS_TOKEN = 'accessToken'
const DB_REFRESH_TOKEN = 'refreshToken'

const [user, setUser] = createSignal<UserInfo | null>(null)
const [accessToken, setAccessToken] = createSignal('')
const [refreshToken, setRefreshToken] = createSignal('')

// 同步从 localStorage 恢复 user 和 accessToken，保证路由守卫立即可用
const cachedUser = localStorage.getItem(STORAGE_USER)
const cachedToken = localStorage.getItem(STORAGE_ACCESS_TOKEN)
if (cachedUser) setUser(JSON.parse(cachedUser))
if (cachedToken) setAccessToken(cachedToken)

// refreshToken 在 IndexedDB，异步恢复即可
get<string>(DB_REFRESH_TOKEN).then((rt) => {
  if (rt) setRefreshToken(rt)
})

async function loginAction(u: UserInfo, at: string, rt: string) {
  localStorage.setItem(STORAGE_USER, JSON.stringify(u))
  localStorage.setItem(STORAGE_ACCESS_TOKEN, at)
  await set(DB_REFRESH_TOKEN, rt)
  setUser(u)
  setAccessToken(at)
  setRefreshToken(rt)
}

async function updateAccessTokenAction(token: string) {
  localStorage.setItem(STORAGE_ACCESS_TOKEN, token)
  setAccessToken(token)
}

async function logoutAction() {
  try {
    if (accessToken()) {
      await logoutApi()
    }
  } catch {
    // 即使服务端登出失败，也继续清除本地状态
  }
  localStorage.removeItem(STORAGE_USER)
  localStorage.removeItem(STORAGE_ACCESS_TOKEN)
  await del(DB_REFRESH_TOKEN)
  setUser(null)
  setAccessToken('')
  setRefreshToken('')
}

const userStore = {
  user,
  accessToken,
  refreshToken,
  isLoggedIn: () => !!accessToken(),
  login: loginAction,
  logout: logoutAction,
  updateAccessToken: updateAccessTokenAction,
}

export default userStore
