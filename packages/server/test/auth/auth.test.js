// ============================================================
// Auth 认证模块测试
//   覆盖接口：
//     POST /api/v1/auth/register       - 用户注册
//     POST /api/v1/auth/login          - 用户登录
//     POST /api/v1/auth/refresh_token  - 刷新 Token
//     POST /api/v1/auth/logout         - 退出登录
//     POST /api/v1/auth/refresh        - 刷新当前 Token
// ============================================================
const { request, log, separator, setTokens } = require('../config')

// ---------- 注册测试 ----------
//   正常注册 → 重复注册 → 空字段注册
async function testRegister() {
  separator('Auth /register')

  // 正常注册新用户
  const r1 = await request('POST', '/auth/register', {
    body: { username: 'testuser', password: '123456' },
  })
  log('注册新用户', r1)

  // 注册成功后自动保存 token，供后续带鉴权测试使用
  if (r1.code === 0) {
    setTokens(r1.data.access_token, r1.data.refresh_token)
  }

  // 重复注册同一用户名，预期返回用户名已存在
  const r2 = await request('POST', '/auth/register', {
    body: { username: 'testuser', password: '123456' },
  })
  log('重复注册同一用户', r2)

  // 空字段注册，预期返回参数校验错误
  const r3 = await request('POST', '/auth/register', {
    body: { username: '', password: '' },
  })
  log('空字段注册', r3)
}

// ---------- 登录测试 ----------
//   正确密码 → 错误密码 → 不存在用户
async function testLogin() {
  separator('Auth /login')

  // 正确用户名和密码，预期登录成功并返回 token
  const r1 = await request('POST', '/auth/login', {
    body: { username: 'testuser', password: '123456' },
  })
  log('正确账号密码登录', r1)

  // 登录成功后保存 token 供后续鉴权测试使用
  if (r1.code === 0) {
    setTokens(r1.data.access_token, r1.data.refresh_token)
    console.log(`       token: ${r1.data.access_token.slice(0, 40)}...`)
    console.log(`       refresh_token: ${r1.data.refresh_token.slice(0, 40)}...`)
  }

  // 错误密码，预期提示密码错误
  const r2 = await request('POST', '/auth/login', {
    body: { username: 'testuser', password: 'wrong' },
  })
  log('错误密码登录', r2)

  // 不存在的用户名，预期提示用户不存在
  const r3 = await request('POST', '/auth/login', {
    body: { username: 'nonexistent', password: '123456' },
  })
  log('不存在的用户登录', r3)
}

// ---------- 刷新 Token 测试 ----------
//   有效 refresh_token → 无效 refresh_token
async function testRefreshToken() {
  separator('Auth /refresh_token')

  // 使用当前保存的 refresh_token 获取新的 access token
  const r1 = await request('POST', '/auth/refresh_token', {
    body: { refresh_token: require('../config').getRefreshToken() },
  })
  log('刷新 Token', r1)

  // 使用无效的 refresh_token，预期鉴权失败
  const r2 = await request('POST', '/auth/refresh_token', {
    body: { refresh_token: 'invalid-token' },
  })
  log('无效 refresh_token', r2)
}

// ---------- 退出登录测试 ----------
//   需携带 Bearer Token，预期清空/失效当前 token
async function testLogout() {
  separator('Auth /logout')

  const r1 = await request('POST', '/auth/logout', { auth: true })
  log('退出登录', r1)
}

// ---------- 刷新当前 Token 测试 ----------
//   需携带 Bearer Token，预期返回新的 access token
async function testRefresh() {
  separator('Auth /refresh')

  const r1 = await request('POST', '/auth/refresh', { auth: true })
  log('刷新当前 Token', r1)
}

// ---------- 入口 ----------
//   按顺序执行所有 Auth 测试用例
async function run() {
  console.log('\n🔐 Auth 认证模块测试\n')

  await testRegister()
  await testLogin()
  await testRefreshToken()
  await testLogout()

  // logout 后 token 被清空，如果有 token 才执行 refresh 测试
  const { getToken } = require('../config')
  if (getToken()) {
    await testRefresh()
  }

  console.log('\n' + '-'.repeat(56))
}

module.exports = run