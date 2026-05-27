// ============================================================
// 用户角色管理模块测试
//   覆盖接口：
//     PUT /api/v1/user/:id/role - 更新用户角色（管理员）
// ============================================================
const { request, log, separator, login, logout } = require('../config')

// ---------- 更新用户角色测试 ----------
//   管理员更新角色 → 非管理员更新 → 不存在用户 → 缺少参数 → 未登录
async function testUpdateRole() {
  separator('User /:id/role (PUT)')

  // 先重置管理员密码，确保可以登录
  await request('POST', '/auth/reset_password', {
    body: { username: 'admin', new_password: 'admin123' },
  })

  // 以管理员身份登录
  const adminLogin = await login('admin', 'admin123')
  if (adminLogin.code !== 0) {
    log('管理员登录失败（角色测试前置）', adminLogin)
    return
  }

  // 获取用户列表，取第一个非管理员用户
  const r0 = await request('GET', '/user/list', { auth: true })
  const users = r0.data?.list || []
  const normalUser = users.find(u => u.role !== 'admin' && u.name !== 'admin')

  if (normalUser) {
    // 管理员更新用户角色为 admin
    const r1 = await request('PUT', `/user/${normalUser.id}/role`, {
      auth: true,
      body: { role: 'admin' },
    })
    log(`管理员将用户 ${normalUser.name} 设为 admin`, r1)

    // 恢复为 user 角色
    if (r1.code === 0) {
      const r1b = await request('PUT', `/user/${normalUser.id}/role`, {
        auth: true,
        body: { role: 'user' },
      })
      log(`恢复用户 ${normalUser.name} 为 user`, r1b)
    }
  } else {
    console.log('  ⚠️  没有普通用户，跳过角色更新...')
  }

  // 缺少 role 参数
  const r2 = await request('PUT', `/user/1/role`, {
    auth: true,
    body: {},
  })
  log('缺少 role 参数', r2)

  // 不存在的用户 ID
  const r3 = await request('PUT', '/user/99999/role', {
    auth: true,
    body: { role: 'admin' },
  })
  log('不存在用户更新角色', r3)

  // 非管理员用户尝试更新角色
  const userName = `roleuser_${Date.now()}`
  await request('POST', '/auth/register', {
    body: { username: userName, password: '123456' },
  })
  await login(userName, '123456')
  const r4 = await request('PUT', `/user/1/role`, {
    auth: true,
    body: { role: 'admin' },
  })
  log('非管理员更新角色', r4)

  // 未登录
  await logout()
  const r5 = await request('PUT', `/user/1/role`, {
    body: { role: 'admin' },
  })
  log('未登录更新角色', r5)
}

// ---------- 入口 ----------
async function run() {
  console.log('\n👤 用户角色管理模块测试\n')

  await testUpdateRole()

  // 恢复管理员登录状态
  await login('admin', 'admin123')
  await logout()

  console.log('\n' + '-'.repeat(56))
}

module.exports = run
