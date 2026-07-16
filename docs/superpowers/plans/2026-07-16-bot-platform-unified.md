# GOSpeak Bot 平台统一实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan phase-by-phase, task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把 GOSpeak Bot 建成可运行的语音房机器人平台：真实后端能力出口、Socket 文本桥、事件驱动与定时主动行为、SFU 旁听 → ASR → 插件决策 → 文本/动作/语音反馈闭环。

**Architecture:** Bot Runtime（`packages/bot`）是唯一宿主。Go 后端负责身份、权限、房间、信令；Bot 进程负责插件、旁听、ASR、TTS 推流。插件只通过 `BotContext`（Capability Router）行动，禁止直连乱打 API。房内互动仅 Socket；业务资源（建房/用户/禁言记录）用既有 REST。

**Tech Stack:** Go (Gin + Socket.IO Hub), TypeScript (`@gospeak/bot`), vitest, go test, LiveKit Node SDK（旁听/推流 MVP）, 可插拔 ASR/TTS Provider

---

## 文档关系（以本文为准）

本文整合并 supersede 以下计划的执行顺序与边界；细节实现仍可回看原文档：

| 原计划 | 并入阶段 | 状态 |
|--------|----------|------|
| `2026-07-16-bot-capability-router-command-bridge.md` | Phase 0–1 | 契约以本文为准（**无 bot REST 桥**） |
| `2026-07-16-bot-sfu-listen-speech-events.md` | Phase 2 | 旁听 + Speech 事件 |
| `2026-07-16-bot-realtime-asr.md` | Phase 3 | 真 ASR Provider |
| 主动行为讨论（autoJoin / Scheduler） | Phase 1.5 / 4 | 写入本文 |
| 音频推送讨论（speak / publishPcm） | Phase 4 | 写入本文 |

**全局否决项（全文有效）：**
- 不做 `POST /api/v1/bot/command|message|kick`
- 不做完整 IM 存储/历史
- 不做把 ASR/TTS 塞进 Go 主路径
- 不在 MVP 一次实现全部 6 个 SFU 的 Node 媒体栈（LiveKit 先通）

---

## 目标架构（终态）

```text
                    ┌─────────────────────────────────────────┐
                    │              Go Server                  │
                    │  Bot Token / RBAC / Room / Mute REST    │
                    │  Signal Hub: join/leave/kick            │
                    │            bot:command / bot:message    │
                    └──────────────────┬──────────────────────┘
                                       │ JWT + Socket.IO
                                       │ (+ 业务 REST)
                    ┌──────────────────▼──────────────────────┐
                    │           Bot Runtime Host              │
                    │  Auth · PluginManager · EventBus        │
                    │  CapabilityRouter · Scheduler           │
                    │  MediaListen · ASR · AudioPublish/TTS   │
                    └──────────────────┬──────────────────────┘
                                       │ ctx.*
                    ┌──────────────────▼──────────────────────┐
                    │ Plugins: welcome / moderation /         │
                    │ voice-react / asr-caption / idle-guard  │
                    └─────────────────────────────────────────┘

听： SFU remote → PCM Hub → ASR → OnSpeech* → 插件
说： 插件 → speak/TTS → SFU local track → 房间听见
文： 插件 → ctx.chat → bot:message
做： 插件 → ctx.voice.kick/mute → room:kick / mute API
```

---

## 分阶段交付（可独立验收）

| Phase | 名称 | 验收一句话 |
|-------|------|------------|
| **0** | 契约与事件地基 | 类型/映射正确，测试不因类型崩 |
| **1** | 能力出口 + Socket 文本桥 | join 后能踢人、能收发 bot 文本、命令插件可跑 |
| **1.5** | 主动驻留 | autoJoin + 欢迎主动触发 |
| **2** | 旁听 + Speech 事件 | LiveKit 旁听出 PCM/Speech 事件（可 mock ASR） |
| **3** | 真 ASR | Final 文本驱动 caption/规则 |
| **4** | 语音反馈 + 调度主动 | speak 出声 + idle 巡检 |
| **5** | 打磨 | 文档、回归、可选多 SFU |

每完成一 Phase 必须：相关单测绿 + 手动 smoke 清单勾选 + 一次 commit 边界清晰。

---

## 统一协议与事件（全 Phase 共用）

### Socket（房内）

| 方向 | 事件 | Payload 要点 |
|------|------|----------------|
| C→S | `room:join` / `room:join:sfu` | JWT 强制 identity |
| C→S | `room:leave` | `{ room }` |
| C→S | `room:kick` | `{ room, targetIdentity }` 需 `signal:kick` |
| C→S | `bot:command` | `{ room, text }` 须在房内，text≤500 |
| C→S | `bot:message` | `{ room, content, replyTo? }` 须在房内 |
| S→C | `bot:command` / `bot:message` | 带 `from{identity,displayName,role}`, `messageId`, `timestamp` |
| S→C | `member:joined/left/updated`, `room:kicked`, `user:muted/unmuted` | 既有 |

### 业务 REST（非 bot 文本桥）

| 能力 | API |
|------|-----|
| 列表房间 | `GET /api/v1/signal/rooms` → `data: [{name,memberCount?}]` |
| 列表成员 | `GET /api/v1/signal/participants?room=` → 数组 |
| 建房 | `POST /api/v1/room/create` |
| 用户 | `POST /api/v1/user/info` `{identity}` |
| 禁言 | `POST /api/v1/mute/create|cancel|list|status` |
| SFU token | `POST /api/v1/signal/token` `{room,password?}` identity 由 JWT |

### Bot 事件（插件可见）

| EventType | 来源 |
|-----------|------|
| `AdapterMessage` / `OnMessageReceived` | `bot:command`（及可选 bot:message） |
| `OnMemberJoined` / `OnMemberLeft` / `OnMemberKicked` | member/room:kicked |
| `OnMemberStateChanged` | member:updated |
| `OnUserMuted` / `OnUserUnmuted` | user:muted/unmuted |
| `OnSpeechPartial` / `OnSpeechFinal` | ASR/SpeechPipeline |
| `OnBotLoaded` / plugin lifecycle | Runtime |

### BotContext 终态能力

```ts
ctx.chat.send/reply          // → socket bot:message
ctx.rooms.*                  // list/get/create REST + join/leave socket
ctx.voice.removeMember       // → room:kick
ctx.voice.muteMember         // → server mute（degrade）或后续 mic 策略
ctx.voice.speak/publishPcm   // Phase 4
ctx.users.getByIdentity      // REST
ctx.scheduler.every/once     // Phase 1.5/4
ctx.listen.add/remove/...    // Phase 2
ctx.kv / ctx.logger
```

### 权限白名单（Bot token）

```
room:read, room:create, user:read, signal:kick, mute:manage
```

---

## File Map（终态关键路径）

### 后端
- `app/server/internal/signal/events.go`
- `app/server/internal/signal/hub.go`
- `app/server/internal/signal/bot_bridge.go` + `_test.go`
- `app/server/internal/model/permission.go`

### Bot Runtime
- `packages/bot/src/core/types.ts` / `context.ts`
- `packages/bot/src/runtime/apiClient.ts`
- `packages/bot/src/runtime/socketClient.ts`
- `packages/bot/src/runtime/capabilityRouter.ts`
- `packages/bot/src/runtime/eventAdapter.ts`
- `packages/bot/src/runtime/scheduler.ts`
- `packages/bot/src/runtime/botRunner.ts`
- `packages/bot/src/core/pluginManager.ts`（已有热加载，保持）

### 媒体
- `packages/bot/src/media/*`（pcm、listen、publish）
- `packages/bot/src/speech/*`（pipeline、asr providers）
- `packages/bot/src/tts/*`（Phase 4）

### 插件
- 修正：`welcome` / `moderation` / `room-manager` / `mute-manager` / `keyword-reply`
- 新增：`listen-manager`、`asr-caption`、`voice-react`、`idle-guard`（可合并）

---

# Phase 0 — 事件与类型地基

**Goal:** 所有后续事件名一次定死，避免来回改插件。

### Task 0.1: 扩展 `EventType` 与 payload

**Files:**
- Modify: `packages/bot/src/core/types.ts`
- Modify: `packages/bot/src/core/index.ts`

- [ ] **Step 1: 写入完整事件枚举**

加入（保留既有）：
- `OnMemberJoined`, `OnMemberLeft`, `OnMemberKicked`
- `OnUserMuted`, `OnUserUnmuted`
- `OnSpeechPartial`, `OnSpeechFinal`
- `MessageEvent` / `RoomEvent` / `UserMuteEvent` / `SpeechEvent` 类型

`SpeechEvent` 最小字段：

```ts
{
  eventType, room, speaker, text, isFinal,
  confidence?: number, timestamp: number
}
```

- [ ] **Step 2: `pnpm test` 收集类型破坏面（允许暂时失败，记下列表）**

```bash
cd packages/bot && pnpm test
```

- [ ] **Step 3: Commit**

```bash
git add packages/bot/src/core/types.ts packages/bot/src/core/index.ts
git commit -m "feat(bot): unify event types for signal, speech, and lifecycle"
```

---

# Phase 1 — 能力出口 + Socket 文本桥（阻塞项）

**Goal:** 插件主路径真正可跑：列表/建房/禁言 REST 正确；踢人/文本仅 Socket。

### Task 1.1: 后端 `bot:command` / `bot:message`

**Files:**
- Create: `app/server/internal/signal/bot_bridge.go`
- Create: `app/server/internal/signal/bot_bridge_test.go`
- Modify: `app/server/internal/signal/events.go`, `hub.go`

- [ ] **Step 1: 失败测试** — 在房广播 / 不在房拒绝 / message+replyTo

- [ ] **Step 2: 实现 `PublishBotCommand` / `PublishBotMessage` + Socket handler**

规则：JWT identity、须在 Members、text≤500、广播带 `from`。

- [ ] **Step 3: 可选** `Hub.Kick` 抽出供 `OnRoomKick` 复用（不写 REST）

- [ ] **Step 4: 测试**

```bash
cd app/server && go test ./internal/signal -run 'Bot|Kick' -count=1
```

- [ ] **Step 5: Commit** `feat(signal): socket bot command/message bridge`

### Task 1.2: 修正 `apiClient` 业务 REST 契约

**Files:**
- Modify: `packages/bot/src/runtime/apiClient.ts`
- Create: `packages/bot/src/runtime/apiClient.test.ts`

- [ ] listRooms / getMembers 按**数组** `data` 解析  
- [ ] createRoom → `POST /room/create`  
- [ ] getSFUToken 不传伪造 identity  
- [ ] getUserByIdentity 归一化 `{id,name,role,uuid}`  
- [ ] 删除对不存在的 `/chat/send`、`/sfu/mute`、`/sfu/remove-participant` 的“假装实现”（改为 throw 或下线）  
- [ ] 测试 + commit `fix(bot): align apiClient with backend REST`

### Task 1.3: Capability Router

**Files:**
- Create: `packages/bot/src/runtime/capabilityRouter.ts` + test
- Modify: `socketClient.ts`（`sendBotMessage` / `kickMember`）
- Modify: `botRunner.ts`（注入 caps）
- Modify: `context.ts`（可选 `users`）

```ts
chat.send/reply     → socket.sendBotMessage
voice.removeMember  → socket.kickMember
voice.muteMember    → user info + mute REST（degrade 标明）
rooms.*             → api + runner join/leave
users.getByIdentity → api
```

- [ ] 单测：send→emit bot:message；removeMember→kick  
- [ ] Commit `feat(bot): capability router over signal and REST`

### Task 1.4: Event Adapter

**Files:**
- Create: `eventAdapter.ts` + test
- Modify: `socketClient.setupListeners` 统一走 adapter

映射见上文事件表；`bot:command` → `[AdapterMessage, OnMessageReceived]`。

- [ ] Commit `feat(bot): adapt socket payloads to bot events`

### Task 1.5: 权限白名单 `room:create`

**Files:** `app/server/internal/model/permission.go`

- [ ] `BotScopedPermissions` 增加 `PermRoomCreate`  
- [ ] Commit `feat(rbac): allow bots room:create`

### Task 1.6: 内置插件改线

**Files:** builtin welcome / moderation / room-manager / mute-manager + tests

- [ ] welcome → `OnMemberJoined` + `ctx.chat.send`  
- [ ] moderation `/kick` → `voice.removeMember`  
- [ ] room-manager 事件与 createRoom  
- [ ] mute-manager 走 users + mute API  
- [ ] `pnpm test` 全绿  
- [ ] Commit `fix(bot): point builtin plugins at real capabilities`

### Task 1.7: Phase 1 手动验收

```bash
# server
cd app/server && go run . server
# bot
cd packages/bot && GOSPEAK_TOKEN=... GOSPEAK_PLUGIN_DIR=./plugins pnpm start
```

清单：
1. Bot 连接成功  
2. 代码 `join("lobby")`  
3. 管理员同房 `emit bot:command {text:"/kick x"}` → 被踢  
4. welcome：他人 join → 房内 `bot:message`  
5. `/mute` 命令后 DB 有禁言记录  

- [ ] README 契约章节更新  
- [ ] Commit `docs(bot): phase1 smoke and contracts`

**Phase 1 完成定义：** 不依赖音频，命令房管插件可用。

---

# Phase 1.5 — 主动驻留（轻量）

**Goal:** Bot 自动成为房间成员，反应式主动行为可发生。

### Task 1.5.1: `autoJoinRooms`

**Files:** `botRunner.ts`, `main.ts`, `.env.example`

```ts
// BotConfig
autoJoinRooms?: string[]; // env GOSPEAK_AUTO_JOIN_ROOMS=a,b
```

`start()` 末尾：

```ts
for (const room of this.config.autoJoinRooms ?? []) {
  await this.joinRoom(room); // 先信令；SFU 旁听 Phase 2 再开
}
```

- [ ] 测试：mock socket，assert join 调用次数  
- [ ] Commit `feat(bot): auto-join rooms on start`

### Task 1.5.2: Scheduler 骨架（可先只给 runner 用）

**Files:** Create `packages/bot/src/runtime/scheduler.ts`

```ts
every(id, ms, fn); once(id, ms, fn); clear(id); clearAll();
```

- [ ] 注入 `ctx.scheduler`（ID 自动加 `pluginName:` 前缀）  
- [ ] `stop()` / 插件 `onUnload` 清理  
- [ ] Commit `feat(bot): plugin scheduler for proactive tasks`

**Phase 1.5 完成定义：** 启动后 bot 已在目标房；welcome 无需手 join。

---

# Phase 2 — SFU 旁听 + Speech 事件

**Goal:** 指定房间旁听，PCM 入进程总线，可发出 Speech 事件（mock pipeline 可先）。

详细类型与 adapter 形状对齐 `bot-sfu-listen-speech-events.md`，任务压缩如下：

### Task 2.1: ListenRoomRegistry

- desired rooms 集合；add/remove/list/clear；变更回调  
- 来源优先级：运行时命令 > `BotConfig.listenRooms` > `GOSPEAK_LISTEN_ROOMS`

### Task 2.2: SFUListenAdapter 接口 + Router

```ts
join/leave/onAudioFrame/dispose
```

LiveKit 实现可运行；其他 provider → clear `not implemented`。

### Task 2.3: MediaListenService

- reconcile desired vs active  
- join：信令 join + token + listen adapter  
- leave：停旁听 + 可选 leave 信令  
- 帧写入 `runner.pcmHub`（**进程内**，不是推 SFU）

### Task 2.4: SpeechPipeline（mock）+ Bus 桥

- 消费 PCM，mock 可按静音阈值吐 fake final，或 passthrough 计数  
- `speechBusBridge` → `OnSpeechPartial/Final`

### Task 2.5: BotRunner 接线 + listen-manager 插件

- `/listen add|remove|list|clear`（依赖 Phase 1 文本桥）  
- `ctx.listen` 可选暴露 registry 操作  

### Task 2.6: 验收

- LiveKit 房内有人说话 → bot 日志出现 frame 或 mock speech 事件  
- `/listen add room` 热更新旁听集合  

- [ ] Commit 系列：`feat(bot): media listen ...` 按子任务拆 commit

**Phase 2 完成定义：** 旁听链路通；Speech 事件可被插件订阅（内容可 mock）。

---

# Phase 3 — 真 ASR

**Goal:** PCM → 流式识别 → 可靠 `OnSpeechFinal`。

对齐 `bot-realtime-asr.md`，压缩任务：

### Task 3.1: ASRProvider 抽象

```ts
createSession({room, identity}) → write(frame)/end()
onPartial / onFinal callbacks
```

### Task 3.2: ASRManager

- 按 `room+identity` 分轨会话  
- track end / leave 时 end session  
- **忽略 bot 自身 identity**（防自激预备）

### Task 3.3: Providers（按优先级）

1. `LocalHttp/WebSocket`（FunASR 等自建）  
2. Deepgram  
3. Azure / 阿里云（接口位 + 至少一个可配）

### Task 3.4: Factory + env 配置

```
GOSPEAK_ASR_PROVIDER=deepgram|local|...
GOSPEAK_ASR_* 密钥与 endpoint
```

### Task 3.5: asr-caption 插件

- 订阅 `OnSpeechFinal`  
- 默认只 log；配置 `broadcast: true` 时 `ctx.chat.send` 发字幕（注意刷屏与冷却）

### Task 3.6: 验收

- 真人说话 → Final 文本正确率可接受  
- caption 可选广播  
- 断流/重连不泄漏 session  

**Phase 3 完成定义：** 语音→文本事件稳定，可供规则插件使用。

---

# Phase 4 — 语音反馈 + 主动巡检

**Goal:** 听完能“说出来”；定时策略能主动作为。

### Task 4.1: SFUPublishAdapter + LiveKit 上行

**Files:** `packages/bot/src/media/publish/*`

```ts
join / publishPcm / unpublish / leave
```

- Bot token 需 canPublish（核对 `GetJoinToken` 对 bot 的 grant）  
- 测试音：1s beep PCM 房间可闻  

### Task 4.2: `ctx.voice.publishPcm` / `speak` / `stopSpeaking`

- TTSProvider 接口：`synthesize(text) → PCM`  
- MVP：本地 HTTP TTS 或占位 sine+后接真 TTS  
- **每房播放队列**；`interrupt` 可抢占  

### Task 4.3: 防自激

- 旁听/ASR 过滤 `identity === botIdentity`  
- speak 期间可选暂停该房 ASR 写入  

### Task 4.4: voice-react 插件

```ts
@On(OnSpeechFinal)
// 唤醒词 → speak 回复
// 违规词 → chat 警告 + 可选 kick
// 语音命令「踢出 xxx」→ removeMember + speak 确认
```

冷却：同 identity 5–60s 去重。

### Task 4.5: idle-guard 插件（Scheduler）

- `every 30s` 扫 joined 房间  
- 结合 mute/speaking 状态（有则用事件缓存，无则温和策略）  
- 默认 `warnOnly: true`；配置才 autoKick  

### Task 4.6: 验收

1. `speak("测试")` 房内听见  
2. 对人说唤醒词 → bot 语音回复  
3. 巡检警告文本或语音出现一次（有冷却）  
4. bot 自己的 speak 不触发二次 ASR 动作  

**Phase 4 完成定义：** 听→想→说/做闭环成立。

---

# Phase 5 — 打磨与扩展

### Task 5.1: 统一 README

`packages/bot/README.md` 单页说明：
- 架构图  
- 权限与创建 bot  
- autoJoin / listenRooms / ASR / TTS env  
- 插件编写  
- 明确无 bot REST 文本桥  

### Task 5.2: 回归矩阵

```bash
cd app/server && go test ./internal/signal ./internal/service -count=1
cd packages/bot && pnpm test
```

手动：Phase 1–4 smoke 各跑一遍。

### Task 5.3（可选）：更多 SFU listen/publish adapter

- MediaSoup / SRS / … 按同一接口扩展  
- 不阻塞主线合并  

### Task 5.4（可选）：Runtime 管理 API

- 与 Go `/plugins/*` 是否统一另开计划  
- 热加载本地 `pluginDir` 已可用，本阶段不强制后端管控  

---

## 跨 Phase 不变式（实现时天天对照）

1. **插件不 `fetch` 任意路径** — 只 `ctx.*`  
2. **房内互动只 Socket** — command/message/kick  
3. **pcmHub.publish ≠ 推送到 SFU** — 进程内总线  
4. **推送到 SFU 必须走 PublishAdapter**  
5. **Bot 先信令在场，再谈收发与媒体**  
6. **高危动作用 Final + 冷却 + 后端权限双重约束**  
7. **忽略自身 identity，防听自己说话**  

---

## 建议 Commit 节奏

- 每 Task 1 commit（或同 Phase 内逻辑紧密的 2–3 task 一组）  
- Phase 边界打 tag 可选：`bot-phase-1` …  
- 不混进无关 SFU/NATS 重构  

---

## 测试策略总表

| 层级 | 内容 |
|------|------|
| Go unit | bot_bridge 广播/拒收；kick 权限回归 |
| Bot unit | apiClient 契约；capabilityRouter；eventAdapter；scheduler；registry reconcile |
| Bot 插件 | welcome/moderation/voice-react 用假 ctx |
| 集成手动 | 真 server + 真/mock LiveKit + 真 ASR 可选 |
| 回归 | `pnpm test` + `go test ./internal/signal` |

---

## 实施顺序（Agent 默认）

```text
Phase 0 → 1（全部）→ 1.5 → 2 → 3 → 4 → 5
```

若资源紧：

```text
必须：0 → 1 → 1.5
高价值：2 → 3
体验闭环：4
```

**禁止并行乱序：** 没有 Phase 1 的文本桥/踢人，做 ASR 字幕与语音命令会空转。

---

## Self-Review

### 覆盖
- 能力路由 / Socket 桥 / 事件 → Phase 0–1  
- 主动 join / 定时 → Phase 1.5、4.5  
- 旁听 Speech → Phase 2  
- 真 ASR → Phase 3  
- 推音频反馈 → Phase 4  
- 文档回归 → Phase 5  

### 边界
- 明确无 bot REST 桥  
- 明确 pcm 进程内 vs SFU 上行  
- LiveKit MVP，多 SFU 可选  

### 依赖
- Phase 4 speak 依赖 1（在场）+ 2/3（若要语音交互）  
- voice-react 可先只绑 Final mock，再换真 ASR  

---

## 执行方式

1. **Subagent-Driven（推荐）**：按 Task 开子代理，Phase 末人工验收  
2. **Inline Execution**：本会话连续推进，每 Phase 停检  

**建议第一刀：** Phase 0 Task 0.1 → Phase 1 Task 1.1（后端文本桥）。

---

## 原计划处置

- 保留原三份文档作细节附录，文首加一行：  
  `> Superseded for execution order by 2026-07-16-bot-platform-unified.md`  
- 新工作只改本文任务状态勾选，避免三份计划并行漂移  

