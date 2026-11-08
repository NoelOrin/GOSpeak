## LiveKit Provider 实现完整度

**评估时间**: 2026-08-03（已对照 `app/server/internal/sfu/providers/livekit/` 源码核实）
**接口**: `sfu.Provider`（[provider.go](/Users/noelorin/GOSpeak/app/server/internal/sfu/provider.go)）

> 图例：✅ 完整 / ⚠️ 降级 / ❌ 返回 `ErrSFUNotSupported` 或静默空操作

### 1. 整体成熟度概览

| Provider | 成熟度 | 接口方法覆盖 | 关键缺口 |
|----------|--------|--------------|----------|
| LiveKit | 完整 | 8/8 ✅ | 无 |

### 2. 方法覆盖矩阵

| 方法 | LiveKit |
|------|---------|
| `GenerateToken` | ✅ |
| `GenerateAdminToken` | ✅ |
| `ListRooms` | ✅ |
| `ListParticipants` | ✅ |
| `MuteParticipant` | ✅ |
| `RemoveParticipant` | ✅ |
| `DeleteRoom` | ✅ |
| `GetHost` | ✅ |

### 3. 实现说明

| Provider | 路径 | 关键事实 |
|----------|------|----------|
| LiveKit | `internal/sfu/providers/livekit/` | 全部 ✅；`MuteParticipant` 支持按 trackSid 精确静音或按 identity 批量静音；`ProviderName()` = `livekit` |

### 4. 前端 SFU 客户端（`packages/sfu-client`）

LiveKit 客户端已完整实现，无已知标记。通过 `livekit-client.ts` 接入，按房间/参与者订阅媒体与事件。

### 5. 发言检测（Active Speakers）

| Provider | 检测机制 | 链路 | 能力 |
|----------|----------|------|------|
| LiveKit | SFU 原生 `RoomEvent.ActiveSpeakersChanged` | 客户端 `onActiveSpeakers` → 前端高亮 | ✅ 原生 |

**信令聚合链路要点**：
- 新成员 `room:join:sfu` 成功后服务端回放当前 active speakers，加入者立即看到正在发言的人
- 成员离开 / 断连 / 被踢 / 切房时清发言态并广播最新列表重置高亮
- 前端离开 / 断连 / 切房时清空 `speakingIdentities`，避免陈旧高亮残留
- 多实例部署下，新成员 join 回放仅包含本实例成员发言态；跨实例发言以实时事件广播为准

### 6. 关于 Mute 语义的说明

SFU `MuteParticipant` 是 **服务端轨道级 SFU mute**，而非用户禁言。

用户禁言由独立层实现：
- `MuteService` + `MuteHandler` + `Hub.BroadcastMute` / `Hub.BroadcastUnmute`
- WebSocket 事件：`user:muted` / `user:unmuted`（被禁言者本人）；`member:muted` / `member:unmuted`（全员订阅端静音）
- 数据库中持久化禁言记录，含过期自动清理
- 该层 **已完整实现且功能齐全**

本地播放静音（静音远端音轨音量）是纯客户端行为，与以上两层无关。

### 7. 信令层踢人分发（按 `ErrSFUNotSupported` 优雅降级）

`internal/signal/hub.go` 的 `removeParticipantSafe` 直接调用 `sfuProvider.RemoveParticipant(room, identity)`，仅当返回 `pkg.ErrSFUNotSupported` 时静默跳过。LiveKit 已实现并被调用。

### Enforcement 事件语义

`room:kicked` / `user:muted` / `user:unmuted` 携带 `enforcement`:
- `hard`：原生媒体强制成功
- `degraded`：替代 API 强制成功
- `soft`：仅信令/策略 + 客户端配合
