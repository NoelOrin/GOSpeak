// ============================================================
// User 用户管理模块测试
//   覆盖接口：
//     GET    /api/v1/user/profile  - 获取个人信息
//     GET    /api/v1/user/list     - 用户列表（分页）
//     GET    /api/v1/user/{id}     - 用户详情
//     DELETE /api/v1/user/{id}     - 删除用户
// ============================================================
const { request, log, separator, login, logout } = require('../config')

// ---------- 获取个人信息测试 ----------
//   已登录 → 未登录（无 Token）
async function testGetProfile() {
  separator('User /profile')

  // 已登录状态获取个人信息，预期返回当前用户信息
  const r1 = await request('GET', '/user/profile', { auth: true })
  log('获取个人信息', r1)

  // 未携带 Token 访问，预期鉴权失败
  const r2 = await request('GET', '/user/profile')
  log('未登录获取个人信息', r2)
}

// ---------- 用户列表测试 ----------
//   默认分页 → 指定分页参数 → 不存在的页码
async function testList() {
  separator('User /list')

  // 默认分页参数（page=1, page_size 默认值）
  const r1 = await request('GET', '/user/list', { auth: true })
  log('默认分页查询用户列表', r1)

  // 手动指定分页参数
  const r2 = await request('GET', '/user/list?page=1&page_size=5', { auth: true })
  log('指定分页 page=1 page_size=5', r2)

  // 查询页码超出范围，预期返回空列表
  const r3 = await request('GET', '/user/list?page=99&page_size=10', { auth: true })
  log('不存在的页码 page=99', r3)
}

// ---------- 用户详情测试 ----------
//   存在用户 → 不存在用户 → 非法 ID
async function testGetByID() {
  separator('User /:id (GET)')

  // 先获取列表，取第一个有效用户的 ID 查询详情
  const r0 = await request('GET', '/user/list', { auth: true })
  const users = r0.data?.list || []

  if (users.length > 0) {
    const id = users[0].id
    const r1 = await request('GET', `/user/${id}`, { auth: true })
    log(`通过 ID=${id} 查询用户`, r1)
  } else {
    console.log('  ⚠️  没有用户数据，跳过...')
  }

  // 使用不存在的 ID，预期返回 404
  const r2 = await request('GET', '/user/99999', { auth: true })
  log('不存在的用户 ID=99999', r2)

  // 使用非法 ID 格式（非数字），预期参数校验错误
  const r3 = await request('GET', '/user/abc', { auth: true })
  log('非法 ID 格式 abc', r3)
}

// ---------- 删除用户测试 ----------
//   删除一个非管理员用户 → 验证已删除
async function testDelete() {
  separator('User /:id (DELETE)')

  // 先获取列表，取第一个非管理员用户删除（避免删除 admin 影响后续测试）
  const r0 = await request('GET', '/user/list', { auth: true })
  const users = r0.data?.list || []
  const deletableUser = users.find(u => u.role !== 'admin')

  if (deletableUser) {
    const id = deletableUser.id
    // 执行删除操作
    const r1 = await request('DELETE', `/user/${id}`, { auth: true })
    log(`删除用户 ${deletableUser.name} (ID=${id})`, r1)

    // 验证删除结果：再次查询该用户应返回 user not found
    const r2 = await request('GET', `/user/${id}`, { auth: true })
    log(`验证用户 ID=${id} 已删除`, r2)
  } else {
    console.log('  ⚠️  没有可删除的非管理员用户，跳过删除...')
  }
}

// ---------- 入口 ----------
//   按顺序执行所有 User 测试用例
async function run() {
  console.log('\n👤 User 用户管理模块测试\n')

  // 登录管理员账号以获取鉴权 Token
  const loginRes = await login('admin', 'admin123')
  if (loginRes.code !== 0) {
    console.log('  ⚠️  管理员登录失败，用户模块测试跳过')
    console.log('\n' + '-'.repeat(56))
    return
  }

  await testGetProfile()
  await testList()
  await testGetByID()
  await testDelete()

  await logout()

  console.log('\n' + '-'.repeat(56))
}

module.exports = run