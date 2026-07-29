# E2E 端到端测试计划 — Guild + WebSocket Migration

> **基础框架:** 基于现有 `.agents/skills/room-voice-e2e/` (Playwright)
> **新增套件:** Guild 管理 / 跨 Guild 隔离 / WS 协议 / WS 断连恢复 / Phase 1+2 全链路
> **测试范围:** 浏览器 → 前端 → API → WS → 信号层 → SFU 全链路

---

## 一、E2E 架构总览

### 测试层次

```
┌─────────────────────────────────────────────────┐
│              Playwright 浏览器 E2E               │
│  (ui-helpers.mjs / media-probe.mjs / 新 helpers) │
├─────────────────────────────────────────────────┤
│         HTTP API 层 (axios/fetch)                │
│  登录 / Guild CRUD / 房间管理 / 权限配置         │
├─────────────────────────────────────────────────┤
│         WebSocket 信号层 (socket.io→原生 WS)      │
│  房间加入/离开/踢人/禁言/消息/发言检测           │
├─────────────────────────────────────────────────┤
│         SFU 媒体层 (LiveKit/SRS/etc.)             │
│  推流/拉流/静音/远端音频                         │
└─────────────────────────────────────────────────┘
```

### 与现有 room-voice-e2e 的关系

| 维度 | 现有（room-voice-e2e） | 新增（本计划） |
|------|----------------------|----------------|
| 测试套件 | join / switch / rapid-switch / media / multi-user | guild / guild-room-isolation / guild-membership / ws-reconnect / ws-protocol / full-phase |
| 测试数据 | 房间名 + 用户 | Guild + 房间 + 跨 Guild 用户 + 角色 |
| 鉴权方式 | 用户名密码登录 → JWT → socket.io | 同左 + API token（Phase 2 后 WS 直连） |
| 媒体验证 | getUserMedia / RTCPeerConnection | 同左 |
| 断言基础 | UI 状态 + media probe | UI 状态 + media probe + API 响应 + WS 消息抓包 |

---

## 二、E2E 基础设施增强

### 2.1 新增 Playwright helper 文件

```
.agents/skills/room-voice-e2e/
├── scripts/
│   ├── run-room-voice-e2e.mjs        # 已有（扩�� suite 列表）
│   ├── ui-helpers.mjs                # 已有（扩增 guild 帮助函数）
│   ├── media-probe.mjs               # 已有（不变）
│   ├── guild-helpers.mjs             # 新增 — Guild API + UI 操作
│   ├── ws-helpers.mjs                # 新增 — WS 消息抓包 + 协议验证
│   └── cleanup-helpers.mjs           # 新增 — 测试清理（删 Guild/房间/用户）
└── references/
    ├── scenarios.md                  # 已有（扩增 Guild/WS 场景）
    ├── selectors.md                  # 已有（扩增 Guild UI 选择器）
    ├── media-assertions.md           # 已有
    └── guild-selectors.md            # 新增 — Guild UI 专用选择器
```

### 2.2 Guild Helpers (`guild-helpers.mjs`)

```javascript
// ===== Guild CRUD =====

/** 通过 API 创建 Guild，返回 guild 对象 */
async function createGuild(page, name, { description, isPublic } = {}) {
  const token = await getAuthToken(page);
  const res = await page.evaluate(async (opts) => {
    const r = await fetch('/api/v1/guild/create', {
      method: 'POST', headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${opts.token}` },
      body: JSON.stringify({ name: opts.name, description: opts.description || '', is_public: opts.isPublic || false }),
    });
    return r.json();
  }, { token, name, description, isPublic });
  if (res.code !== 0) throw new Error(`createGuild failed: ${res.msg}`);
  return res.data;
}

/** 通过 API 加入 Guild */
async function joinGuild(page, inviteCode) {
  // ... 类似模式
}

/** 通过 UI 左侧栏查看当前 Guild */
async function getVisibleGuilds(page) {
  return page.$$eval('[data-testid="guild-icon"]', els =>
    els.map(el => ({ name: el.title, uuid: el.dataset.guildUuid }))
  );
}

/** 在 UI 中切换到指定 Guild */
async function selectGuild(page, guildUUID) {
  await page.click(`[data-testid="guild-icon"][data-guild-uuid="${guildUUID}"]`);
  await page.waitForSelector('[data-testid="guild-name"]');
}

// ===== Guild 成员管理 =====

/** 通过 API 获取 Guild 成员列表 */
async function getGuildMembers(page, guildUUID) {
  // ... GET /api/v1/guild/members
}

/** 通过 API 踢出成员 */
async function kickGuildMember(page, guildUUID, userUUID) {
  // ... POST /api/v1/guild/kick
}
```

### 2.3 WS 协议 Helpers (`ws-helpers.mjs`)

```javascript
/**
 * 在浏览器上下文中注入 WS 消息监听器。
 * 捕获所有 WebSocket 消息（客户端 → 服务端 & 服务端 → 客户端）。
 * 返回一个 { sent, received, waitForEvent } 对象。
 *
 * Phase 2 迁移前: 监听 socket.io 的 emit/on 调用
 * Phase 2 迁移后: 监听原生 WebSocket message 事件
 */
async function installWSProbe(page) {
  return await page.evaluate(() => {
    const probe = { sent: [], received: [], waitResolve: {} };

    // 兼容 socket.io（Phase 2 前）
    const origEmit = window.socket?.emit;
    if (origEmit) {
      window.socket.emit = function(event, ...args) {
        probe.sent.push({ event, args: args.map(a => typeof a === 'string' ? JSON.parse(a) : a) });
        return origEmit.apply(this, arguments);
      };
    }

    // 兼容原生 WebSocket（Phase 2 后）— 替换全局 WebSocket
    const OrigWS = window.WebSocket;
    if (OrigWS) {
      window.WebSocket = function(...args) {
        const ws = new OrigWS(...args);
        const origSend = ws.send;
        ws.send = function(data) {
          probe.sent.push({ raw: data });
          origSend.apply(this, arguments);
        };
        ws.addEventListener('message', (event) => {
          probe.received.push({ raw: event.data });
          // 解析 JSON 后 check waitForEvent
          try {
            const msg = JSON.parse(event.data);
            if (msg.event && probe.waitResolve[msg.event]) {
              probe.waitResolve[msg.event](msg);
              delete probe.waitResolve[msg.event];
            }
          } catch {}
        });
        return ws;
      };
    }

    probe.waitForEvent = (event, timeout = 10000) => {
      return new Promise((resolve, reject) => {
        probe.waitResolve[event] = resolve;
        setTimeout(() => reject(new Error(`waitForEvent ${event} timeout`)), timeout);
      });
    };

    return probe;
  });
}

/**
 * RoomRequest 帮助函数：构建标准 WS 消息。
 * Phase 1 后自动加入 guild_uuid 字段。
 */
function makeRoomRequest(room, guildUUID, extra = {}) {
  return { room, guild_uuid: guildUUID, ...extra };
}
```

### 2.4 测试账号 seed 脚本

新建 `scripts/seed-e2e-data.mjs`（独立执行，在每个 E2E 运行前准备数据）：

```javascript
/**
 * E2E 测试数据准备：
 * 1. 创建/确保 E2E_USER / E2E_USER_B 账号存在
 * 2. 重置密码为非强制改密态
 * 3. 创建 E2E Guild（若不存在）
 * 4. 创建 E2E 房间（若不存在）
 *
 * 运行: node scripts/seed-e2e-data.mjs [--reset]
 */
```

---

## 三、Phase 1 — Guild E2E 套件

### Suite: `guild` — Guild 创建管理

| 步骤 | 操作 | 断言 |
|------|------|------|
| 1 | 用户 A 登录 | Dashboard 可见 |
| 2 | 点击"创建服务器"按钮 | CreateGuildModal 弹出 |
| 3 | 输入名称 "E2E Test Guild"，点创建 | API 返回 0，返回 guild UUID |
| 4 | 左侧 Guild 栏出现新图标 | 图标首字母显示 "ET" |
| 5 | 点图标进入 Guild | 标题显示 "E2E Test Guild" |
| 6 | 创建第二个 Guild "E2E Guild B" | 左侧出现两个图标 |
| 7 | 在两个 Guild 间切换 | 每次切换后标题和房间列表更新 |

**通过标准:**
- Guild 图标在左侧栏正确渲染（首字母 / 头像）
- 切换 Guild 时房间列表刷新
- API 返回值与 UI 显示一致

### Suite: `guild-join` — Guild 邀请加入

| 步骤 | 操作 | 断言 |
|------|------|------|
| 1 | 用户 A 创建 Guild "E2E Private" | 返回 invite_code |
| 2 | 用户 A 复制邀请码显示在 UI | UI 展示邀请码 |
| 3 | 用户 B 打开"加入服务器"弹窗 | JoinGuildModal 弹出 |
| 4 | 用户 B 输入邀请码，点加入 | API 返回 0 |
| 5 | 用户 B 左侧栏出现 Guild 图标 | 可见 |
| 6 | 用户 A 查看成员列表 | 显示 2 名成员（A + B） |

**通过标准:**
- 邀请码加入全流程
- 双方成员列表同步

### Suite: `guild-room-isolation` — 跨 Guild 房间隔离

| 步骤 | 操作 | 断言 |
|------|------|------|
| 1 | 用户 A 在 Guild A 创建房间 "lobby" | 房间创建成功 |
| 2 | 用户 A 在 Guild B 创建同名房间 "lobby" | 房间创建成功 |
| 3 | 用户 A 进入 Guild A.lobby | 房间列表显示 Guild A 的房间 |
| 4 | 用户 B 加入 Guild B 并进入 Guild B.lobby | 双方在不同 Guild 的同名房间 |
| 5 | 用户 A 在 Guild A.lobby 说话 | 用户 B **不** 应收到 Guild A 的音频 |
| 6 | 用户 B 查看 Guild B 的房间列表 | 不包含 Guild A 的房间 |

**通过标准:**
- `roomKey(guildUUID, roomName)` 隔离生效
- WS/Fanout 广播不跨 Guild
- 房间列表按 Guild 过滤

### Suite: `guild-membership` — Guild 成员角色

| 步骤 | 操作 | 断言 |
|------|------|------|
| 1 | 用户 A (owner) 将用户 B 提升为 admin | API 返回 0 |
| 2 | 用户 B 查看成员管理 UI | 可见踢人按钮 |
| 3 | 用户 B (admin) 踢出用户 C | API 返回 0 |
| 4 | 用户 C 尝试重新加入 | 可重新加入（被踢不 shut） |
| 5 | 用户 A 转让 owner 给用户 B | API 返回 0 |
| 6 | 用户 A 尝试删除 Guild（现为普通成员）| API 返回 403 |
| 7 | 用户 B (新 owner) 删除 Guild | API 返回 0 |

**通过标准:**
- 角色层级正确执行 (owner ≥ admin ≥ member)
- 转让 owner 后权限即时生效
- 非 owner 不能删除 Guild

### Suite: `guild-multi-room` — Guild 内多房间语音

**前置条件:** 需准备 2 个用户 (A, B)，同一 Guild，同一 SFU provider。

| 步骤 | 操作 | 断言 |
|------|------|------|
| 1 | 用户 A 在 Guild X 创建房间 A 和 房间 B | 两个房间 |
| 2 | 用户 A 进房间 A，用户 B 进房间 B | 各自在对应房间 |
| 3 | 用户 A 切换到房间 B | A 离开房间 A + A 加入房间 B |
| 4 | 用户 A 在房间 B 与 B 通话 | 双方媒体正常（如 multi-user 套件标准） |
| 5 | 创建第 3 个用户 C，加入 Guild X | C 看到房间 A、B 列表 |

**通过标准:**
- 同 Guild 内多房间互不干扰
- 跨 Guild 房间切换不影响其他 Guild
- 新人看到所有可见房间

---

## 四、Phase 2 — WS 迁移 E2E 套件

### Suite: `ws-connect` — WS 连接生命周期

| 步骤 | 操作 | 断言 |
|------|------|------|
| 1 | 用户登录后安装 WS probe | WS 连接建立（socket.io 的 connect 事件或 WS readyState=OPEN） |
| 2 | 检查连接鉴权 | 请求携带 JWT token（header / cookie / query） |
| 3 | 触发 `room:list` 请求 | 收到 `room:list:result` 响应 |
| 4 | 检查消息格式 | 请求: `{"id":"...","event":"room:list"}` 响应: `{"id":"...","event":"room:list:result","data":{...}}` |
| 5 | 触发无 id 推送事件（如 member:joined）| 响应不含 `id` 字段 |
| 6 | 故意发非法 JSON | 服务端不 panic，客户端不崩溃 |

**通过标准:**
- WS 连接在登录后自动建立
- 消息 JSON 格式与 Phase 2 协议定义一致
- 请求-应答使用 `id` 关联，推送无 `id`
- 非法消息静默忽略

### Suite: `ws-reconnect` — WS 断连恢复

| 步骤 | 操作 | 断言 |
|------|------|------|
| 1 | 用户 A 进房间 (Guild X.lobby) | 媒体正常 (Phase 1 标准) |
| 2 | 模拟断连: `page.evaluate(() => socket?.disconnect())` | 客户端触发重连逻辑 |
| 3 | 等待自动重连 | WS 重新连接 |
| 4 | 用户 A 仍显示在房间成员列表中 | 服务端 OnClientDisconnect 处理后重建？取决于设计 |
| 5 | 断连期间其他用户发消息 | 重连后收到（如实现了消息持久化） |

**通过标准（Phase 2 设计决策，二选一）:**
- **方案 A（内存状态）:** 断连后 OnClientDisconnect 清理房间成员，重连后需重新 join room. 成员列表暂时消失后恢复。
- **方案 B（持久化状态）:** 断连不清成员，重连后重新注册到 fanout。成员列表断连期间可见但静音。

### Suite: `ws-protocol` — Guild-WS 联合协议

| 步骤 | 操作 | 断言 |
|------|------|------|
| 1 | 用户 A 在 Guild X 发送 `room:join` 带 guild_uuid | 加入房间 `roomKey(GuildX, roomName)` |
| 2 | 用户 B 在 Guild Y 对**同名 room** 发 `room:join` | 独立加入，不互相影响 |
| 3 | 用户 A 发 `room:kick` 带目标 identity | 仅目标收到 `room:kicked` 事件 |
| 4 | 用户 A 发 `member:mic-state` | 房间内广播 `member:updated` |
| 5 | 用户 A 发 `message:send` 带 content | ACK 返回 message ID，房间广播 `message:new` |

**通过标准:**
- 所有 11 个事件通过 WS 线协议可正常收发
- GuildUUID 正确出现在 room 相关请求中
- ACK 和非 ACK 事件格式区分正确
- error ACK 返回正确错误码

### Suite: `ws-concurrency` — 多连接并发

| 步骤 | 操作 | 断言 |
|------|------|------|
| 1 | 创建 3 个用户 (A, B, C) 同时登录 | 3 个 WS 连接 |
| 2 | A 创建房间，B、C 同时加入 | 三人都收到 member:joined 广播 |
| 3 | A 同时踢 B（通过 WS），同时 C 发送消息 | B 收到 kicked，C 的消息正常 ACK |
| 4 | A 同时发送 mic:state（静音）+ B 发消息 | 两条广播均被各自目标接收 |
| 5 | 断开 A 的连接（模拟网络断开） | B、C 收到 member:left(identity=A) |

**通过标准:**
- 并发消息无丢失、无乱序、无 panic
- Fanout 读写锁不导致死锁
- 广播正确到达所有预期目标

---

## 五、跨阶段全链路 Suite

### Suite: `full-phase` — Phase 1 + 2 端到端

整合所有场景的单次全链路验证：

```
1. 用户 A 登录
2. 创建 Guild "E2E Full"
3. 创建房间 "lobby"（Guild 内）
4. 进入房间
5. 用户 B 通过邀请码加入 Guild
6. 用户 B 进入同一房间
7. WS probe 验证双方消息格式正确
8. 双方媒体正常（getUserMedia + RTCPeerConnection）
9. 用户 A 踢出用户 B
10. 用户 B 收到 kicked 事件
11. 用户 A 切换房间 → 房间列表正确
12. 用户 A 发送文本消息
13. ACK 返回 + 广播
14. 用户 A 断开网络 → 自动重连
15. 创建第二个 Guild → 房间隔离验证
16. 清理：删除 Guild、登出
```

**执行命令:**

```bash
cd .agents/skills/room-voice-e2e/scripts
E2E_USER=e2e_user E2E_PASS=e2e_pass E2E_USER_B=e2e_user_b E2E_PASS_B=e2e_pass_b node run-room-voice-e2e.mjs --suite full-phase
```

---

## 六、套件矩阵总表

### Phase 1 套件

| Suite ID | 名称 | 用户数 | 执行时间 | 通过标准 |
|----------|------|--------|----------|----------|
| `guild` | Guild 创建管理 | 1 | ~30s | CRUD API + UI 一致 |
| `guild-join` | 邀请加入 | 2 | ~30s | 邀请码全流程 |
| `guild-room-isolation` | 跨 Guild 房间隔离 | 2 | ~40s | roomKey 隔离生效 |
| `guild-membership` | 成员角色 | 2~3 | ~45s | 角色层级执行正确 |
| `guild-multi-room` | Guild 多房间语音 | 2~3 | ~60s | 同 Guild 多房间媒体 |

### Phase 2 套件

| Suite ID | 名称 | 用户数 | 执行时间 | 通过标准 |
|----------|------|--------|----------|----------|
| `ws-connect` | WS 连接生命周期 | 1 | ~20s | 协议格式 + 鉴权 |
| `ws-reconnect` | WS 断连恢复 | 1~2 | ~30s | 重连后状态一致 |
| `ws-protocol` | Guild-WS 联合协议 | 2 | ~45s | 所有事件收发正确 |
| `ws-concurrency` | 多连接并发 | 3 | ~30s | 无丢失/死锁 |

### 全链路套件

| Suite ID | 名称 | 用户数 | 执行时间 | 通过标准 |
|----------|------|--------|----------|----------|
| `full-phase` | Phase 1+2 全链路 | 2 | ~120s | 16 步全流程通过 |

### 已有套件（保持不变）

| Suite ID | 名称 | 用户数 |
|----------|------|--------|
| `join` | 创建并进入房间 | 1 |
| `switch` | 切房 | 1 |
| `rapid-switch` | 快速切房 | 1 |
| `media` | 推流/会话 | 1 |
| `multi-user` | 多人拉流 | 2 |

---

## 七、运行与 CI 集成

### 7.1 本地执行

```bash
# 安装依赖（首次）
cd .agents/skills/room-voice-e2e/scripts
pnpm install

# 确保后端+前端运行
cd /Users/noelorin/GOSpeak
# 终端1: app/server && go run . server
# 终端2: cd app/web && pnpm dev

# Phase 1 套件
E2E_USER=e2e_user E2E_PASS=e2e_pass node run-room-voice-e2e.mjs --suite guild

# Phase 2 套件
E2E_USER=e2e_user E2E_PASS=e2e_pass E2E_USER_B=e2e_user_b E2E_PASS_B=e2e_pass_b   node run-room-voice-e2e.mjs --suite ws-protocol

# 全链路
E2E_USER=e2e_user E2E_PASS=e2e_pass E2E_USER_B=e2e_user_b E2E_PASS_B=e2e_pass_b   node run-room-voice-e2e.mjs --suite full-phase
```

### 7.2 CI Job 配置

```yaml
# .github/workflows/e2e.yml
name: E2E Tests
on:
  push:
    branches: [main, phase-*]
  pull_request:

jobs:
  guild-e2e:
    runs-on: ubuntu-latest
    services:
      app:  # Go 后端
        build: ./app/server
        ports: ['8998:8998']
    steps:
      - uses: actions/checkout@v4
      - uses: pnpm/action-setup@v4
      - uses: actions/setup-node@v4
      - uses: actions/setup-go@v5
      - run: cd app/server && go run . server --env dev &
      - run: cd app/web && pnpm install && pnpm build && pnpm preview --port 4173 &
      - run: |
          cd .agents/skills/room-voice-e2e/scripts
          pnpm install
          E2E_USER=e2e_test E2E_PASS=e2e_test           E2E_USER_B=e2e_test_b E2E_PASS_B=e2e_test_b           node run-room-voice-e2e.mjs --suite guild

  ws-e2e:
    # ... 类似，--suite ws-protocol,ws-connect

  full-phase-e2e:
    # ... 仅 main/phase-2 merge 后触发
    if: github.ref == 'refs/heads/main'
    steps:
      # ... 同上，--suite full-phase
```

### 7.3 测试账号管理

```bash
# 首次运行前 seed
node scripts/seed-e2e-data.mjs

# seed + 清理旧数据
node scripts/seed-e2e-data.mjs --reset
```

需要的环境变量：

| 变量 | 必填 | 说明 |
|------|------|------|
| `E2E_USER` | 是 | 主测试用户账号（自动创建） |
| `E2E_PASS` | 是 | 密码 |
| `E2E_USER_B` | 多人套件 | 第二用户 |
| `E2E_PASS_B` | 多人套件 | 密码 |
| `E2E_USER_C` | concurrency 套件 | 第三用户 |
| `E2E_PASS_C` | concurrency 套件 | 密码 |
| `BASE_URL` | 否 | 默认 `http://localhost:3000` |
| `E2E_HEADLESS` | 否 | 默认 1 |
| `E2E_SKIP_MEDIA` | 否 | 跳过媒体断言（CI 无虚拟音频时用）|

### 7.4 报告与失败诊断

执行后自动生成报告到 `agent_test_logs/`:

```
agent_test_logs/
├── e2e-guild-2026-07-29-14-30.md          # 套件报告
├── e2e-ws-protocol-2026-07-29-14-31.md
├── artifacts/
│   ├── guild-room-isolation-failure-1.png  # 失败截图
│   └── ws-protocol-msg-log.json            # WS 消息日志（调试用）
```

报告格式（与现有 `test-logging` skill 一致）：

```markdown
# E2E Report: guild (2026-07-29 14:30)
- Suite: guild
- Environment: CI / Local
- SFU Provider: livekit
- Result: ✅ 4/4 passed (1 skipped: guild-multi-room requires 3 users)

## Results
| Test | Status | Duration |
|------|--------|----------|
| Guild 创建管理 | ✅ | 12.3s |
| 邀请加入 | ✅ | 14.1s |
| 跨 Guild 隔离 | ✅ | 18.7s |
| 成员角色 | ✅ | 22.4s |
| 多房间语音 | ⏭️ SKIP | — (needs 3 users) |

## Details
- guild-create: API 返回 0, UI 显示正确
- guild-join: B 用户通过邀请码加入, 双方成员列表同步
- guild-room-isolation: A 说话 B 在另一个 Guild 听不到
- guild-kick: admin 踢 member 成功
```

---

## 八、执行顺序与里程碑

### Phase 1 E2E 就绪（Guild 实现后）

```
合并条件:
├── guild 套件 ✅
├── guild-join 套件 ✅
├── guild-room-isolation 套件 ✅
├── guild-membership 套件 ✅
└── guild-multi-room 套件 ✅ (可选, 依赖 3 用户)
```

### Phase 2 E2E 就绪（WS 迁移后）

```
合并条件:
├── ws-connect 套件 ✅ (协议格式验证)
├── ws-protocol 套件 ✅ (所有事件类型)
├── ws-reconnect 套件 ✅ (断连恢复)
└── ws-concurrency 套件 ✅ (并发安全)
```

### 全链路就绪（Phase 2 合并后）

```
合入 main 条件:
├── full-phase 套件 ✅ (16 步全流程)
├── 已有 voice 套件全量回归: join/switch/rapid-switch/media/multi-user ✅
└── 对比 Phase 1 基线无退化
```
