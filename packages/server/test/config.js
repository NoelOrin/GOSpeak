// ============================================================
// 测试工具库 - 提供统一的请求封装、日志输出和 Token 管理
// ============================================================

// API 基础地址，所有请求都会拼接此前缀
const BASE_URL = 'http://localhost:8998/api/v1'

// 全局 Token 存储（登录成功后由 setTokens 写入）
let token = ''
let refreshToken = ''

// 保存登录成功后获取的 token 和 refresh_token
function setTokens(t, rt) {
  token = t
  refreshToken = rt
}

// 获取当前 access token
function getToken() {
  return token
}

// 获取当前 refresh token
function getRefreshToken() {
  return refreshToken
}

// 通用 HTTP 请求封装
//   method: 请求方法 (GET/POST/DELETE 等)
//   path:   接口路径（自动拼接 BASE_URL）
//   options.body  - 请求体（对象自动 JSON 序列化）
//   options.auth  - 为 true 时自动带上 Authorization 头
//   options.headers - 自定义请求头
async function request(method, path, options = {}) {
  const url = `${BASE_URL}${path}`
  const headers = { 'Content-Type': 'application/json', ...options.headers }

  // 如果需要鉴权且 token 存在，自动注入 Bearer Token
  if (options.auth && token) {
    headers['Authorization'] = `Bearer ${token}`
  }

  const res = await fetch(url, {
    method,
    headers,
    body: options.body ? JSON.stringify(options.body) : undefined,
  })

  const data = await res.json()
  return { status: res.status, ...data }
}

// 格式化输出测试结果
//   根据业务 code 显示 ✅ 或 ❌，附带状态码和提示信息
//   data 过长时截断显示
function log(desc, result) {
  const icon = result.code === 0 ? '✅' : '❌'
  console.log(`  ${icon} [${result.status}] ${desc}: ${result.msg}`)
  if (result.data) {
    const str = JSON.stringify(result.data, null, 4)
    if (str.length > 120) {
      console.log(`       数据: ${str.slice(0, 120)}...`)
    } else {
      console.log(`       数据: ${str}`)
    }
  }
}

// 打印分隔线，区分不同测试模块/用例
function separator(title) {
  console.log(`\n${'='.repeat(56)}`)
  console.log(`  ${title}`)
  console.log(`${'='.repeat(56)}`)
}

// 导出所有公共方法供各测试模块使用
module.exports = { request, log, separator, setTokens, getToken, getRefreshToken, BASE_URL }