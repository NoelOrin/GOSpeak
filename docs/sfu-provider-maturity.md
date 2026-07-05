## SFU Provider 实现完整度矩阵

**评估时间**: 2026-07-02
**接口**: `sfu.Provider`（[provider.go](/Users/noelorin/GOSpeak/app/server/internal/sfu/provider.go)）

### 方法覆盖

| 方法 | LiveKit | Agora | MediaSoup | SRS | Daily |
|------|---------|-------|-----------|-----|-------|
| `GenerateToken` | ✅ | ✅ | ✅ | ✅ | ✅ |
| `GenerateAdminToken` | ✅ | ⚠️ | ⚠️ | ✅ | ⚠️ |
| `ListRooms` | ✅ | ✅ | ✅ | ✅ | ✅ |
| `ListParticipants` | ✅ | ✅ | ❌ | ❌ | ✅ |
| `MuteParticipant` | ✅ | ❌ | ❌ | ❌ | ❌ |
| `MuteRoomParticipant` | ✅ | ❌ | ❌ | ❌ | ❌ |
| `RemoveParticipant` | ✅ | ❌ | ❌ | ✅ | ❌ |
| `DeleteRoom` | ✅ | ✅ | ✅ | ✅ | ✅ |
| `GetHost` | ✅ | ✅ | ✅ | ✅ | ✅ |

图例：✅ 完整 / ⚠️ 降级 / ❌ 返回 `ErrNotSupported` 或静默空操作

### 详细缺口

#### Agora — `internal/agora/provider.go`

| 方法 | 严重程度 | 当前行为 | 应改为 |
|------|----------|----------|--------|
| `MuteParticipant` | 高 | 静默返回 nil，不执行任何操作 | 返回 `ErrNotSupported`，与其他 provider 保持一致 |
| `MuteRoomParticipant` | 高 | 静默返回 nil | 同上 |
| `GenerateAdminToken` | 中 | 返回空字符串 `""` | 返回 token 或 `AppError` |
| `RemoveParticipant` | 低 | 返回显式错："requires a dedicated REST implementation" | 已有适当报错，需实现 Agora REST 踢人接口 |

#### MediaSoup — `internal/mediasoup/provider.go`

| 方法 | 严重程度 | 当前行为 | 说明 |
|------|----------|----------|------|
| `ListParticipants` | 中 | `notSupportedErr()` | Mediasoup bridge 无参与者列表端点 |
| `MuteParticipant` | 中 | `notSupportedErr()` | 已正确返回错误 |
| `MuteRoomParticipant` | 中 | `notSupportedErr()` | 已正确返回错误 |
| `RemoveParticipant` | 中 | `notSupportedErr()` | 已正确返回错误 |

MediaSoup 拥有自己的信令路径（[signal.go](/Users/noelorin/GOSpeak/app/server/internal/mediasoup/signal.go)），通过 Socket.IO 提供：
`sfu:get-router-capabilities`、`sfu:create-transport`、`sfu:connect-transport`、`sfu:produce`、`sfu:consume`。
Provider 方法返回 `ErrNotSupported` 是合理设计——Mediasoup 的参与者管理需通过其他路径。

#### SRS — `internal/srs/provider.go`

| 方法 | 严重程度 | 当前行为 |
|------|----------|----------|
| `ListParticipants` | 中 | `notSupportedErr()` |
| `MuteParticipant` | 中 | `notSupportedErr()` |
| `MuteRoomParticipant` | 中 | `notSupportedErr()` |

SRS WHIP 不支持参与者列表和轨道静音。`RemoveParticipant` 已实现（kick），`DeleteRoom` 已实现。

> 自部署 e2e 已验证(2026-07-05): docker compose + WHIP/WHEP 双向音频通,runbook 见 `docs/srs-selfhost-runbook.md`。

#### Daily — `internal/daily/provider.go`

| 方法 | 严重程度 | 当前行为 |
|------|----------|----------|
| `MuteParticipant` | 中 | `notSupportedErr()` |
| `MuteRoomParticipant` | 中 | `notSupportedErr()` |
| `RemoveParticipant` | 中 | `notSupportedErr()` |

Daily API 权限模型可能不支持服务端 mute/kick。`GenerateAdminToken` 降级调用 `GenerateToken("admin","admin")`，可接受但不够干净。

#### LiveKit — `internal/livekit/client.go`

唯一完整实现所有 9 个方法的 provider。`MuteRoomParticipant` 通过先查询参与者、再逐个 mute track 的方式实现。

### 前端 SFU 客户端（`packages/sfu-client`）

五端均已实现（LiveKit、Agora、MediaSoup、SRS、Daily）。已知标记：

| 文件 | 问题 |
|------|------|
| [mediasoup-client.ts:271](/Users/noelorin/GOSpeak/packages/sfu-client/src/mediasoup-client.ts:271) | `FIXME`：活跃发言者回退为列出全部远端音轨，非实际检测 |

### 关于 Mute 语义的说明

SFU `MuteParticipant` / `MuteRoomParticipant` 是 **服务端轨道级 SFU mute**，而非用户禁言。

用户禁言由独立层实现：
- `MuteService` + `MuteHandler` + `Hub.BroadcastMute` / `Hub.BroadcastUnmute`
- WebSocket 事件：`user:muted` / `user:unmuted`
- 数据库中持久化禁言记录，含过期自动清理
- 该层 **已完整实现且功能齐全**

本地播放静音（静音远端音轨音量）是纯客户端行为，与以上两层无关。

