# GOSpeak Caveman Review — Round 1 (Hub / Signal / Handler / Service / Middleware)

Generated: 2026-08-05
Scope: internal/signal/hub.go, internal/signal/hub_*.go, internal/handler/signal_handler.go, internal/service/room_service.go, internal/middleware/auth.go
**注意**：本文件为补生成，Round 1 findings 来自初次 review session。

---

## 🔴 Bug（Round 1 原始发现）

**B04 — hub_queries.go:228** `enrichMembers` mute 查询失败时 fail-closed 将所有成员标记为 `IsMuted=true`。DB 抖动时整个房间所有用户被集体禁言显示，影响范围极大。
→ 建议降级为"未禁言"（fail-open）并打日志告警。

**B05 — signal_handler.go:199** `ListParticipants` SFU 调用出错时返回空数组 `pkg.Success(c, [])` 而非错误响应。SFU 故障被静默掩盖，客户端无法区分"真的没人"和"服务异常"。
→ 出错时 `pkg.HandleError(c, err)` 或至少返回错误码。

**B06 — domain_service.go / room_service.go** `gorm.ErrRecordNotFound` 比较用 `==` 而非 `errors.Is()`。某些 GORM 版本或驱动可能返回 wrapper 类型的 NotFound，漏检会导致内部错误被当作 NotFound 处理。
→ 统一改为 `errors.Is(err, gorm.ErrRecordNotFound)`。

**B07 — hub_streams.go:136** `ClearRoom` 对 map 做 `for rk, local := range h.roomStreams { delete(...) }` — 遍历中删值在 Go map 迭代语义上未定义，可能跳元素或 panic。
→ 先收集 key 列表再删除。

---

## 🟡 Risk（Round 1 原始发现）

**R06 — hub_mute.go:76** `enforceUserMediaMute` RLock 释放后、KV 查询前状态可能变化，锁粒度不衔接。
→ 在 RLock 内构建 KV 查询的 identity 列表，再统一做 KV 调用。

**R07 — hub_mute.go:89** KV 循环内的 `kvCtx, kvCancel := kvTimeoutCtx()` 在 `continue` 时未调用 `kvCancel()`，造成 context 泄漏。
→ `defer kvCancel()` 应在每次 `kvCtx` 创建后立即附加。

**R08 — middleware/auth.go:189** `CORS` 默认 `allowed = "*"` 全域开放。生产部署时需确保 `CORS_ORIGIN` env 被正确设置。
→ 考虑在 `release` GIN_MODE 下拒绝 `*` 配置。

**R09 — repository/*（系统性）** 多处 `err == gorm.ErrRecordNotFound` 而非 `errors.Is`，Round 3 已专项整理。

---

## 🔵 Nit（Round 1 原始发现）

**N09 — signal/hub.go** 约 17 个函数的注释块完全重复两份（复制粘贴残留）。
→ 清理所有重复注释。

**N10 — hub_room_join.go** `OnRoomJoin` 150+ 行、`OnRoomJoinSFU` 200+ 行，需拆分出 `validateJoinPolicy()` / `trackConnSlot()` 等子方法。

**N11 — middleware/auth.go** `permChecker`/`tokenVersionCheck`/`botTokenCheck` 三个全局 setter 注入点，package 级单例状态。并发测试需注意状态隔离。

---

## 二次验证状态

| # | Severity | 描述 | Verified |
|---|----------|---------|----------|
| B04 | 🔴 | enrichMembers fail-closed | ✅ 确认 |
| B05 | 🔴 | ListParticipants 静默失败 | ✅ 确认 |
| B06 | 🔴 | gorm.ErrRecordNotFound `==` | ✅ 确认 |
| B07 | 🔴 | ClearRoom map 遍历删值 | ✅ 确认 |
| R06 | 🟡 | hub_mute RLock/KV 锁粒度 | ✅ 确认 |
| R07 | 🟡 | hub_mute context 泄漏 | ✅ 确认 |
| R08 | 🟡 | CORS 默认 `*` | ✅ 确认 |
