# GOSpeak Caveman Review — Round 2 (SFU / Cluster / pkg) — 二次验证版

Generated: 2026-08-05 (initial) → 二次验证完成
Scope: sfu/providers/*, cluster/*, pkg/*

---

## 验证结论汇总

| # | Severity | File:Line | Finding | 验证结果 |
|---|----------|-----------|---------|----------|
| 1 | 🟡 | agora/provider.go:116 | clearMuteRule store miss 时用 FindKickingRuleIDs 恢复，API 拉全量再 in-memory 过滤 | ✅ 确认（但有恢复失败静默风险）|
| 2 | 🔴 | cluster/leader.go:61 | strings.Contains 分支在 errors.Is return false 后永不可达，死代码 | ✅ 确认 |
| 3 | 🟡 | cluster/leader.go:75 | RenewLoop Create 失败静默 continue，无重试无日志 | ✅ 确认 |
| 4 | 🔵 | cloudflare/provider.go:163 | DeleteRoom map 删除早于 WaitGroup goroutines | ⚠️ 部分确认（goroutines 抓 sessionIDs 副本，安全但顺序反常规）|
| 5 | 🟡 | pkg/jwt.go:57 | 历史密钥 + Redis 不可用 = blacklist 失效 | ✅ 确认（graceful degradation 设计限制）|
| 6 | 🔵 | cluster/scheduler.go:35 | preferred 语义无文档 | ✅ 确认（逻辑正确但无说明）|
| 7 | 🔵 | agora/provider.go:67 | 每次 restClient() NewRESTClient 轻微开销 | ✅ 确认 |
| 8 | 🟡 | srs/client.go:115 | KickByStreams 同步 HTTP 无并发无超时 | ✅ 确认 |
| 9 | 🔵 | cluster/control.go:38 | mute/unmute 无 scope 校验 | ✅ 确认（设计选择，非缺陷）|
| 10 | 🔵 | livekit/client.go:88 | NOT_FOUND 用 SFU_ERROR 码 | ✅ 确认 |
| 11 | 🔵 | cloudflare/provider.go:90 | ListRooms 语义需注释 | ✅ 确认 |
| 12 | 🔵 | pkg/errors.go:95 | ErrSFUNotSupported Unwrap + HandleError | ✅ 确认无问题（caller 用 errors.Is 绕过了 HandleError 的 As 限制）|
| 13 | 🔵 | cluster/leader.go:28 | Renew interval=2s, TTL=5s 缓冲 3s | ✅ 确认（安全）|
| 14 | ❓ | cloudflare/provider.go:29 | 多实例 sessions 不共享 | ✅ 确认（Cloudflare 已知限制）|
| 15 | ✅ | pkg/jwt.go:83 | WSTicket 双重校验配套 | ✅ 确认（Subject + TokenType + IssuedAt 三重检查）|

---

## 确认 Bug

### 🔴 B2 — cluster/leader.go:61: strings.Contains 分支永不可达

**代码：**
```go
_, err := l.kv.Create("active", []byte(nodeID))
if err == nats.ErrKeyExists {
    return false, nil
}
if err != nil {
    msg := err.Error()
    if errors.Is(err, nats.ErrKeyExists) || strings.Contains(msg, "wrong last sequence") || strings.Contains(msg, "key exists") {
        return false, nil
    }
    return false, err
}
return err == nil, err
```

**分析：** 第一层 `err == nats.ErrKeyExists` 已处理 `ErrKeyExists` case return false；第二层 `errors.Is(err, nats.ErrKeyExists)` 永远不会被第一层的 false 分支触发；第三层 `strings.Contains` 同理——`Create` 返回 `nil` 时 err 不为 nil，`Create` 返回错误时已在第一层处理后 return，永不到第三层。**死代码，删掉 strings.Contains 那两行。**

---

## 确认风险

### 🟡 R1 — cluster/leader.go:75: RenewLoop 失败静默放弃 leader

```go
entry, err := l.kv.Get("active")
if err != nil || string(entry.Value()) != nodeID {
    _, _ = l.kv.Create("active", []byte(nodeID)) // Create 失败后静默 continue
    continue
}
```

`Create` 返回 `ErrKeyExists`（另一个节点在此期间抢了锁）时静默 continue，节点永久放弃 leader 身份而不自知。**建议：Create 失败时打日志，或给失败计数，超过阈值时退出 RenewLoop。**

### 🟡 R3 — srs/client.go:115: KickByStreams 同步 HTTP DELETE 无并发

```go
for _, id := range toKick {
    req, _ := http.NewRequest(http.MethodDelete, ...)
    if err := c.doCodeRequest(req, "kick participant"); err != nil {
        remaining++
        continue
    }
    kicked++
}
```

对每个 client ID 同步发 HTTP DELETE，N 个参与者等 N×RT。若 SRS 响应慢（>5s），整个信令操作阻塞。**建议：并发 kick（goroutine + WaitGroup），加 context 超时。**

### 🟡 R5 — pkg/jwt.go:57: 历史密钥 + Redis 不可用 = blacklist 绕过

`redis.IsBlacklisted` 在 Redis 不可用时是 no-op（直接返回 false）。此时任何以当前密钥或历史密钥签发的 token，只要不在 blacklist 里就能通过验证。**这是 graceful degradation 的固有风险**，不是 bug，但应加文档说明：Redis 不可用时 JWT revoke 失效，应在恢复后强制重新登录。

### 🟡 R1-alt — agora/provider.go:116: clearMuteRule 恢复失败静默降级

`FindKickingRuleIDs` 查 Agora 全量规则 in-memory 过滤（按 channelName + UIDStr/UID 匹配 identity）。逻辑正确，但如果 API 调用失败或返回空（store miss + API 不可达），`clearMuteRule` 返回 nil，用户的 Agora 服务端 media mute 规则未被清除。**客户端侧已解除，但服务端 media 仍在（等 TTL 自然过期）**。建议：记录静默失败的 identity，供人工干预或后续清理 job。

---

## Nit / 改进项

### 🔵 N1 — livekit/client.go:88: MuteParticipant participant not found 返回 SFU_ERROR 而非 NOT_FOUND

参与者不存在不是服务内部错误，建议用 `pkg.NOT_FOUND`。

### 🔵 N4 — cloudflare/provider.go:163: DeleteRoom map 删除早于 WaitGroup

代码顺序：`sessionIDs` → `delete(s.sessions, room)` → goroutines → `WaitGroup.Wait()` → `registry.ClearRoom`。goroutines 捕获 `sessionIDs` 副本，无 data race，但顺序反直觉。建议注释说明 goroutines 不依赖 map，或将 map 删除移到 WaitGroup.Wait() 后。

### 🔵 N6 — cluster/scheduler.go:35: preferred 语义无文档

`ChooseNodes` 的 `preferred` 参数只影响同评分排序，不保证选中。应加 godoc 说明：优先节点若不满足调度条件会被静默忽略。

### 🔵 N7 — agora/provider.go:67: restClient() 每次 NewRESTClient

```go
func (s *Service) restClient() *RESTClient {
    if s.rest != nil { return s.rest }
    return NewRESTClient(...)
}
```

当 `s.rest == nil`（未注入时）每次调用 New，性能轻微损耗。建议字段持有或加 memoize。

### 🔵 N8 — cluster/control.go:38: mute/unmute 无 scope

`Validate` 允许 domain_uuid 为空的 mute/unmute，Hub 会遍历所有房间。建议要求 domain_uuid 作为 scope 限制，避免误操作全平台。

### 🔵 N10 — cloudflare/provider.go:90: ListRooms 语义

Cloudflare ListRooms 返回本地 sessions map 计数（内存），与其他 provider 从 SFU API 查询语义不同。应加注释：`// returns locally tracked session count, not authoritative SFU data`。

### 🔵 N11 — cluster/leader.go:28: interval vs TTL

Renew interval=2s，TTL=5s，缓冲 3s。只要单个 renew 操作（含网络 RTT）不超过 3s 就安全——当前实现每次 renew 是同步 Update，理论上一次网络慢（>3s）会导致锁过期但 leader 仍以为自己持有。**建议：interval = TTL/2 = 2.5s，round up 到 3s。**

---

## ❓ Q — 已确认非问题

### ✅ Q14: Cloudflare 多实例 sessions 不共享

Cloudflare 的 rooms/sessions 是本地内存 map，无跨实例同步。这是 Cloudflare Realtime 的架构限制（无原生 rooms），各实例独立追踪。**是已知限制，非 bug。**

### ✅ Q15: WSTicket 双重校验

`IsWSTicket` 校验 `Subject == WSTicketSubject || TokenType == WSTicketType`；`WSTicketExpired` 用 `IssuedAt` 判断（45s）。`GenerateWSTicket` 同时设置 `ExpiresAt` 和 `Subject`。三重校验配套使用，未发现绕过路径。

---

## 累计确认待修复项（Round 1 + Round 2）

**Round 1（hub/signal/handler/service/middleware）**
- B1: `gorm.ErrRecordNotFound` 用 `==` 而非 `errors.Is`（domain_service.go, room_service.go）
- B2: `ListParticipants` 出错返回空数组（signal_handler.go:199）
- B3: `enrichMembers` mute 失败 fail-closed 全成员禁言（hub_queries.go:228）
- B4: `hub_mute.go:76` RLock→KV 查询状态可能变化（锁粒度）
- B5: `ws/client.go:149` Send 双检不完整
- R1: CORS 默认 `*`
- R2: `isRejoinBlocked` 双次锁竞争
- R3: `ClearRoom` map 遍历中删除
- R4: hub_cluster 收集 key 后并发删除
- R5: `Signal` handler 空实现
- R6: `hub_mute.go:89` KV 循环 context 泄漏

**Round 2（本 round）**
- B2: cluster/leader.go:61 死代码
- R1: cluster/leader.go:75 RenewLoop 静默放弃
- R3: srs/client.go:115 同步 HTTP 无并发
- R5: 历史密钥 + Redis 不可用 = blacklist 绕过
- R1-alt: agora clearMuteRule 恢复失败静默
