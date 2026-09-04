## SFU Provider 实现完整度矩阵

**评估时间**: 2026-08-03（已对照 `app/server/internal/sfu/providers/` 源码核实）
**接口**: `sfu.Provider`（[provider.go](/Users/noelorin/GOSpeak/app/server/internal/sfu/provider.go)）

> 图例：✅ 完整 / ⚠️ 降级 / ❌ 返回 `ErrSFUNotSupported` 或静默空操作

### 1. 整体成熟度概览

| Provider | 成熟度 | 接口方法覆盖 | 关键缺口 |
|----------|--------|--------------|----------|
| LiveKit | 完整 | 8/8 ✅ | 无 |
| SRS | 高 | 8/8 | mute 为 Discord 式（订阅端静音 + on_publish 禁推，degraded）；成员仍可收听 |
| Agora | 中 | 8/8 | mute/kick = degraded（kicking-rule） |
| Cloudflare | 低–中 | 8/8 | `List*` 仅进程内内存、`MuteParticipant` ❌（soft） |

### 2. 方法覆盖矩阵

| 方法 | LiveKit | SRS | Agora | Cloudflare |
|------|---------|-----|-------|------------|
| `GenerateToken` | ✅ | ✅ | ✅ | ⚠️ (JSON 配置块) |
| `ListRooms` | ✅ | ✅ | ✅ | ⚠️ (进程内缓存) |
| `ListParticipants` | ✅ | ✅ | ✅ | ⚠️ (进程内缓存) |
| `MuteParticipant` | ✅ | ✅ (禁推黑名单，不踢流) | ✅ (禁发布 rule) | ❌ |
| `RemoveParticipant` | ✅ | ✅ | ✅ (短时 kicking-rule) | ✅ (DeleteSession) |
| `DeleteRoom` | ✅ | ✅ | ✅ | ✅ (批量删 session) |
| `GetHost` | ✅ | ✅ | ✅ | ✅ |

### 3. 缺口与补全（统一表）

| Provider | 方法 | 严重程度 | 当前行为 | 建议 / 状态 |
|----------|------|----------|----------|-------------|
| Agora | `RemoveParticipant` | 低 | 短时 kicking-rule（默认 60s）强制离会 | 语义接近 hard kick，短暂挡重进；非永久 ban |
| SRS | `MuteParticipant` | 中高 | degraded：禁推黑名单（不踢流，订阅端静音） | unmute 删除黑名单，前端恢复音量 |
| Cloudflare | `GenerateToken` | 中 | 返回 JSON 配置块（sessionId/appId/stunUrl），非真实鉴权 token | 设计如此（无原生 token 体系） |
| Cloudflare | `ListRooms` | 中 | 仅进程内 `sessions` map（room→identity→sessionId） | 非跨实例权威，进程重启/多实例不同步 |
| Cloudflare | `ListParticipants` | 中 | 仅进程内 map 的 identity 集合，`JoinedAt` 为当前时间 | 非跨实例权威 |
| Cloudflare | `MuteParticipant` | 中 | `ErrSFUNotSupported` | 前端停推兜底 |

### 4. 各 Provider 说明

| Provider | 路径 | 关键事实 |
|----------|------|----------|
| LiveKit | `internal/sfu/providers/livekit/` | 唯一全 ✅；`MuteParticipant` 支持按 trackSid 精确静音或按 identity 批量静音；`ProviderName()` = `livekit` |
| SRS | `internal/sfu/providers/srs/` | WHIP/WHEP；`List*`/`DeleteRoom` 经 `/api/v1/streams`+`/api/v1/clients` 直查 + stream→room 反查；`StreamProvider`/`ClientInfoProvider`；`GenerateToken` 签发 stream token |
| Agora | `internal/sfu/providers/agora/` | Token/列举可用；mute/kick 走 kicking-rule（degraded）；rule id 经 MuteRuleStore 跨实例缓存（nats KV→memory）；`ClientInfo` 暴露 `appId` |
| Cloudflare | `internal/sfu/providers/cloudflare/` | 无原生 room/token；`GenerateToken` 建 session 返回 JSON 配置；`List*` 仅内存；`StreamProvider`/`ClientInfoProvider` |

### 5. 前端 SFU 客户端（`packages/sfu-client`）

四端均已实现（LiveKit、SRS、Agora、Cloudflare）。无已知标记。SRS/Cloudflare 通过 `srs-client.ts` / `cloudflare-client.ts` 接入，使用 background signal 并按 session/stream 订阅对端。

### 6. 发言检测（Active Speakers）能力矩阵

**评估时间**: 2026-08-05（已对照 `packages/sfu-client/src/` 与 `internal/signal/hub_room_events.go` 源码核实）

| Provider | 检测机制 | 链路 | 能力 |
|----------|----------|------|------|
| LiveKit | SFU 原生 `RoomEvent.ActiveSpeakersChanged` | 客户端 `onActiveSpeakers` → 前端高亮 | ✅ 原生 |
| Agora | SFU 原生音量上报 `volume-indicator`（level>5 过滤） | 客户端 `onActiveSpeakers` → 前端高亮 | ✅ 原生（阈值） |
| SRS | 无原生 → 本地 WebAudio 音量分析（AudioWorkletProcessor 事件驱动采样，avg>10） | `onLocalSpeakingChange` → `member:speaking` → 服务端聚合 → `room:active-speakers` | ⚠️ 降级 |
| Cloudflare | 无原生 → 本地 WebAudio 音量分析（AudioWorkletProcessor 事件驱动采样，avg>10） | 同上 | ⚠️ 降级 |

**信令聚合链路要点**（SRS / Cloudflare）：
- 客户端仅上报「自身」状态；服务端按房间聚合 `Room.Speaking` 后广播 `room:active-speakers`（identities 列表）
- 服务端 `OnMemberSpeaking`：本人校验 + 禁言 fail-closed 检查 + **同值去重**（状态未变化不广播）
- 客户端采样为事件驱动（AudioWorkletProcessor 运行在音频渲染线程，按块 `postMessage` 触发主线程采样，无 JS 定时器轮询；默认 16 块 ≈ 43ms），本地滞回（SRS holdOn 120ms / holdOff 300ms；Cloudflare holdOn 150ms / holdOff 500ms）只在状态翻转时上报，另加同值节流（≥150ms）兜底
- 新成员 `room:join:sfu` 成功后服务端回放当前 active speakers，加入者立即看到正在发言的人
- 成员离开 / 断连 / 被踢 / 切房时清发言态并广播最新列表重置高亮
- 前端离开 / 断连 / 切房时清空 `speakingIdentities`，避免陈旧高亮残留

**已知限制**：
- SRS / Cloudflare 为本地音量阈值判定，非 SFU 端权威；阈值附近依赖滞回抑制闪烁
- 多实例部署下，新成员 join 回放仅包含本实例成员发言态；跨实例发言以实时事件广播为准
- `srs-client.ts` / `cloudflare-client.ts` 的 `onActiveSpeakers` 为接口占位，实际发言态只走 `onLocalSpeakingChange` 信令链路

### 7. 关于 Mute 语义的说明

SFU `MuteParticipant` 是 **服务端轨道级 SFU mute**，而非用户禁言。

用户禁言由独立层实现：
- `MuteService` + `MuteHandler` + `Hub.BroadcastMute` / `Hub.BroadcastUnmute`
- WebSocket 事件：`user:muted` / `user:unmuted`（被禁言者本人）；`member:muted` / `member:unmuted`（全员订阅端静音）
- 数据库中持久化禁言记录，含过期自动清理
- 该层 **已完整实现且功能齐全**

本地播放静音（静音远端音轨音量）是纯客户端行为，与以上两层无关。服务器禁言在 SRS 等无原生媒体静音的
provider 上，通过 `member:muted` 事件驱动订阅端强制静音（`handler_audio.setServerMutedByIdentity`），
与本地个人静音/音量独立。

### 8. 信令层踢人分发（按 `ErrSFUNotSupported` 优雅降级）

`internal/signal/hub.go` 的 `removeParticipantSafe` 不再硬编码 provider 名，而是直接调用 `sfuProvider.RemoveParticipant(room, identity)`，仅当返回 `pkg.ErrSFUNotSupported` 时静默跳过。因此「踢人是否真正到达 SFU」完全由各 provider 自身实现决定：

- 已实现并会被调用：`livekit`、`srs`、`cloudflare`
- Agora 现通过短时 kicking-rule 实现 hard kick（短暂挡重进）

新增 provider 只要实现 `RemoveParticipant` 即自动生效，无需改 Hub。

### Enforcement 事件语义

`room:kicked` / `user:muted` / `user:unmuted` 携带 `enforcement`:
- `hard`：原生媒体强制成功
- `degraded`：替代 API 强制成功（Agora rule / SRS 踢推流）
- `soft`：仅信令/策略 + 客户端配合
