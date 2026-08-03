# GOSpeak E2E 测试规范

> 状态：生效
> 适用范围：浏览器端到端测试
> 基础实现：`.agents/skills/room-voice-e2e/`
> 报告规则：`agent_test_logs/AGENTS.md`

## 1. 目的与范围

本规范定义 GOSpeak 浏览器端到端测试的统一执行方式，保证语音房间相关回归测试可重复、可诊断、可报告。

测试链路：

```text
Playwright/浏览器 UI
  → 前端页面与状态
  → HTTP/WS 信令
  → 信号 Hub 与房间状态
  → SFU Provider（LiveKit / SRS / Agora / Cloudflare）
```

本规范覆盖：

- 登录、房间列表、创建房间、加入房间
- 房间切换与快速切换
- 本地推流、远端拉流、多人同房音频
- 浏览器操作、媒体探针、失败截图、测试报告

本规范不覆盖纯 API、单元测试、真实听感验收；这些场景分别由后端测试、前端单测和 computer-use 补充。

## 2. 测试分层与判定优先级

浏览器 E2E 是最高层验收，失败时按以下顺序定位：

1. UI 状态：能否稳定停留在目标房间
2. 信令成员一致性：多人是否互见
3. 媒体会话：本地是否成功采集并发布
4. 远端拉流：多人是否收到并 attach 远端 audio

判定不得只依据“页面没有报错”，必须使用 UI 状态、媒体探针和网络/控制台日志组合验证。

## 3. 环境与前置条件

### 3.1 服务要求

| 服务 | 默认地址 | 健康检查 |
|------|----------|----------|
| Go 后端 | `http://localhost:8998` | `curl -sS http://localhost:8998/ping` |
| SolidJS 前端 | `http://localhost:3000` | `curl -sS -o /dev/null -w "%{http_code}\n" http://localhost:3000/login` |
| 当前 SFU Provider | 随 `.env.dev` / SFU 配置 | 进入房间后按媒体探针验证 |

启动命令：

```bash
# 后端
cd app/server && go run . server

# 前端
pnpm --filter @gospeak/web dev
```

### 3.2 测试账号

| 变量 | 必填 | 用途 |
|------|------|------|
| `E2E_USER` | 是 | 主测试用户 |
| `E2E_PASS` | 是 | 主测试用户密码 |
| `E2E_USER_B` | 多人套件 | 第二用户 |
| `E2E_PASS_B` | 多人套件 | 第二用户密码 |
| `E2E_USER_C` | 并发/多用户扩展套件 | 第三用户 |
| `E2E_PASS_C` | 并发/多用户扩展套件 | 第三用户密码 |

账号约束：

- 禁止使用默认 `admin/admin123` 首登强制改密态账号。
- 测试账号应可重复登录，且不处于首次改密流程。
- 报告中不得记录完整密码或长 token。

### 3.3 测试数据隔离

- 房间名必须使用 `e2e-*` 前缀。
- 测试创建的 Guild、房间、消息等数据应在执行后清理。
- 禁止删除用户数据库、生产房间或非 `e2e-*` 数据。
- 多用户测试使用不同 Playwright context，不得复用同一登录态。

### 3.4 浏览器选择

默认优先系统默认浏览器，其次按以下顺序 fallback：

```text
E2E_BROWSER 显式指定 → OS 默认浏览器 → chrome → chromium → msedge → firefox → webkit
```

- Chromium 系默认注入 fake media；Firefox/WebKit 不注入 fake media 启动参数。
- 失败时允许自动 fallback，但报告中必须标记 `fallback`。
- 需要真实窗口、权限弹窗或真实听感时，才改用 computer-use。

## 4. 套件矩阵

| Suite | 目标 | 最少用户 | 通过标准 |
|-------|------|----------|----------|
| `join` | 创建并进入房间 | 1 | 目标房间标题 + `离开` 可见 + 媒体 ready |
| `switch` | A→B 直接切房 | 1 | 最终停留 B，UI/媒体正常 |
| `rapid-switch` | A/B 快速来回 | 1 | 每轮 join 成功，无 failed/retry |
| `media` | 单人推流/会话 | 1 | getUserMedia + RTCPeerConnection/local track |
| `multi-user` | 本地多人拉流 | 2 | 双方人数 ≥2，各自收到远端 audio |
| `guild`（扩展） | Guild 创建/切换/成员 | 1~2 | API 与 UI 一致，成员同步 |
| `ws-*`（扩展） | WS 协议/重连/并发 | 1~3 | 协议格式、状态恢复、无消息丢失 |

当前语音回归以 `join`、`switch`、`rapid-switch`、`media`、`multi-user` 为准；`guild` 与 `ws-*` 在对应功能合入后纳入全量回归。

## 5. 用例规范

### 5.1 join

1. 登录后进入域工作区 `/domain/:domainUUID`。
2. 使用 `title="新建房间"` 图标按钮创建唯一房间 `e2e-join-*`。
3. 双击左侧房间行进入，不使用单击。
4. 等待 `离开` 按钮可见且房间标题为目标房间名。
5. 检查媒体探针：`getUserMediaCalls >= 1` 或 local audio track live，且存在 RTCPeerConnection（部分 provider 可放宽到 joined UI + gum）。

失败信号：

- 长期停在 `加载语音引擎...` / `连接媒体...`
- 出现 `加入失败` / `重试`
- console 有 token/SFU/ICE 致命错误

### 5.2 switch

1. 创建 roomA、roomB。
2. 进入 roomA，确认 joined。
3. 不点离开，直接双击 roomB。
4. 确认标题为 roomB，`离开` 仍可见。
5. 媒体探针仍 ready，允许短暂 reconnecting。

关键回归点：

- 旧房 teardown 与新房 join 竞态。
- `leaveRoom` fire-and-forget 不应清掉新房 `currentRoom`。

### 5.3 rapid-switch

1. 创建 roomA、roomB。
2. 默认以 `120ms` 间隔在 A/B 间切换，默认 `3` 轮。
3. 每轮必须等待 joined 后再切下一个。
4. 结束后最终房间 media ready。

失败信号：

- 任意一次切换超时。
- UI 显示旧房名但成员/媒体是新房，或反向不一致。
- 多次切换后必须手动刷新才能恢复。

### 5.4 media

1. 进房。
2. 断言本地采集与发布路径：
   - getUserMedia 被调用
   - 存在 audio local track 或 sender
   - RTCPeerConnection 进入 connecting/connected（或 provider 等价成功态）
3. 单人套件不能证明远端拉流，拉流以 `multi-user` 为准。

可选补充：

- 观察成员卡 speaking 指示。
- 切换麦克风静音后 local sender muted/enabled 变化。

### 5.5 multi-user

1. Context A 登录 userA，创建 roomM 并加入。
2. Context B 登录 userB，加入同一 roomM。
3. 双方 UI 显示 ≥2 人在线。
4. 双方媒体探针检测到至少 1 路 remote audio（`srcObject` 含 live audio track）。
5. 可选：A 离开后，B 远端音频移除、人数回落。

失败信号：

- 成员列表互不可见：信令问题。
- 成员可见但无 remote audio：订阅/拉流问题。
- 只有一方能听到：单向发布/订阅、权限、autoplay 问题。

## 6. 浏览器操作规范

- 进入域工作区 `/domain/:domainUUID` 后操作房间列表。
- 房间进入方式为**双击**左侧房间行；创建按钮使用 `button[title="新建房间"]`。
- 切房时直接双击目标房间，不需要先点 `离开`。
- 多人场景使用两个独立 Playwright context，避免共享 cookie。
- 每次关键操作后记录：
  - 页面截图
  - 媒体探针快照
  - console 错误
  - 网络请求中的 token/SFU 关键失败

选择器基线见 `.agents/skills/room-voice-e2e/references/selectors.md`；不得在业务代码中加入 provider 特判或测试专用选择器，除非有明确的测试可维护性需求。

## 7. 媒体断言规范

### 7.1 推流

```js
const snap = await page.evaluate(() => window.__gospeakMediaProbe.getSnapshot());
const published =
  snap.getUserMediaCalls > 0 ||
  snap.localTracks.some((t) => t.kind === "audio" && t.readyState === "live");
const sessioned = snap.peerConnections.some((pc) =>
  ["connecting", "connected", "checking", "completed"].includes(pc.iceConnectionState) ||
  ["connecting", "connected"].includes(pc.connectionState)
);
expect(published && (sessioned || snap.hasLeaveButton)).toBeTruthy();
```

### 7.2 拉流

```js
const remote = await page.evaluate(() =>
  window.__gospeakMediaProbe.waitForRemoteAudio(1, 25000)
);
// remote.ok === true
// remote.remote[i].liveTracks >= 1
```

### 7.3 Provider 差异

| Provider | 说明 |
|----------|------|
| LiveKit | PC/track 语义最标准 |
| SRS | WHIP publish + WHEP subscribe；`media_ready` 可能早于 signal 完成 |
| Agora | SDK 内部封装，PC hook 可能不完整，优先 UI joined + remote audio DOM |
| Cloudflare | WHIP/WHEP 会话；关注 tracks/new 与 remote attach |

当某 provider 无法通过 `RTCPeerConnection` wrap 抓取实例时，按以下顺序降级：

1. joined UI
2. getUserMedia 调用
3. multi-user remote audio DOM
4. 网络面板 token/WHIP/WHEP 2xx

详细实现见 `.agents/skills/room-voice-e2e/references/media-assertions.md`。

## 8. 失败分类与诊断

| 归类 | 典型现象 | 优先排查 |
|------|----------|----------|
| 环境 | 前后端/SFU 未起、账号改密拦截 | 健康检查、账号 seed、provider 配置 |
| 信令 | 房间列表空、成员互不可见 | WS 连接、房间列表、member 广播 |
| 进房编排 | 长期 loading、token 失败 | signal token、SFU client 加载 |
| 推流 | gum 无调用、无 local track/PC | 浏览器权限、fake media、采集逻辑 |
| 拉流 | 成员可见但无 remote audio | subscribe、attach、autoplay |
| 切房竞态 | 快切后卡死、房名与媒体不一致 | teardown/join 竞态、currentRoom 状态 |

失败处理流程：

1. 保留失败截图到 artifacts。
2. 导出媒体探针快照。
3. 收集 console/network 关键错误。
4. 记录当前 `SFU_PROVIDER` 和套件阶段。
5. 归类后写入报告；无法归类时标注“待人工分析”，不得直接判定产品通过。

## 9. 测试数据生命周期

### 9.1 准备

- 首次运行前确保 `E2E_USER` / `E2E_USER_B` 存在且可登录。
- 可使用 seed 脚本创建/重置测试账号，禁止破坏现有账号状态。
- 测试房间统一使用 `e2e-*` 前缀。

### 9.2 清理

- 每个套件结束或失败后清理临时房间。
- Guild/WS 扩展套件结束后删除测试 Guild 与关联房间。
- 保留 artifacts 与报告，不保留临时账号密码明文。

## 10. 执行方式

### 10.1 本地执行

```bash
cd .agents/skills/room-voice-e2e/scripts
pnpm install

# 全量语音回归
BASE_URL=http://localhost:3000 \
E2E_USER=user1 E2E_PASS=pass1 \
E2E_USER_B=user2 E2E_PASS_B=pass2 \
node run-room-voice-e2e.mjs --suite all

# 单套件
node run-room-voice-e2e.mjs --suite join
node run-room-voice-e2e.mjs --suite switch
node run-room-voice-e2e.mjs --suite rapid-switch
node run-room-voice-e2e.mjs --suite media
node run-room-voice-e2e.mjs --suite multi-user

# 有头调试
E2E_HEADLESS=0 node run-room-voice-e2e.mjs --suite rapid-switch --headed
```

### 10.2 环境变量

| 变量 | 默认 | 含义 |
|------|------|------|
| `BASE_URL` | `http://localhost:3000` | 前端地址 |
| `E2E_SUITE` | `all` | 套件选择 |
| `E2E_BROWSER` | `auto` | 浏览器选择策略 |
| `E2E_RAPID_ROUNDS` | `3` | 快切轮数 |
| `E2E_RAPID_DELAY_MS` | `120` | 快切间隔 |
| `E2E_FAKE_MEDIA` | `1` | Chromium 系 fake mic/cam |
| `E2E_ARTIFACT_DIR` | `scripts/artifacts/<ts>` | 截图与报告目录 |

### 10.3 CI

CI 建议按以下矩阵拆分，避免单 job 时间过长：

```text
voice-regression: join + switch + rapid-switch + media + multi-user
guild-regression: guild 套件（功能合入后）
ws-regression: ws-connect + ws-reconnect + ws-protocol + ws-concurrency（功能合入后）
full-regression: 全链路套件（仅 main / 发布前）
```

CI 必须：

- 使用独立测试数据库或可清理的 SQLite 文件。
- 提供可登录测试账号。
- 配置 `SFU_PROVIDER` 与对应凭据。
- 失败时上传 artifacts（截图、report、probe snapshot）。

## 11. 报告规范

- 路径：`agent_test_logs/room-voice-e2e-YYYY-MM-DD-HH-MM.md`
- 表头包含：环境、SFU provider、套件、浏览器、结果。
- 每套件记录：状态、耗时、通过标准是否满足。
- 失败项必须附截图路径、probe snapshot 摘要、console/network 关键错误。
- token 截断，不写密码。
- 遵循 `agent_test_logs/AGENTS.md` 的状态标识：✅ / ❌ / ⚠️ / ⏭️。

## 12. 通过标准与回归准入

一次语音 E2E 回归通过，必须满足：

- `join`：UI joined + 本地媒体发布。
- `switch`：直接切房后目标房名与媒体一致。
- `rapid-switch`：默认轮数全部无失败、无 retry。
- `media`：本地采集与会话成立。
- `multi-user`：双方互见且有远端 audio；无第二用户时明确标记跳过，不得宣称通过。
- 所有失败均有归类；环境类失败不得计为产品通过。

## 13. 扩展原则

后续新增 Guild、WS 迁移、文本消息等 E2E 套件时：

- 复用现有 `ui-helpers.mjs`、`media-probe.mjs`、`ws-helpers.mjs`。
- 套件必须定义前置条件、步骤、断言、通过标准、清理步骤。
- 任何对 provider 行为的降级判断只存在于测试资产中，不进入业务代码。
