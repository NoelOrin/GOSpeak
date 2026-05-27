// ============================================================
// Signal 信令模块测试
//   覆盖接口：
//     POST /api/v1/signal/token        - 获取 LiveKit 加入 Token
//     POST /api/v1/signal/signal       - 信令中转 (offer/answer/ICE)
//     GET  /api/v1/signal/rooms        - 房间列表
//     GET  /api/v1/signal/participants - 房间参与者
// ============================================================
const { request, log, separator, getToken } = require('../config')

// ---------- 获取 Join Token 测试 ----------
//   正常参数 → 空参数
async function testGetJoinToken() {
  separator('Signal /token')

  // 正常请求，预期返回 LiveKit 房间加入 Token
  const r1 = await request('POST', '/signal/token', {
    body: { room: 'test-room', identity: 'test-user' },
  })
  log('获取加入房间 Token', r1)

  if (r1.data?.access_token) {
    console.log(`       token: ${r1.data.access_token.slice(0, 40)}...`)
  }

  // 空房间和身份，预期参数校验错误
  const r2 = await request('POST', '/signal/token', {
    body: { room: '', identity: '' },
  })
  log('空参数获取 Token', r2)
}

// ---------- 信令中转测试 ----------
//   依次发送 offer / answer / ICE candidate 三种信令
async function testSignal() {
  separator('Signal /signal')

  // 发送 WebRTC Offer
  const r1 = await request('POST', '/signal/signal', {
    body: { type: 'offer', room: 'test-room', identity: 'test-user', data: { sdp: 'mock-sdp' } },
  })
  log('发送 Offer 信令', r1)

  // 发送 WebRTC Answer
  const r2 = await request('POST', '/signal/signal', {
    body: { type: 'answer', room: 'test-room', identity: 'test-user', data: { sdp: 'mock-sdp' } },
  })
  log('发送 Answer 信令', r2)

  // 发送 ICE Candidate
  const r3 = await request('POST', '/signal/signal', {
    body: { type: 'ice_candidate', room: 'test-room', identity: 'test-user', data: { candidate: 'mock-candidate' } },
  })
  log('发送 ICE Candidate 信令', r3)

  // 空请求体，预期参数校验错误
  const r4 = await request('POST', '/signal/signal', {
    body: {},
  })
  log('缺少 type 字段', r4)
}

// ---------- 获取房间列表测试 ----------
async function testListRooms() {
  separator('Signal /rooms')

  // 获取所有活跃房间（依赖 LiveKit 配置，未配置时可能返回空或错误）
  const r1 = await request('GET', '/signal/rooms')
  log('获取房间列表', r1)
}

// ---------- 获取参与者列表测试 ----------
//   带 room 参数 → 不带 room 参数
async function testListParticipants() {
  separator('Signal /participants')

  // 获取指定房间的参与者列表
  const r1 = await request('GET', '/signal/participants?room=test-room')
  log('获取房间参与者', r1)

  // 不传 room 参数，预期提示 room is required
  const r2 = await request('GET', '/signal/participants')
  log('缺 room 参数', r2)
}

// ---------- 入口 ----------
//   按顺序执行所有 Signal 测试用例
async function run() {
  console.log('\n🎥 Signal 信令模块测试\n')

  await testGetJoinToken()
  await testSignal()
  await testListRooms()
  await testListParticipants()

  console.log('\n' + '-'.repeat(56))
}

module.exports = run