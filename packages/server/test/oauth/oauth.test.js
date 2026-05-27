// ============================================================
// OAuth 管理模块测试
//   覆盖接口：
//     GET    /api/v1/oauth/admin/providers     - 获取 OAuth 提供商列表
//     POST   /api/v1/oauth/admin/providers     - 创建 OAuth 提供商
//     PUT    /api/v1/oauth/admin/providers     - 更新 OAuth 提供商
//     DELETE /api/v1/oauth/admin/providers/:id - 删除 OAuth 提供商
// ============================================================
const { request, log, separator, login, logout } = require('../config')

// ---------- 创建 OAuth 提供商测试 ----------
//   正常创建 → 重复名称 → 缺少参数 → 非管理员
async function testCreateProvider() {
  separator('OAuth /admin/providers (POST)')

  // 先重置管理员密码，确保可以登录
  await request('POST', '/auth/reset_password', {
    body: { username: 'admin', new_password: 'admin123' },
  })

  // 管理员登录
  const adminLogin = await login('admin', 'admin123')
  if (adminLogin.code !== 0) {
    log('管理员登录失败（OAuth测试前置）', adminLogin)
    return
  }

  // 正常创建一个 OAuth 提供商
  const providerName = `test_github_${Date.now()}`
  const r1 = await request('POST', '/oauth/admin/providers', {
    auth: true,
    body: {
      name: providerName,
      client_id: 'test_client_id',
      client_secret: 'test_client_secret',
      auth_url: 'https://github.com/login/oauth/authorize',
      token_url: 'https://github.com/login/oauth/access_token',
      userinfo_url: 'https://api.github.com/user',
      redirect_url: 'http://localhost:8998/api/v1/oauth/callback/github',
      scopes: 'user:email',
      enabled: true,
    },
  })
  log('创建 OAuth 提供商', r1)

  // 重复名称创建（应失败）
  const r2 = await request('POST', '/oauth/admin/providers', {
    auth: true,
    body: {
      name: providerName,
      client_id: 'another_id',
      client_secret: 'another_secret',
      auth_url: 'https://github.com/login/oauth/authorize',
      token_url: 'https://github.com/login/oauth/access_token',
      userinfo_url: 'https://api.github.com/user',
      redirect_url: 'http://localhost:8998/callback',
      scopes: 'user:email',
    },
  })
  log('重复名称创建', r2)

  // 缺少 name 参数
  const r3 = await request('POST', '/oauth/admin/providers', {
    auth: true,
    body: {
      client_id: 'no_name_id',
      client_secret: 'secret',
    },
  })
  log('缺少 name 参数创建', r3)

  // 非管理员创建
  const userName = `oauthuser_${Date.now()}`
  await request('POST', '/auth/register', {
    body: { username: userName, password: '123456' },
  })
  await login(userName, '123456')
  const r4 = await request('POST', '/oauth/admin/providers', {
    auth: true,
    body: {
      name: 'unauthorized_provider',
      client_id: 'x',
      client_secret: 'x',
    },
  })
  log('非管理员创建提供商', r4)

  // 恢复管理员登录
  await login('admin', 'admin123')
}

// ---------- 获取提供商列表测试 ----------
async function testListProviders() {
  separator('OAuth /admin/providers (GET)')

  const r1 = await request('GET', '/oauth/admin/providers', { auth: true })
  log('获取提供商列表', r1)

  // 非管理员访问
  const userName = `oauthlist_${Date.now()}`
  await request('POST', '/auth/register', {
    body: { username: userName, password: '123456' },
  })
  await login(userName, '123456')
  const r2 = await request('GET', '/oauth/admin/providers', { auth: true })
  log('非管理员获取提供商列表', r2)

  // 未登录访问
  await logout()
  const r3 = await request('GET', '/oauth/admin/providers')
  log('未登录获取提供商列表', r3)

  // 恢复管理员登录
  await login('admin', 'admin123')
}

// ---------- 更新 OAuth 提供商测试 ----------
async function testUpdateProvider() {
  separator('OAuth /admin/providers (PUT)')

  // 先获取列表找到已创建的提供商
  const listRes = await request('GET', '/oauth/admin/providers', { auth: true })
  const providers = listRes.data || []
  const testProvider = providers.find(p => p.name && p.name.startsWith('test_github_'))

  if (testProvider) {
    // 更新提供商信息
    const r1 = await request('PUT', '/oauth/admin/providers', {
      auth: true,
      body: {
        id: testProvider.id,
        name: testProvider.name,
        client_id: 'updated_client_id',
        client_secret: 'updated_client_secret',
        auth_url: testProvider.auth_url,
        token_url: testProvider.token_url,
        userinfo_url: testProvider.userinfo_url,
        redirect_url: testProvider.redirect_url,
        scopes: 'user:email,repo',
        enabled: false,
      },
    })
    log('更新 OAuth 提供商', r1)
  } else {
    console.log('  ⚠️  没有找到测试提供商，跳过更新...')
  }

  // 缺少参数更新
  const r2 = await request('PUT', '/oauth/admin/providers', {
    auth: true,
    body: {},
  })
  log('缺少参数更新', r2)
}

// ---------- 删除 OAuth 提供商测试 ----------
async function testDeleteProvider() {
  separator('OAuth /admin/providers/:id (DELETE)')

  // 先获取列表找到已创建的提供商
  const listRes = await request('GET', '/oauth/admin/providers', { auth: true })
  const providers = listRes.data || []
  const testProvider = providers.find(p => p.name && p.name.startsWith('test_github_'))

  if (testProvider) {
    // 删除测试提供商
    const r1 = await request('DELETE', `/oauth/admin/providers/${testProvider.id}`, {
      auth: true,
    })
    log(`删除提供商 ID=${testProvider.id}`, r1)

    // 验证已删除
    if (r1.code === 0) {
      const listAfter = await request('GET', '/oauth/admin/providers', { auth: true })
      const still = (listAfter.data || []).find(p => p.id === testProvider.id)
      log('验证已删除', still ? { code: 1, status: 200, msg: '提供商仍然存在' } : { code: 0, status: 200, msg: '提供商已成功删除' })
    }
  } else {
    console.log('  ⚠️  没有找到测试提供商，跳过删除...')
  }

  // 不存在的 ID
  const r2 = await request('DELETE', '/oauth/admin/providers/99999', {
    auth: true,
  })
  log('删除不存在的提供商', r2)

  // 非管理员删除
  const userName = `oauthdel_${Date.now()}`
  await request('POST', '/auth/register', {
    body: { username: userName, password: '123456' },
  })
  await login(userName, '123456')
  const r3 = await request('DELETE', '/oauth/admin/providers/1', {
    auth: true,
  })
  log('非管理员删除提供商', r3)

  // 未登录删除
  await logout()
  const r4 = await request('DELETE', '/oauth/admin/providers/1')
  log('未登录删除提供商', r4)
}

// ---------- OAuth 公开路由测试 ----------
async function testOAuthPublicRoutes() {
  separator('OAuth 公开路由')

  // 访问不存在的 provider 登录（应返回错误）
  const r1 = await request('GET', '/oauth/login/nonexistent')
  log('不存在的 OAuth provider 登录', { ...r1, code: r1.code, status: r1.status || 200 })

  // 缺少 code 参数的 callback
  const r2 = await request('GET', '/oauth/callback/github')
  log('缺少 code 的 callback', r2)
}

// ---------- 入口 ----------
async function run() {
  console.log('\n🔗 OAuth 管理模块测试\n')

  await testCreateProvider()
  await testListProviders()
  await testUpdateProvider()
  await testDeleteProvider()
  await testOAuthPublicRoutes()

  // 恢复状态
  await logout()

  console.log('\n' + '-'.repeat(56))
}

module.exports = run
