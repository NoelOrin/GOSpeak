## SFU Provider 实现完整度矩阵

**评估时间**: 2026-07-10
**接口**: `sfu.Provider`（[provider.go](/Users/noelorin/GOSpeak/app/server/internal/sfu/provider.go)）

### 方法覆盖

| 方法 | LiveKit | Agora | MediaSoup | SRS | Daily |
|------|---------|-------|-----------|-----|-------|
| `GenerateToken` | ✅ | ✅ | ✅ | ✅ | ✅ |
| `GenerateAdminToken` | ✅ | ⚠️ | ⚠️ | ✅ | ⚠️ |
| `ListRooms` | ✅ | ✅ | ✅ | ✅ | ✅ |
| `ListParticipants` | ✅ | ✅ | ✅ | ✅ | ✅ |
| `MuteParticipant` | ✅ | ❌ | ✅ | ❌ | ❌ |
| `MuteRoomParticipant` | ✅ | ❌ | ✅ | ❌ | ❌ |
| `RemoveParticipant` | ✅ | ❌ | ✅ | ✅ | ❌ |
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

MediaSoup 已实现全部 participant 相关方法:
- `ListParticipants` — bridge 转发 worker participant 索引
- `MuteParticipant` — producer pause/resume(trackSid 当 producerId;空则批量)
- `MuteRoomParticipant` — 批量 pause/resume 该 identity 所有 producer
- `RemoveParticipant` — close 该 identity 的 transport(级联关 producer)

MediaSoup 仍通过自有信令路径([signal.go](/Users/noelorin/GOSpeak/app/server/internal/mediasoup/signal.go))协商媒体,通过 Socket.IO 提供:`sfu:get-router-capabilities`、`sfu:create-transport`、`sfu:connect-transport`、`sfu:produce`、`sfu:consume`、`sfu:close-transport`。并实现 `ParticipantCleanupHandler` 接口,在 Hub OnDisconnect 时广播 `sfu:producer-closed` + 清理 worker transport。active speaker 由前端 WebAudio AnalyserNode 检测(sfu-client),非服务端 observer。

#### SRS — `internal/srs/provider.go`

| 方法 | 严重程度 | 当前行为 |
|------|----------|----------|
| `ListRooms` | — | ✅ 通过 `RoomRegistry`（Hub room→streams）聚合真实 room，并填充 `MemberCount`。registry 未注入时返回 `SFU_ERROR`。 |
| `ListParticipants` | — | ✅ 按 registry 的 stream 集合过滤 `/api/v1/clients/`，并用 `IdentityForStream` 映射回 identity。无登记 stream 时返回空列表（不再误用 room 名当 stream）。 |
| `GetHost` | — | ✅ 返回 SRS HTTP API base（`http://{host}:{apiPort}`）。 |
| `RemoveParticipant` | — | ✅ list-then-kick：优先 registry 实际 stream，降级 `GenerateStreamName`；`DELETE /api/v1/clients/{id}`。 |
| `DeleteRoom` | — | ✅ kick 该 room 全部 stream client 后 `ClearRoom`；无 registry 时降级旧 streams DELETE（SRS5 返 2048）。 |
| `MuteParticipant` | 中 | ❌ `NewErrSFUNotSupported()` — SRS 无服务端轨道静音，前端停推/关 track。 |
| `MuteRoomParticipant` | 中 | ❌ 接口层未暴露于 `sfu.Provider`；语义上同 mute，SRS 不支持服务端 mute。 |

补充能力：
- `StreamProvider` / `ClientInfoProvider`：`StreamInfo` 发 stream+HMAC token；`ClientInfo.whipUrl` 同源相对路径。
- `POST /api/v1/srs/callback`：`on_publish` 校验 streamToken；`on_play` 查 `activeStreams`；`on_unpublish` 注销。
- `deploy/srs/srs.conf` 已配 `http_hooks` 到 `host.docker.internal:8998`。
- `SRS_SECRET` 空时 token 拒绝签发（`SFU_NOT_CONFIGURED`）。注意 DB 持久化的 sfu config 优先于 env；旧空 secret 需走 `/api/v1/sfu/update-config` 写入。

> 可用性验证(2026-07-10): SRS 6.0.184 docker 起、API versions/clients/streams 通；backend 切 srs 后 join token 返回 provider/stream/streamToken/serverUrl/whipUrl；callback 鉴权/放行/注销链路通过。runbook 见 `docs/srs-selfhost-runbook.md`。

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

五端均已实现（LiveKit、Agora、MediaSoup、SRS、Daily）。无已知标记。

### 关于 Mute 语义的说明

SFU `MuteParticipant` / `MuteRoomParticipant` 是 **服务端轨道级 SFU mute**，而非用户禁言。

用户禁言由独立层实现：
- `MuteService` + `MuteHandler` + `Hub.BroadcastMute` / `Hub.BroadcastUnmute`
- WebSocket 事件：`user:muted` / `user:unmuted`
- 数据库中持久化禁言记录，含过期自动清理
- 该层 **已完整实现且功能齐全**

本地播放静音（静音远端音轨音量）是纯客户端行为，与以上两层无关。

