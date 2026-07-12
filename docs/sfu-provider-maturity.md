## SFU Provider 实现完整度矩阵

**评估时间**: 2026-07-12（已逐 provider 对照 `app/server/internal/*/provider.go` 源码核实）
**接口**: `sfu.Provider`（[provider.go](/Users/noelorin/GOSpeak/app/server/internal/sfu/provider.go)）

> 图例：✅ 完整 / ⚠️ 降级 / ❌ 返回 `ErrSFUNotSupported` 或静默空操作　|　★ = 本轮回填/修正记录（源码早已实现，历史文档误标）

### 1. 整体成熟度概览

| Provider | 成熟度 | 接口方法覆盖 | 关键缺口 | 补全实现（★） |
|----------|--------|--------------|----------|----------------|
| LiveKit | 完整 | 8/8 ✅ | 无 | — |
| MediaSoup | 完整 | 8/8（token 降级） | token 为 `room:identity` 内部约定，非真实 JWT | `RemoveParticipant` ★ |
| SRS | 高 | 7/8 | `MuteParticipant` ❌ | — |
| Daily | 中–高 | 7/8 | `MuteParticipant` ❌ | `RemoveParticipant` ★ |
| Agora | 中 | 5/8 | `MuteParticipant`/`RemoveParticipant` ❌，`GenerateAdminToken` ⚠️ | — |
| Cloudflare | 低–中 | 6/8 | `List*` 仅进程内内存、`MuteParticipant` ❌、token 为 JSON 配置块 | — |

### 2. 方法覆盖矩阵

| 方法 | LiveKit | Agora | MediaSoup | SRS | Daily | Cloudflare |
|------|---------|-------|-----------|-----|-------|------------|
| `GenerateToken` | ✅ | ✅ | ⚠️ | ✅ | ✅ | ⚠️ |
| `GenerateAdminToken` | ✅ | ⚠️ | ⚠️ | ✅ | ⚠️ | ✅ |
| `ListRooms` | ✅ | ✅ | ✅ | ✅ | ✅ | ⚠️ |
| `ListParticipants` | ✅ | ✅ | ✅ | ✅ | ✅ | ⚠️ |
| `MuteParticipant` | ✅ | ❌ | ✅ | ❌ | ❌ | ❌ |
| `MuteRoomParticipant` † | ✅ | ❌ | ✅ | ❌ | ❌ | ❌ |
| `RemoveParticipant` | ✅ | ❌ | ✅ ★ | ✅ | ✅ ★ | ✅ |
| `DeleteRoom` | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| `GetHost` | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |

- † `MuteRoomParticipant` 为历史方法名，当前 `sfu.Provider` 接口已不再包含；上表 LiveKit / MediaSoup 的 ✅ 为历史实现记录，新 provider 均按 `MuteParticipant` 统一语义处理（用户禁言见第 5 节）。
- ★ MediaSoup / Daily 的 `RemoveParticipant` 在源码中早已实现（bridge `CloseParticipant` / list→session id remove），历史 dispatch 文档误标为「跳过」，本次修正；Hub 现已按 `ErrSFUNotSupported` 优雅降级，实现即生效。

### 3. 缺口与补全（统一表）

| Provider | 方法 | 严重程度 | 当前行为 | 建议 / 状态 |
|----------|------|----------|----------|-------------|
| Agora | `MuteParticipant` | 高 | 静默返回 nil（无副作用） | 建议返回 `ErrSFUNotSupported`，与 SRS/Daily/Cloudflare 一致，便于 Hub 统一识别 |
| Agora | `GenerateAdminToken` | 中 | 返回空字符串 `""` | 返回真实 token 或 `AppError` |
| Agora | `RemoveParticipant` | 低 | 返回 `SFU_ERROR` + `ErrSFUNotSupported` cause（无单用户踢人 REST API） | Hub 已优雅降级，依赖频道离会后自动回收 |
| MediaSoup | `GenerateToken` | 中 | 返回 `room:identity` 内部约定，非真实 JWT | 设计如此，可接受 |
| MediaSoup | `GenerateAdminToken` | 中 | 返回 `mediasoup-admin` | 设计如此，可接受 |
| SRS | `MuteParticipant` | 中 | `ErrSFUNotSupported`（无服务端轨道静音） | 前端停推/关 track 兜底 |
| Daily | `MuteParticipant` | 中 | `ErrSFUNotSupported` | 前端停推兜底 |
| Daily | `GenerateAdminToken` | 低 | 降级调用 `GenerateToken("admin","admin")` | 可接受但不够干净 |
| Cloudflare | `GenerateToken` | 中 | 返回 JSON 配置块（sessionId/appId/stunUrl），非真实鉴权 token | 设计如此（无原生 token 体系） |
| Cloudflare | `ListRooms` | 中 | 仅进程内 `sessions` map（room→identity→sessionId） | 非跨实例权威，进程重启/多实例不同步 |
| Cloudflare | `ListParticipants` | 中 | 仅进程内 map 的 identity 集合，`JoinedAt` 为当前时间 | 非跨实例权威 |
| Cloudflare | `MuteParticipant` | 中 | `ErrSFUNotSupported` | 前端停推兜底 |

### 4. 各 Provider 说明

| Provider | 路径 | 关键事实 |
|----------|------|----------|
| LiveKit | `internal/livekit/client.go` | 唯一全 ✅；`MuteParticipant` 支持按 trackSid 精确静音或按 identity 批量静音；`ProviderName()` = `livekit` |
| Agora | `internal/agora/provider.go` | Token/列举可用；`MuteParticipant` 静默空操作、`RemoveParticipant` 不支持、`GenerateAdminToken` 空串；`ClientInfo` 暴露 `appId` |
| MediaSoup | `internal/mediasoup/provider.go` | 经 bridge 实现列举/静音/踢人/删房；自有 Socket.IO 信令路径（`sfu:produce` 等）；`ProviderName()` = `mediasoup` |
| SRS | `internal/srs/provider.go` | WHIP/WHEP；`List*` 经 `RoomRegistry` 聚合真实房间；`StreamProvider`/`ClientInfoProvider`；`GenerateToken` 签发 stream token |
| Daily | `internal/daily/provider.go` | `RemoveParticipant` 已实现（list→session id）；`MuteParticipant` 不支持；`GenerateAdminToken` 降级 |
| Cloudflare | `internal/cloudflare/provider.go` | 无原生 room/token；`GenerateToken` 建 session 返回 JSON 配置；`List*` 仅内存；`StreamProvider`/`ClientInfoProvider` |

### 5. 前端 SFU 客户端（`packages/sfu-client`）

六端均已实现（LiveKit、Agora、MediaSoup、SRS、Daily、Cloudflare）。无已知标记。Cloudflare 通过 `cloudflare-client.ts` 接入，使用 background signal 并按 session stream 订阅对端。

### 6. 关于 Mute 语义的说明

SFU `MuteParticipant` / `MuteRoomParticipant` 是 **服务端轨道级 SFU mute**，而非用户禁言。

用户禁言由独立层实现：
- `MuteService` + `MuteHandler` + `Hub.BroadcastMute` / `Hub.BroadcastUnmute`
- WebSocket 事件：`user:muted` / `user:unmuted`
- 数据库中持久化禁言记录，含过期自动清理
- 该层 **已完整实现且功能齐全**

本地播放静音（静音远端音轨音量）是纯客户端行为，与以上两层无关。

### 7. 信令层踢人分发（按 `ErrSFUNotSupported` 优雅降级）

`internal/signal/hub.go` 的 `removeParticipantSafe` 不再硬编码 provider 名，而是直接调用 `sfuProvider.RemoveParticipant(room, identity)`，仅当返回 `pkg.ErrSFUNotSupported` 时静默跳过。因此「踢人是否真正到达 SFU」完全由各 provider 自身实现决定：

- 已实现并会被调用：`livekit`、`mediasoup`、`srs`、`daily`
- 返回 `ErrSFUNotSupported` 被跳过：`agora`（无单用户断开 API）

这与历史上的「livekit/srs 才调，其余跳过」硬编码表不同，新增 provider 只要实现 `RemoveParticipant` 即自动生效，无需改 Hub。
