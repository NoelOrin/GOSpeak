// ============================================================
// 密码管理模块测试
//   覆盖接口：
//     POST /api/v1/auth/change_password       - 修改密码（需旧密码）
//     POST /api/v1/auth/first_change_password - 首次登录修改密码（管理员）
//     POST /api/v1/auth/reset_password        - 重置密码（忘记密码）
// ============================================================
const { request, log, separator, login, logout } = require('../config')

// ---------- 修改密码测试 ----------
//   正确旧密码 → 错误旧密码 → 缺少参数 → 未登录
async function testChangePassword() {
  separator('Auth /change_password')

  // 先注册并登录一个测试用户
  const uniqueName = `pwduser_${Date.now()}`
  await request('POST', '/auth/register', {
    body: { username: uniqueName, password: '123456' },
  })
  const loginRes = await login(uniqueName, '123456')
  if (loginRes.code !== 0) {
    log('登录测试用户失败（修改密码前置）', loginRes)
    return
  }

  // 正确旧密码修改密码
  const r1 = await request('POST', '/auth/change_password', {
    auth: true,
    body: { old_password: '123456', new_password: 'newpass789' },
  })
  log('正确旧密码修改密码', r1)

  // 用新密码登录验证修改成功
  if (r1.code === 0) {
    await logout()
    const r1Login = await login(uniqueName, 'newpass789')
    log('新密码登录验证', r1Login)
  }

  // 错误旧密码
  const r2 = await request('POST', '/auth/change_password', {
    auth: true,
    body: { old_password: 'wrongpass', new_password: 'another123' },
  })
  log('错误旧密码修改', r2)

  // 缺少参数
  const r3 = await request('POST', '/auth/change_password', {
    auth: true,
    body: {},
  })
  log('缺少参数修改密码', r3)

  // 未登录访问
  await logout()
  const r4 = await request('POST', '/auth/change_password', {
    body: { old_password: '123456', new_password: 'newpass789' },
  })
  log('未登录修改密码', r4)
}

// ---------- 首次登录修改密码测试 ----------
//   管理员默认密码修改 → 非管理员 → 非默认密码
async function testFirstChangePassword() {
  separator('Auth /first_change_password')

  // 先将管理员密码重置为默认值，确保 first_change_password 前置条件成立
  await request('POST', '/auth/reset_password', {
    body: { username: 'admin', new_password: 'admin123' },
  })

  // 用管理员默认密码登录
  const loginRes = await login('admin', 'admin123')
  if (loginRes.code !== 0) {
    log('管理员登录失败（首次改密前置）', loginRes)
    return
  }

  // 管理员首次修改密码（带改名）
  const r1 = await request('POST', '/auth/first_change_password', {
    auth: true,
    body: { new_password: 'admin_new_pass', name: 'admin' },
  })
  log('管理员首次修改密码（带同名）', r1)

  // 恢复默认密码，方便后续测试
  if (r1.code === 0) {
    await login('admin', 'admin_new_pass')
    await request('POST', '/auth/change_password', {
      auth: true,
      body: { old_password: 'admin_new_pass', new_password: 'admin123' },
    })
  }

  // 注册一个普通用户尝试首次改密（应失败：仅管理员）
  const userName = `firstpwd_${Date.now()}`
  await request('POST', '/auth/register', {
    body: { username: userName, password: '123456' },
  })
  await login(userName, '123456')
  const r2 = await request('POST', '/auth/first_change_password', {
    auth: true,
    body: { new_password: 'newpass123' },
  })
  log('普通用户尝试首次改密', r2)

  // 缺少参数
  await login('admin', 'admin123')
  const r3 = await request('POST', '/auth/first_change_password', {
    auth: true,
    body: {},
  })
  log('缺少参数首次改密', r3)

  // 未登录
  await logout()
  const r4 = await request('POST', '/auth/first_change_password', {
    body: { new_password: 'newpass123' },
  })
  log('未登录首次改密', r4)
}

// ---------- 重置密码测试 ----------
//   存在用户重置 → 不存在用户 → 缺少参数
async function testResetPassword() {
  separator('Auth /reset_password')

  // 创建一个用户用于重置
  const userName = `resetuser_${Date.now()}`
  await request('POST', '/auth/register', {
    body: { username: userName, password: 'origpass123' },
  })

  // 重置密码
  const r1 = await request('POST', '/auth/reset_password', {
    body: { username: userName, new_password: 'resetpass456' },
  })
  log('重置密码', r1)

  // 用新密码登录验证
  if (r1.code === 0) {
    const loginRes = await login(userName, 'resetpass456')
    log('新密码登录验证', loginRes)
    await logout()
  }

  // 不存在的用户
  const r2 = await request('POST', '/auth/reset_password', {
    body: { username: 'nonexistent_user_xyz', new_password: 'whatever' },
  })
  log('不存在用户重置密码', r2)

  // 缺少参数
  const r3 = await request('POST', '/auth/reset_password', {
    body: {},
  })
  log('缺少参数重置密码', r3)
}

// ---------- 入口 ----------
async function run() {
  console.log('\n🔑 密码管理模块测试\n')

  await testChangePassword()
  await testFirstChangePassword()
  await testResetPassword()

  // 恢复管理员默认状态
  await logout()

  console.log('\n' + '-'.repeat(56))
}

module.exports = run
