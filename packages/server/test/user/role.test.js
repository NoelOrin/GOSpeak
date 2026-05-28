// ============================================================
// 角色管理模块测试
//   覆盖接口：
//     GET    /api/v1/role/list     - 获取角色列表
//     POST   /api/v1/role/create   - 创建角色（管理员）
//     DELETE /api/v1/role/:id      - 删除角色（管理员）
//     PUT    /api/v1/user/:id/role - 更新用户角色（管理员）
// ============================================================
const { request, separator, login, logout } = require('../config')

let passCount = 0
let failCount = 0

// 断言：期望 code === 0
function assertSuccess(desc, result) {
  if (result.code === 0) {
    console.log(`  ✅ ${desc}`)
    passCount++
  } else {
    console.log(`  ❌ ${desc} — 期望成功，实际: [${result.status}] ${result.msg}`)
    failCount++
  }
}

// 断言：期望 code !== 0（负面测试）
function assertFail(desc, result, expectedStatus) {
  if (result.code !== 0) {
    const statusMatch = !expectedStatus || result.status === expectedStatus
    if (statusMatch) {
      console.log(`  ✅ ${desc} → [${result.status}] ${result.msg}`)
      passCount++
    } else {
      console.log(`  ❌ ${desc} — 期望 ${expectedStatus}，实际: [${result.status}] ${result.msg}`)
      failCount++
    }
  } else {
    console.log(`  ❌ ${desc} — 期望失败，实际成功`)
    failCount++
  }
}

// ---------- 获取角色列表测试 ----------
async function testListRoles() {
  separator('Role /list (GET)')

  await request('POST', '/auth/reset_password', {
    body: { username: 'admin', new_password: 'admin123' },
  })
  const adminLogin = await login('admin', 'admin123')
  if (adminLogin.code !== 0) {
    console.log('  ❌ 管理员登录失败，跳过')
    failCount++
    return
  }

  // 管理员获取角色列表
  const r1 = await request('GET', '/role/list', { auth: true })
  assertSuccess('管理员获取角色列表', r1)
  if (r1.data?.length > 0) {
    console.log(`       角色数: ${r1.data.length}, 包含: ${r1.data.map(r => r.name).join(', ')}`)
  }

  // 普通用户获取角色列表
  const userName = `rolelist_${Date.now()}`
  await request('POST', '/auth/register', {
    body: { username: userName, password: '123456' },
  })
  await login(userName, '123456')
  const r2 = await request('GET', '/role/list', { auth: true })
  assertSuccess('普通用户获取角色列表', r2)

  // 未登录获取角色列表
  await logout()
  const r3 = await request('GET', '/role/list')
  assertFail('未登录获取角色列表（应拒绝）', r3, 401)

  await login('admin', 'admin123')
}

// ---------- 创建角色测试 ----------
async function testCreateRole() {
  separator('Role /create (POST)')

  await request('POST', '/auth/reset_password', {
    body: { username: 'admin', new_password: 'admin123' },
  })
  const adminLogin = await login('admin', 'admin123')
  if (adminLogin.code !== 0) {
    console.log('  ❌ 管理员登录失败，跳过')
    failCount++
    return
  }

  // 创建新角色
  const roleName = `testrole_${Date.now()}`
  const r1 = await request('POST', '/role/create', {
    auth: true,
    body: { name: roleName },
  })
  assertSuccess(`创建角色 "${roleName}"`, r1)

  // 重复创建同名角色
  const r2 = await request('POST', '/role/create', {
    auth: true,
    body: { name: roleName },
  })
  assertFail('重复创建同名角色（应失败）', r2)

  // 缺少 name 参数
  const r3 = await request('POST', '/role/create', {
    auth: true,
    body: {},
  })
  assertFail('缺少 name 参数（应 400）', r3, 400)

  // 空名称
  const r4 = await request('POST', '/role/create', {
    auth: true,
    body: { name: '' },
  })
  assertFail('空名称创建角色（应 400）', r4, 400)

  // 非管理员创建角色
  const userName = `rolecreate_${Date.now()}`
  await request('POST', '/auth/register', {
    body: { username: userName, password: '123456' },
  })
  await login(userName, '123456')
  const r5 = await request('POST', '/role/create', {
    auth: true,
    body: { name: 'hackrole' },
  })
  assertFail('非管理员创建角色（应 403）', r5, 403)

  // 未登录创建角色
  await logout()
  const r6 = await request('POST', '/role/create', {
    body: { name: 'hackrole2' },
  })
  assertFail('未登录创建角色（应 401）', r6, 401)

  await login('admin', 'admin123')
}

// ---------- 删除角色测试 ----------
async function testDeleteRole() {
  separator('Role /:id (DELETE)')

  await request('POST', '/auth/reset_password', {
    body: { username: 'admin', new_password: 'admin123' },
  })
  const adminLogin = await login('admin', 'admin123')
  if (adminLogin.code !== 0) {
    console.log('  ❌ 管理员登录失败，跳过')
    failCount++
    return
  }

  // 创建待删除角色
  const roleName = `delrole_${Date.now()}`
  const createRes = await request('POST', '/role/create', {
    auth: true,
    body: { name: roleName },
  })
  if (createRes.code !== 0) {
    console.log('  ❌ 创建待删除角色失败，跳过')
    failCount++
    return
  }
  const roleId = createRes.data?.id
  assertSuccess(`创建待删除角色 "${roleName}" (id=${roleId})`, createRes)

  // 管理员删除角色
  const r1 = await request('DELETE', `/role/${roleId}`, { auth: true })
  assertSuccess(`管理员删除角色 id=${roleId}`, r1)

  // 删除不存在的角色
  const r2 = await request('DELETE', '/role/99999', { auth: true })
  assertFail('删除不存在的角色（应失败）', r2)

  // 无效 ID
  const r3 = await request('DELETE', '/role/abc', { auth: true })
  assertFail('无效 ID 删除角色（应 400）', r3, 400)

  // 非管理员删除角色
  const roleName2 = `delrole2_${Date.now()}`
  const createRes2 = await request('POST', '/role/create', {
    auth: true,
    body: { name: roleName2 },
  })
  const roleId2 = createRes2.data?.id

  const userName = `roledelete_${Date.now()}`
  await request('POST', '/auth/register', {
    body: { username: userName, password: '123456' },
  })
  await login(userName, '123456')
  const r4 = await request('DELETE', `/role/${roleId2}`, { auth: true })
  assertFail('非管理员删除角色（应 403）', r4, 403)

  // 未登录删除角色
  await logout()
  const r5 = await request('DELETE', `/role/${roleId2}`)
  assertFail('未登录删除角色（应 401）', r5, 401)

  // 清理
  await login('admin', 'admin123')
  if (roleId2) {
    await request('DELETE', `/role/${roleId2}`, { auth: true })
  }
}

// ---------- 更新用户角色测试 ----------
async function testUpdateRole() {
  separator('User /:id/role (PUT)')

  await request('POST', '/auth/reset_password', {
    body: { username: 'admin', new_password: 'admin123' },
  })
  const adminLogin = await login('admin', 'admin123')
  if (adminLogin.code !== 0) {
    console.log('  ❌ 管理员登录失败，跳过')
    failCount++
    return
  }

  // 获取普通用户
  const r0 = await request('GET', '/user/list', { auth: true })
  const users = r0.data?.list || []
  const normalUser = users.find(u => u.role !== 'admin' && u.name !== 'admin')

  if (normalUser) {
    // 管理员更新用户角色为 admin
    const r1 = await request('PUT', `/user/${normalUser.id}/role`, {
      auth: true,
      body: { role: 'admin' },
    })
    assertSuccess(`管理员将用户 ${normalUser.name} 设为 admin`, r1)

    // 恢复为 user
    if (r1.code === 0) {
      const r1b = await request('PUT', `/user/${normalUser.id}/role`, {
        auth: true,
        body: { role: 'user' },
      })
      assertSuccess(`恢复用户 ${normalUser.name} 为 user`, r1b)
    }
  } else {
    console.log('  ⚠️  没有普通用户，跳过角色更新')
  }

  // 缺少 role 参数
  const r2 = await request('PUT', '/user/1/role', {
    auth: true,
    body: {},
  })
  assertFail('缺少 role 参数（应 400）', r2, 400)

  // 不存在的用户
  const r3 = await request('PUT', '/user/99999/role', {
    auth: true,
    body: { role: 'admin' },
  })
  assertFail('不存在用户更新角色（应 404）', r3, 404)

  // 非管理员更新角色
  const userName = `roleuser_${Date.now()}`
  await request('POST', '/auth/register', {
    body: { username: userName, password: '123456' },
  })
  await login(userName, '123456')
  const r4 = await request('PUT', '/user/1/role', {
    auth: true,
    body: { role: 'admin' },
  })
  assertFail('非管理员更新角色（应 403）', r4, 403)

  // 未登录
  await logout()
  const r5 = await request('PUT', '/user/1/role', {
    body: { role: 'admin' },
  })
  assertFail('未登录更新角色（应 401）', r5, 401)
}

// ---------- 入口 ----------
async function run() {
  console.log('\n👤 角色管理模块测试\n')
  passCount = 0
  failCount = 0

  await testListRoles()
  await testCreateRole()
  await testDeleteRole()
  await testUpdateRole()

  // 恢复管理员登录状态
  await login('admin', 'admin123')
  await logout()

  console.log('\n' + '='.repeat(56))
  console.log(`  角色管理测试汇总: ✅ ${passCount} 通过 | ❌ ${failCount} 失败`)
  console.log('='.repeat(56))
}

module.exports = run
