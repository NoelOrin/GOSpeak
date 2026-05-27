// ============================================================
// 测试套件入口
//   按顺序依次执行各模块测试，最后输出汇总报告
// ============================================================
const { separator } = require('./config')

async function runAll() {
  // 打印测试套件标题头
  console.log()
  console.log('  ╔══════════════════════════════════════════════╗')
  console.log('  ║         GoRTC API 接口测试套件               ║')
  console.log('  ╚══════════════════════════════════════════════╝')
  console.log()
  console.log(`  目标地址: http://localhost:8998/api/v1`)
  console.log(`  开始时间: ${new Date().toLocaleString()}`)

  // 待执行的测试模块列表
  const modules = [
    { name: '🔐 Auth 认证模块', file: './auth/auth.test' },
    { name: '🎥 Signal 信令模块', file: './signal/signal.test' },
    { name: '👤 User 用户管理模块', file: './user/user.test' },
  ]

  const results = []
  // 顺序执行每个模块，捕获异常避免一个模块失败影响后续执行
  for (const mod of modules) {
    console.log(`\n${'━'.repeat(56)}`)
    console.log(`  ▶ 执行: ${mod.name}`)
    console.log(`${'━'.repeat(56)}`)
    try {
      const run = require(mod.file)
      await run()
      results.push({ name: mod.name, status: '✅ 通过' })
    } catch (err) {
      console.error(`  ❌ ${mod.name} 执行失败:`, err.message)
      results.push({ name: mod.name, status: '❌ 失败' })
    }
  }

  // 输出汇总报告：各模块通过/失败统计
  separator('测试汇总')
  let passed = 0
  let failed = 0
  for (const r of results) {
    console.log(`  ${r.status}  ${r.name}`)
    if (r.status.includes('通过')) passed++
    else failed++
  }
  console.log()
  console.log(`  总计: ${results.length} | ✅ 通过: ${passed} | ❌ 失败: ${failed}`)
  console.log(`  结束时间: ${new Date().toLocaleString()}`)
  console.log()
}

runAll().catch(console.error)