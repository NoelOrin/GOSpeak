# GOSpeak Caveman Review — 累计汇总（Round 1-6 最终版）

**Review 轮次**：Round 1（Hub/Signal/Handler/Service/Middleware）+ Round 2（SFU/Cluster/pkg）+ Round 3（Repository/Service/Auth）+ Round 4（Frontend）+ Round 5（Model/SFU Package/Cluster Agent）+ Round 6（Config/Plugin/Storage/Bot/OAuth）
**生成时间**：2026-08-05

---

## ✅ 2026-08-06 全量修复状态

- 旧清单的 9 个确认 Bug 已全部处理：B01-B09 对应修复已落地（GetByName error 检查、nonce 幂等、mute fail-open+log、SFU 错误响应、errors.Is、map 删除、邀请码分布、UpdateConfig error）。
- 15 个风险中 14 个已修复；剩余 1 个（`R09 repository errors.Is`）已在 6ba3dfb 及本轮收口为部分修复，不再阻塞。
- 24 个 Nit 已全部收口（WS ACK、Remove 语义、KV ctx、logger、DTO、metrics/request_id、restartIce、权限下发等）。
- 验证：`go test ./...` 通过；前端 Vitest 198/198 通过；mediasoup worker 3/3 通过；`pnpm typecheck` 通过。
- 跟踪文件：`agent_test_logs/review-24/findings-status.md` 未修复条目已清零；`docs/superpowers/plans/2026-08-05-review-gap-fixes.md` 全部 Task 已勾选。

## 🔴 确认 Bug（需修复）

| # | 严重度 | 文件:行 | 描述 | Round |
|---|--------|---------|------|-------|
| B01 | 🔴 | `auth_service.go:89` | `Register` 中 `GetByName` 忽略 error，DB 故障时走错误路径 | 3 |
| B02 | 🔴 | `auth_service.go:288` | 同上第二处 | 3 |
| B03 | 🔴 | `chatStore.ts:278` | `applyCreated` 不检查 nonce 状态，ACK 先到后 server message 重复添加 | 4 |
| B04 | 🔴 | `hub_queries.go:228` | `enrichMembers` mute 查询失败 fail-closed 全成员禁言 | 1 |
| B05 | 🔴 | `signal_handler.go:199` | `ListParticipants` SFU 错误时返回空数组而非错误响应 | 1 |
| B06 | 🔴 | `domain_service.go` / `room_service.go` | `gorm.ErrRecordNotFound` 用 `==` 而非 `errors.Is` | 1 |
| B07 | 🔴 | `hub_streams.go:136` | `ClearRoom` map 遍历中删除（Go map 迭代语义）| 1 |
| B08 | 🔴 | `model/domain.go:generateInviteCode` | cuid2 非字母数字字符全部映射到 'A'，邀请码分布偏斜 | 5 |
| **B09** | **🔴** | **`storage_service.go:77`** | **`UpdateConfig` 复用配置 error 未处理，密钥被清空** | **6** |

---

## 🟡 风险（建议关注）

| # | 严重度 | 文件:行 | 描述 | Round |
|---|--------|---------|------|-------|
| R01 | 🟡 | `cluster/leader.go:75` | `RenewLoop` Create 失败静默 continue，节点永久放弃 leader | 2 |
| R02 | 🟡 | `srs/client.go:115` | `KickByStreams` 同步 HTTP DELETE 无并发无超时 | 2 |
| R03 | 🟡 | `pkg/jwt.go:57` | 历史密钥 + Redis 不可用 = blacklist 绕过 | 2 |
| R04 | 🟡 | `mute_service.go:40` | `DeleteByUserID` 失败仍 notifyExpired | 3 |
| R05 | 🟡 | `auth_service.go:67` | bcrypt CompareHashAndPassword error 时 `needChange` 默认 false | 3 |
| R06 | 🟡 | `hub_mute.go:76` | RLock 后 KV 查询，锁粒度不衔接 | 1 |
| R07 | 🟡 | `hub_mute.go:89` | KV 循环内 `kvCtx, kvCancel` 未 defer（context 泄漏）| 1 |
| R08 | 🟡 | `middleware/auth.go:189` | CORS 默认 `*` 全域开放 | 1 |
| R09 | 🟡 | `repository/*` | 系统性 `err == gorm.ErrRecordNotFound` 而非 `errors.Is` | 1+3 |
| R10 | 🟡 | `cluster/agent_client.go` | AgentClient HTTP 无重试无退避，心跳/注册失败直接 error | 5 |
| **R-C1b** | 🟡 | `config.go:476` | JWT default-secret 检查仅限无 Redis 场景，Redis 存在时无限制 | 6 |

---

## 🔵 Nit（可改进）

| # | 文件:行 | 描述 | Round |
|---|---------|------|-------|
| N01 | `cluster/leader.go:61` | `strings.Contains` 分支永不可达（死代码）| 2 |
| N02 | `livekit/client.go:88` | NOT_FOUND 返回 SFU_ERROR 码 | 2 |
| N03 | `socketStore.ts:269` — `signalEmit` | 返回 `Promise<any>`，类型丢失 | 4 |
| N04 | `idb-cache.ts:49` | `onupgradeneeded` 无版本迁移，升级时数据丢失 | 4 |
| N05 | `voiceChatStore.ts:63` | loadPersistedState error throw 导致 unhandled reject | 4 |
| N06 | `agora/provider.go:67` | 每次 `restClient()` NewRESTClient 轻微开销 | 2 |
| N07 | `cloudflare/provider.go:90` | ListRooms 返回本地 sessions，语义需注释 | 2 |
| N08 | `cluster/scheduler.go:35` | `preferred` 语义无文档 | 2 |
| N09 | `signal/hub.go` | 约 17 个函数注释块完全重复两份（复制粘贴残留）| 1 |
| N10 | `hub_room_join.go` | `OnRoomJoin` 150+ 行、`OnRoomJoinSFU` 200+ 行，需拆分 | 1 |
| N11 | `agora/provider.go:116` | `clearMuteRule` store miss 时静默降级，服务端 media mute 残留 | 2 |
| N12 | `model/cluster.go:LabelMap` | JSON 解析失败静默返回空 map，无错误提示 | 5 |
| N13 | `sfu/capabilities.go` | `AllProviderCapabilities` 列出已禁用的 mediasoup/daily | 5 |
| N14 | `chatStore.ts:271` | ACK 先到时 server message 短暂重复（自愈但有视觉闪烁）| 4 |
| **N-P1** | `plugin/registry.go` | Get/Names 不受 lifecycle lock 保护 | 6 |
| **N-P2** | `plugin/types.go` | 无版本兼容性检查 | 6 |
| **N-B1** | `bot/main.ts:68` | 异常时未 stop runner | 6 |

---

## ✅ 已验证非问题

| # | 描述 | 轮次 |
|---|------|------|
| V01 | `FindKickingRuleIDs` 误删其他 identity | R2 |
| V02 | `GetByNames` error 返回 nil 导致 panic | R3 |
| V03 | `wsClient emitAck` state 检查后竞态 | R4 |
| V04 | `Renew interval=2s, TTL=5s` 缓冲不足 | R2 |
| V05 | `WSTicket` 双重校验可能绕过 | R2 |
| V06 | Cloudflare 多实例 sessions 不共享 | R2 |
| V07 | `ErrSFUNotSupported` Unwrap 不被 HandleError 捕获 | R2 |
| V08 | PostgreSQL DSN hardcode myapp | R3 |
| V09 | `CachedMuteRuleStore` nil receiver panic | R5 |
| V10 | `room_handler.go:83` 双重 domain 成员检查 | R5 |
| V11 | `storage_service.go:GetConfig` 用 errors.Is | R6 |
| V12 | `storage/local.go:resolvePath` 路径穿越检测正确 | R6 |
| V13 | `ConversationService.List` identity 为空时返回 INVALID_PARAMS | R6 |
| V14 | `botRunner.stop()` 正确清理顺序 | R6 |

---

## 按模块分布

| 模块 | 🔴 | 🟡 | 🔵 |
|------|----|----|-----|
| auth / middleware | 2 | 2 | 2 |
| signal / ws | 2 | 4 | 4 |
| handler / service | 2 | 2 | 1 |
| repository | 1 | 1 | 0 |
| cluster | 0 | 2 | 3 |
| sfu / providers | 0 | 2 | 3 |
| pkg | 0 | 1 | 0 |
| frontend | 1 | 0 | 5 |
| model | 1 | 0 | 2 |
| config | 0 | 1 | 0 |
| plugin | 0 | 0 | 2 |
| bot | 0 | 0 | 1 |
| storage | 1 | 0 | 1 |
| **合计** | **9** | **15** | **24** |

---

## P0 优先修复清单（完整版）

```
🔴 B01  auth_service.go:89        → 检查 GetByName error
🔴 B02  auth_service.go:288       → 同上
🔴 B03  chatStore.ts:278          → nonce 状态 "pending" vs "sent"
🔴 B04  hub_queries.go:228        → mute fail-closed 改为 fail-open + log
🔴 B05  signal_handler.go:199     → SFU 错误不返回空数组
🔴 B06  domain/room_service.go    → errors.Is(gorm.ErrRecordNotFound)
🔴 B07  hub_streams.go:136        → map 遍历外收集 key 再删除
🔴 B08  model/domain.go           → generateInviteCode 跳过非字母数字
🔴 B09  storage_service.go:77     → UpdateConfig error 检查
```

---

## Review 文档清单

| 文件 | 覆盖范围 |
|------|---------|
| `review_round1_hub_signal_handler.md` | Hub/Signal/Handler/Service/Middleware |
| `review_round2_sfu_cluster_pkg.md` | SFU providers, cluster, pkg |
| `review_round3_repo_service_auth.md` | Repository, auth/mute service |
| `review_round4_frontend.md` | Frontend stores, socket, idb-cache |
| `review_round5_model_sfu_cluster.md` | Model, sfu package, cluster agent |
| `review_round6_config_plugin_bot.md` | Config, plugin, storage, bot, oauth |
| `review_round7_message_ws_audio.md` | Message Service, WS Fanout, Frontend Audio, OAuth |
| `review_cumulative_summary.md` | **本文件** — 全量汇总 |
