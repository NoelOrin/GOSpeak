---
name: room-voice-e2e
description: Automate GOSpeak voice-room end-to-end checks for join, room switch, rapid room switch, publish/subscribe media health, and local multi-user audio. Use when testing 进入房间, 切换房间, 快速切房, 推拉流, 多人房间音频, voice chat regressions, SFU media sessions, or when the user asks for Playwright/computer-use room voice QA.
---

# Room Voice E2E

为 GOSpeak 语音房间做可重复的自动化验收。默认优先 Playwright；需要真实窗口/听感时改用 computer-use。

## 默认选择

| 场景 | 工具 |
|------|------|
| CI/可重复回归、双用户、快切 | **Playwright**（`scripts/run-room-voice-e2e.mjs`） |
| 桌面真浏览器、权限弹窗、真实听感 | **computer-use** |
| 仅 API/信令、无 UI | 不要用本 skill；走后端/API 测试 |

先读：

1. `references/scenarios.md` — 套件定义与通过标准
2. `references/selectors.md` — UI 选择器
3. `references/media-assertions.md` — 推拉流判定
4. 仓库 `.agents/skills/test-logging/SKILL.md` — 报告落盘

## 前置检查

执行前确认：

```bash
# 后端健康
curl -sS http://localhost:8998/ping

# 前端（Vite 默认 3000，反代 /api 与 socket.io）
curl -sS -o /dev/null -w "%{http_code}\n" http://localhost:3000/login
```

若未启动：

```bash
# 后端
cd app/server && go run . server

# 前端
pnpm --filter @gospeak/web dev
```

准备账号：

- `E2E_USER` / `E2E_PASS`（必填）
- `E2E_USER_B` / `E2E_PASS_B`（多人套件必填）
- 避免默认 `admin/admin123` 首登强制改密态

## 浏览器选择

默认 **系统默认浏览器优先**，不写死 Edge/Chrome。

解析顺序：

1. `E2E_BROWSER` / `--browser` 显式指定（若给了且不是 `auto|system`）
2. 读取 OS 默认浏览器（macOS LaunchServices / Linux xdg / Windows UserChoice）
3. Fallback 链：`chrome → chromium → msedge → firefox → webkit`（跳过已尝试项；中立顺序，不把 Edge 放第一）

说明：

- 你机器默认是 Edge 时，第 2 步会解析到 `msedge`，这是系统配置结果，不是代码写死
- 系统默认不可用时自动 fallback，并在日志/报告标记 `fallback`
- Chromium 系才注入 fake media 启动参数；Firefox/WebKit 自动跳过该参数

```bash
# 默认：系统浏览器 + fallback
node run-room-voice-e2e.mjs --suite join

# 显式指定（仍会在失败时 fallback）
E2E_BROWSER=chrome node run-room-voice-e2e.mjs --suite join
node run-room-voice-e2e.mjs --browser firefox --suite media
```

## Playwright 执行（首选）

脚本目录：`.agents/skills/room-voice-e2e/scripts/`

```bash
cd .agents/skills/room-voice-e2e/scripts
pnpm install   # 首次

# 全量
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

环境变量：

| 变量 | 默认 | 含义 |
|------|------|------|
| `BASE_URL` | `http://localhost:3000` | 前端地址 |
| `E2E_SUITE` | `all` | `join/switch/rapid-switch/media/multi-user/all` |
| `E2E_BROWSER` | `auto` | `auto/system`=系统默认优先；也可 `msedge/chrome/chromium/firefox/webkit` |
| `E2E_RAPID_ROUNDS` | `3` | 快切轮数 |
| `E2E_RAPID_DELAY_MS` | `120` | 快切间隔 |
| `E2E_FAKE_MEDIA` | `1` | Chromium 系 fake mic/cam（非 Chromium 自动忽略） |
| `E2E_ARTIFACT_DIR` | `scripts/artifacts/<ts>` | 截图与 report |

产物：

- `report.md` / `report.json`
- 失败截图 `*-failure.png`

依赖：`playwright`（脚本 `package.json`）。

- 系统 Edge/Chrome：走 Playwright channel，一般无需 `playwright install`
- 若 fallback 到 Playwright 自带浏览器才需要：

```bash
pnpm exec playwright install chromium firefox webkit
```

## 套件速查

| Suite | 做什么 | 通过线 |
|-------|--------|--------|
| `join` | 创建房间 → 双击进入 | `离开` 可见 + media ready |
| `switch` | A 进房后直接切 B | 稳定停在 B，媒体未崩 |
| `rapid-switch` | A/B 快速来回 N 轮 | 每轮 join 成功，无 retry |
| `media` | 单人推流/会话 | getUserMedia + PC/local track |
| `multi-user` | 两用户同房 | 双方 ≥2 人在线 + 各自 remote audio |

先进入域工作区 `/domain/:domainUUID`。房间进入方式：**双击**左侧房间行（不是单击）；创建按钮是 `title="新建房间"` 的图标按钮。切房通常**不必先点离开**。

## Computer-use 流程

当用户明确要求 computer-use，或 Playwright 无法覆盖真实权限/听感时：

1. 打开 `BASE_URL/login`，登录 userA
2. 创建 `e2e-cu-*` 房间，双击进入，确认 `离开` 与人数
3. 再开一窗口登录 userB，加入同一房间
4. 验证：
   - 双方成员列表互见
   - 一方出声时另一方有 speaking/可听（若环境允许）
5. 切房/快切：在 A/B 两房间间连续双击，观察是否卡在 `连接媒体...` 或 `加入失败`
6. 截图关键状态，写入 `agent_test_logs/`

详细选择器见 `references/selectors.md`。

## 媒体判定原则

不要只靠“页面没报错”。按优先级：

1. **UI joined**：目标房名 + `离开` + 无 `重试`
2. **本地推流**：getUserMedia / local audio track / RTCPeerConnection
3. **远端拉流**（多人）：DOM 中带 live audio track 的 `audio/video[srcObject]`
4. **快切稳定性**：每次切换都重新达到 1+2

探针实现：`scripts/media-probe.mjs`（注入 `window.__gospeakMediaProbe`）。

provider 差异与降级策略见 `references/media-assertions.md`。

## Agent 工作流

1. 确认前后端与 SFU 可用
2. 选工具：默认 Playwright；听感/真窗才 computer-use
3. 跑请求的 suite（未指定则 `all`，无第二账号则跳过 `multi-user` 并在报告标明）
4. 失败时收集：截图、probe snapshot、浏览器 console/network 关键错误、当前 `SFU_PROVIDER`
5. 按 `test-logging` 写中文报告到 `agent_test_logs/room-voice-e2e-YYYY-MM-DD-HH-MM.md`
6. 报告必须含：环境、套件表、失败根因归类（信令/进房编排/推流/拉流/快切竞态）

## 失败归类（写进报告）

| 归类 | 典型现象 |
|------|----------|
| 环境 | 前端/后端/SFU 未起，账号改密拦截 |
| 信令 | 房间列表空、成员互不可见 |
| 进房编排 | 长期 loading、token 失败 |
| 推流 | gum 无调用、无 local track/PC |
| 拉流 | 成员可见但无 remote audio |
| 切房竞态 | 快切后卡死、房名与媒体不一致 |

## 不要做的事

- 不要删除用户数据库或生产房间数据；只用 `e2e-*` 前缀临时房间
- 不要在报告写完整密码/长 token
- 不要把 provider 特判写进业务代码；本 skill 只做验证
- 不要在无第二用户时宣称“多人拉流通过”

## 脚本索引

| 文件 | 用途 |
|------|------|
| `scripts/run-room-voice-e2e.mjs` | 主 runner |
| `scripts/ui-helpers.mjs` | 登录/建房/进房/切房 |
| `scripts/media-probe.mjs` | 媒体探针 |
| `scripts/package.json` | 本地 playwright 依赖 |
