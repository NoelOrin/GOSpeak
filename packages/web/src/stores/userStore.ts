import { createSignal } from 'solid-js'
import { get, set, del } from 'idb-keyval'

export interface UserInfo {
  id: number
  uuid: string
  name: string
  role: string
}

const [user, setUser] = createSignal<UserInfo | null>(null)
const [accessToken, setAccessToken] = createSignal('')
const [refreshToken, setRefreshToken] = createSignal('')

;(async () => {
  const [u, at, rt] = await Promise.all([
    get<UserInfo>('user'),
    get<string>('accessToken'),
    get<string>('refreshToken'),
  ])
  if (u) setUser(u)
  if (at) setAccessToken(at)
  if (rt) setRefreshToken(rt)
})()

async function loginAction(u: UserInfo, at: string, rt: string) {
  await Promise.all([set('user', u), set('accessToken', at), set('refreshToken', rt)])
  setUser(u)
  setAccessToken(at)
  setRefreshToken(rt)
}

async function logoutAction() {
  await Promise.all([del('user'), del('accessToken'), del('refreshToken')])
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
}

export default userStore
