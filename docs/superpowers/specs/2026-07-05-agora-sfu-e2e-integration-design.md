# Agora SFU 端到端联调 Design

**日期**: 2026-07-05
**分支**: `feature/agora-sfu`
**状态**: 已确认，待写实现计划

## 1. 目标

对 Agora SFU 后端 provider + 前端 client 做端到端真实联调，验证核心音频房链路在真实 Agora 云上跑通，暴露并修复集成问题。

**范围外（YAGNI）**:
- 不实现 mute REST 修复（maturity doc 标记的 `MuteParticipant`/`MuteRoomParticipant` 静默 nil），除非阻塞核心链路
- 不加服务端录制（方案 B）
- 不做多机跨网（方案 C）

## 2. 核心约束：抽象层统一处理

Agora 集成必须走 `sfu.Provider` 抽象层，**禁止在 handler/service 泄漏 provider 分支**（CLAUDE.md 已有原则）。

已验证现状合规：

| 层 | 调用 | 是否泄漏 agora 分支 |
|----|------|---------------------|
| `signal_handler.go:GetJoinToken` | `h.sfuProvider.GenerateToken()` + `GetHost()` + `ClientInfo()` | 否，全走接口 |
| `signal_handler.go:ListRooms/ListParticipants` | `h.sfuProvider.ListRooms()` 等 | 否 |
| `sfu/factory.go` | `switch name` 实例化 | 仅工厂，允许 |
| 前端 `useRoomJoinSession` | `loadSfuClient(provider)` + `joinRoom(token, connectTarget, ...)` | 否，provider-agnostic |
| 前端 `agora-client.ts` | `AgoraRTC` SDK | 封装在 client 内，不外泄 |

`ClientInfo()` 机制是抽象层的正确扩展点：agora provider 返回 `{appId}`，signal handler 透传到前端，前端 `resolveJoinSession` 解析为 `connectTarget`。**无需新增 provider 分支**。

联调中若发现需要 Agora 特殊处理，必须内聚到 `internal/agora/` 或 `agora-client.ts`，不得外泄到 handler/service/room 组件。

## 3. 现状基线

### 后端 `internal/agora/`
- `provider.go` — `Service` 实现 `sfu.Provider`：token/list/listParticipants/delete/getHost/clientInfo 完整
- `rtc_token.go` — `rtctokenbuilder2.BuildTokenWithUserAccount`，TTL 3600s，RolePublisher
- `rest.go` — Customer ID/Secret Basic Auth，list channels / channel users / delete channel
- 已知缺口（maturity doc）：
  - `MuteParticipant` / `MuteRoomParticipant` 静默返回 nil（应改 `ErrNotSupported`，本次不修）
  - `GenerateAdminToken` 返回空串
  - `RemoveParticipant` 显式 `ErrSFUNotSupported`（kicking-rule 是 ban 语义，与踢出不符）

### 前端 `packages/sfu-client/src/agora-client.ts`
- `AgoraSFUClient` 实现 `SFUClient` 接口
- `joinRoom`: `createClient({mode:"rtc", codec:"vp8"})` + `enableAudioVolumeIndicator` + `join` + `createMicrophoneAudioTrack` + `publish`
- 事件: `user-published`/`user-unpublished`/`volume-indicator`/`connection-state-change`
- 重连: `hasJoined` 守卫 + `prevConnState` 跟踪，`RECONNECTING`→`onReconnecting`，`CONNECTED`(prev=RECONNECTING)→`onReconnected`，`DISCONNECTED`→`onDisconnected`
- `getExistingRemoteAudioTracks` 补 join 竞态
- `agora-rtc-sdk-ng@^4.24.5` 已装

### 链路完整性
- signal handler 返回 `{token, serverUrl, provider:"agora", appId}` — 前端零配置可 join
- `app/web/.env` 的 `VITE_AGORA_APP_ID` 仅 fallback，非必需
- Agora 是云 SFU，无需本地 docker（livekit/redis/minio 可不启；Redis 缺失走静态 JWT 降级）

## 4. 执行步骤

### 4.1 凭据配置（用户执行，敏感不提交 git）

`app/server/.env.dev`:
```
SFU_PROVIDER=agora
AGORA_APP_ID=<console Project Management 复制>
AGORA_APP_CERTIFICATE=<同项目页，需先启用，5min 生效>
AGORA_CUSTOMER_ID=<Account Profile>
AGORA_CUSTOMER_SECRET=<同页 View>
```

`app/web/.env`:
```
VITE_SFU_PROVIDER=agora
VITE_AGORA_APP_ID=<同 AGORA_APP_ID>
```

凭据获取指引已在前置对话提供（Agora Console → Project Management / Account Profile）。

### 4.2 启动
```bash
pnpm start:dev   # air 后端热重载 + vite 前端
```
无需 `docker compose`。

### 4.3 测试用户与房间
- `POST /api/v1/auth/register` 注册 2 个用户（或走 web 登录页）
- 用户 A 通过 web 建房（room API）

### 4.4 双 tab 联调
- A: 普通窗口
- B: 隐身窗口 / 不同浏览器
- 同进一房，执行验证清单

## 5. 验证清单

| # | 场景 | 验证点 | 通过判据 |
|---|------|--------|----------|
| 1 | join/publish/subscribe | A 说 B 听到，B 说 A 听到 | 双向音频可达 |
| 2 | leave | A leave → B 收 `user-unpublished` + 远端音轨移除 | 事件触发 + UI 反馈 |
| 3 | 重连 | B 断网 → `onReconnecting` toast → 恢复 `onReconnected` toast | 状态机正确 |
| 4 | active-speaker | 说话方 uid 上报，静默方不上报 | `volume-indicator` 阈值 `level>5` 合理 |
| 5 | 后端 ListRooms | `GET /api/v1/sfu/rooms` | 走 Customer ID/Secret Basic Auth 返回 channel 列表 |
| 6 | 后端 ListParticipants | `GET /api/v1/sfu/rooms/:room/participants` | 返回 channel users |
| 7 | 后端 DeleteRoom | `DELETE /api/v1/sfu/rooms/:room` | channel 删除成功 |

## 6. 预期发现（联调中可能要改）

| 项 | 风险 | 处理 |
|----|------|------|
| mute 走哪条路 | UI mute 按钮若走后端 SFU mute → Agora 静默 nil 不生效 | 联调中确认；若走 `setMicEnabled`（本地轨道）则 OK |
| active-speaker threshold | `level > 5` 经验值 | 实测调整 |
| 重连瞬态 | join 阶段 `DISCONNECTED` 是否误触发 `onDisconnected` | `hasJoined` 守卫已防，观察确认 |
| `user-published` 后无声 | `setVolume` 0-100 缩放 / `autoplay` 策略 | 排查 |

修复原则：所有改动内聚到 `internal/agora/` 或 `agora-client.ts`，不外泄 provider 分支。

## 7. 错误处理预案

| 症状 | 排查 |
|------|------|
| token 生成失败 | App Certificate 是否启用 + 5min 生效 |
| join `INVALID_TOKEN` | 后端 App ID 与前端 appId 是否一致（透传链路） |
| REST 401 | Customer ID/Secret 错 |
| 浏览器无麦克风 | localhost 权限，或需 HTTPS |
| `user-published` 后无声 | `setVolume` 缩放或 `autoplay` 策略 |

## 8. 测试证据

按 `CLAUDE.md` 规范存 `agent_test_logs/agora-e2e-{时间}.md`：
- 每项验证结果（通过/失败/部分）
- 后端 + 前端日志摘录
- 发现的问题清单 + 修复建议
- 凭据脱敏

## 9. 验收标准

- 验证清单 1-4 全通过（核心音频链路）
- 验证清单 5-7 至少 1 项通过（后端管理 API 走通 Customer ID/Secret）
- 测试日志归档 `agent_test_logs/`
- 联调中发现的集成问题已修复或已记录到 maturity doc
