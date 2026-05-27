// ============================================================
// Socket.IO WebSocket 集成测试
//   覆盖事件：
//     connection / disconnect          - 连接/断开
//     room:create                      - 创建房间
//     room:join                        - 加入房间
//     room:leave                       - 离开房间
//     room:list                        - 房间列表
// ============================================================
const http = require('http')
const { separator } = require('../config')

const BASE = 'http://localhost:8998'
const EIO_PACKET = { OPEN: '0', CLOSE: '1', PING: '2', PONG: '3', MESSAGE: '4' }
const SIO_PACKET = { CONNECT: '0', DISCONNECT: '1', EVENT: '2', ACK: '3', CONNECT_ERROR: '4' }

// ---- 底层 Engine.IO 轮询工具 ----

// 通过 HTTP 轮询获取 Engine.IO session（sid + 配置）
function pollGet(sid) {
  return new Promise((resolve, reject) => {
    const url = sid
      ? `/socket.io/?EIO=4&transport=polling&sid=${sid}`
      : `/socket.io/?EIO=4&transport=polling`
    http.get(`${BASE}${url}`, (res) => {
      let body = ''
      res.on('data', (d) => body += d)
      res.on('end', () => resolve(body))
    }).on('error', reject)
  })
}

// 通过 HTTP 轮询发送 Engine.IO 数据
function pollPost(sid, data) {
  return new Promise((resolve, reject) => {
    const url = `/socket.io/?EIO=4&transport=polling&sid=${sid}`
    const payload = `4${data}`
    const options = {
      hostname: 'localhost', port: 8998,
      path: url, method: 'POST',
      headers: { 'Content-Type': 'text/plain;charset=UTF-8', 'Content-Length': Buffer.byteLength(payload) },
    }
    const req = http.request(options, (res) => {
      let body = ''
      res.on('data', (d) => body += d)
      res.on('end', () => resolve(body))
    })
    req.on('error', reject)
    req.write(payload)
    req.end()
  })
}

// 解析 Engine.IO 轮询响应中的 Socket.IO 消息
function parsePollResponse(body) {
  const messages = []
  // 响应格式：长度（十进制或十六进制）:EIO包类型+数据
  // 简单解析：找到所有 "4" 开头的 Socket.IO 消息
  let i = 0
  while (i < body.length) {
    // 跳过长度前缀和分隔符
    const colonIdx = body.indexOf(':', i)
    if (colonIdx === -1) break
    const len = parseInt(body.substring(i, colonIdx), 10)
    if (isNaN(len)) { i++; continue }
    const chunk = body.substring(colonIdx + 1, colonIdx + 1 + len)
    // Engine.IO packet type 是第一个字符
    if (chunk.startsWith('4')) {
      messages.push(chunk.substring(1)) // 去掉 '4' 前缀
    }
    i = colonIdx + 1 + len
  }
  return messages
}

// 发送 Socket.IO 事件（通过轮询）
async function emitEvent(sid, eventName, data) {
  const sioPayload = `${SIO_PACKET.EVENT}["${eventName}",${JSON.stringify(data)}]`
  return pollPost(sid, sioPayload)
}

// 等待并获取轮询消息
async function waitForMessages(sid, timeout = 2000) {
  const start = Date.now()
  while (Date.now() - start < timeout) {
    const body = await pollGet(sid)
    const msgs = parsePollResponse(body)
    if (msgs.length > 0) return msgs
    await sleep(200)
  }
  return []
}

function sleep(ms) { return new Promise(r => setTimeout(r, ms)) }

function log(desc, ok, detail) {
  const icon = ok ? '✅' : '❌'
  console.log(`  ${icon} ${desc}${detail ? ': ' + detail : ''}`)
}

// ---- 测试用例 ----

async function testConnection() {
  separator('Socket.IO 连接测试')

  // 1. 建立轮询 session
  const initBody = await pollGet()
  const openMatch = initBody.match(/"sid":"([^"]+)"/)
  const sid = openMatch ? openMatch[1] : null
  log('建立 Engine.IO session', !!sid, sid ? `sid=${sid}` : '无响应')

  if (!sid) return null

  // 2. 获取 Socket.IO CONNECT 确认
  const msgs = await waitForMessages(sid, 3000)
  const hasConnect = msgs.some(m => m.startsWith(SIO_PACKET.CONNECT))
  log('收到 Socket.IO CONNECT', hasConnect, `消息数: ${msgs.length}`)

  return sid
}

async function testRoomCreate(sid) {
  separator('Socket.IO room:create 测试')

  // 创建房间
  await emitEvent(sid, 'room:create', { room: 'ws-test-room', identity: 'tester-1' })
  await sleep(500)

  const msgs = await waitForMessages(sid, 3000)
  const created = msgs.find(m => m.includes('room:created'))
  log('收到 room:created 事件', !!created, created ? created.slice(0, 80) : '无响应')

  // 创建同名房间（应返回已存在错误）
  await emitEvent(sid, 'room:create', { room: 'ws-test-room', identity: 'tester-2' })
  await sleep(300)
  const msgs2 = await waitForMessages(sid, 3000)
  const duplicate = msgs2.find(m => m.includes('room:created') && m.includes('already exists'))
  log('重复创建房间返回错误', !!duplicate, duplicate ? duplicate.slice(0, 80) : '无响应')

  // 空房间名
  await emitEvent(sid, 'room:create', { room: '', identity: 'tester' })
  await sleep(300)
  const msgs3 = await waitForMessages(sid, 3000)
  const emptyErr = msgs3.find(m => m.includes('room name is required'))
  log('空房间名返回错误', !!emptyErr, emptyErr ? emptyErr.slice(0, 80) : '无响应')
}

async function testRoomJoin(sid) {
  separator('Socket.IO room:join 测试')

  // 加入已创建的房间
  await emitEvent(sid, 'room:join', { room: 'ws-test-room', identity: 'player-1' })
  await sleep(500)

  const msgs = await waitForMessages(sid, 3000)
  const joined = msgs.find(m => m.includes('room:joined'))
  log('收到 room:joined 事件', !!joined, joined ? joined.slice(0, 100) : '无响应')

  // 空房间名加入
  await emitEvent(sid, 'room:join', { room: '', identity: 'player' })
  await sleep(300)
  const msgs2 = await waitForMessages(sid, 3000)
  const emptyErr = msgs2.find(m => m.includes('room name is required'))
  log('空房间名加入返回错误', !!emptyErr)
}

async function testRoomList(sid) {
  separator('Socket.IO room:list 测试')

  await emitEvent(sid, 'room:list', {})
  await sleep(500)

  const msgs = await waitForMessages(sid, 3000)
  const listResult = msgs.find(m => m.includes('room:list:result'))
  log('收到 room:list:result 事件', !!listResult, listResult ? listResult.slice(0, 100) : '无响应')
}

async function testRoomLeave(sid) {
  separator('Socket.IO room:leave 测试')

  // 离开已加入的房间
  await emitEvent(sid, 'room:leave', { room: 'ws-test-room' })
  await sleep(500)

  const msgs = await waitForMessages(sid, 3000)
  const left = msgs.find(m => m.includes('room:left'))
  log('收到 room:left 事件', !!left, left ? left.slice(0, 80) : '无响应')

  // 空房间名离开
  await emitEvent(sid, 'room:leave', { room: '' })
  await sleep(300)
  const msgs2 = await waitForMessages(sid, 3000)
  const emptyErr = msgs2.find(m => m.includes('room name is required'))
  log('空房间名离开返回错误', !!emptyErr)
}

async function testDisconnect(sid) {
  separator('Socket.IO 断开测试')

  // 发送 Engine.IO CLOSE 包
  await pollPost(sid, '1')
  log('发送断开请求', true)

  // 验证 session 已关闭（再次轮询应失败或返回空）
  await sleep(500)
  try {
    const body = await pollGet(sid)
    const isClosed = body.length === 0 || body.includes('"errors"')
    log('Session 已关闭', isClosed || body.length < 10, `响应长度: ${body.length}`)
  } catch (e) {
    log('Session 已关闭', true, '连接被拒')
  }
}

async function testMultipleClients() {
  separator('Socket.IO 多客户端测试')

  // 客户端 A 创建房间
  const bodyA = await pollGet()
  const sidA = (bodyA.match(/"sid":"([^"]+)"/) || [])[1]
  if (!sidA) { log('客户端 A 连接', false); return }
  await waitForMessages(sidA, 3000) // 等 CONNECT

  await emitEvent(sidA, 'room:create', { room: 'multi-test-room', identity: 'user-A' })
  await sleep(500)
  await waitForMessages(sidA, 2000) // 消费 room:created

  // 客户端 B 连接并加入同一房间
  const bodyB = await pollGet()
  const sidB = (bodyB.match(/"sid":"([^"]+)"/) || [])[1]
  if (!sidB) { log('客户端 B 连接', false); return }
  await waitForMessages(sidB, 3000) // 等 CONNECT

  await emitEvent(sidB, 'room:join', { room: 'multi-test-room', identity: 'user-B' })
  await sleep(500)

  const msgsB = await waitForMessages(sidB, 3000)
  const joinResult = msgsB.find(m => m.includes('room:joined') && m.includes('user-B'))
  log('B 收到加入确认', !!joinResult, joinResult ? joinResult.slice(0, 100) : '无响应')

  // A 应收到 member:joined 通知
  const msgsA = await waitForMessages(sidA, 3000)
  const memberJoined = msgsA.find(m => m.includes('member:joined') && m.includes('user-B'))
  log('A 收到 member:joined 通知', !!memberJoined)

  // 清理
  await emitEvent(sidB, 'room:leave', { room: 'multi-test-room' })
  await pollPost(sidB, '1')
  await emitEvent(sidA, 'room:leave', { room: 'multi-test-room' })
  await pollPost(sidA, '1')
}

// ---- 入口 ----
async function run() {
  console.log('\n🔌 Socket.IO WebSocket 集成测试\n')

  const sid = await testConnection()
  if (sid) {
    await testRoomCreate(sid)
    await testRoomJoin(sid)
    await testRoomList(sid)
    await testRoomLeave(sid)
    await testDisconnect(sid)
    await testMultipleClients()
  }

  console.log('\n' + '-'.repeat(56))
}

module.exports = run
