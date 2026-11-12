# GOSpeak Caveman Review — Round 4 (Frontend) — 二次验证版

Generated: 2026-08-05
Scope: stores/socketStore.ts, stores/chatStore.ts, stores/voiceChatStore.ts, socket/wsClient.ts, utils/idb-cache.ts

---

## 二次验证结论

| # | Severity | File:Line | Finding | 验证结果 |
|---|----------|-----------|---------|----------|
| F1 | 🔵 | socketStore.ts:269 | signalEmit 返回 Promise<any> 类型丢失 | ✅ 确认 |
| F2 | 🔵 | wsClient.ts:247 | emitAck state 检查后 ws.send 前有竞态 | ✅ 可接受（固有竞态）|
| F3 | 🔵 | idb-cache.ts:152 | renameConversation 事务原子性 | ✅ 确认（理论风险）|
| F4 | 🔵 | idb-cache.ts:49 | onupgradeneeded 无版本迁移 | ✅ 确认 |
| **F5** | **🔴** | **chatStore.ts:278** | **applyCreated 不检查 nonce 状态，ACK 先到时 server message 重复添加** | **✅ 确认（BUG）** |
| F6 | 🔵 | socketStore.ts:202 | leaveRoom 依赖 namespace 广播 | ✅ 确认（双重保障）|
| F7 | 🔵 | wsClient.ts:215 | emitFireAndForget 返回 false toast | ✅ 确认 |
| F8 | ✅ | socketStore.ts:280 | disconnect 后 serverEventsBound 重置 | ✅ 确认（安全）|
| F9 | 🔵 | voiceChatStore.ts:63 | await-to-js to() 忽略 error 后 throw | ✅ 确认 |
| F10 | ✅ | chatStore.ts:224 | crypto.randomUUID collision | ✅ 忽略（2^-122 概率）|

---

## 🔴 Bug — chatStore.ts:278: applyCreated 不检查 nonce 状态

**代码：**
```ts
function applyCreated(dto: MessageDTO) {
    setMessages((prev) => {
        if (dto.client_nonce && pendingNonces.has(dto.client_nonce)) {
            pendingNonces.delete(dto.client_nonce);
            return prev
                .filter((m) => m.client_nonce !== dto.client_nonce)
                .concat(dto)
                .sort(...);
        }
        return mergeMessages(prev, [dto]);
    });
}
```

**ACK handler：**
```ts
.then((resp: any) => {
    pendingNonces.set(client_nonce, "sent");  // ← ACK 先到后 nonce 设为 "sent"
})
```

**Bug 流程：**
1. 发消息（nonce="pending"），optimistic 消息显示
2. ACK 先到 → nonce 设为 "sent"，server 消息加入 UI
3. message:created 后到 → `pendingNonces.has(dto.client_nonce)` 返回 true（nonce 存在，状态="sent"），**再次执行 filter+concat**，server 消息被添加第二次 → **重复消息**

**修复：**
```ts
if (dto.client_nonce && pendingNonces.get(dto.client_nonce) === "pending") {
    pendingNonces.delete(dto.client_nonce);
    return prev
        .filter((m) => m.client_nonce !== dto.client_nonce)
        .concat(dto)
        .sort(...);
}
// nonce 不存在或状态为 "sent" → 作为普通消息处理（忽略 duplicate）
return mergeMessages(prev, [dto]);
```

---

## 待修复项汇总

### 🔴 Bug（需修复）
1. `auth_service.go:89,288` — GetByName 忽略 DB 错误
2. `chatStore.ts:278` — applyCreated nonce 状态未检查导致重复消息

### 🟡 Risk（建议关注）
1. `cluster/leader.go:75` — RenewLoop Create 失败静默放弃 leader
2. `srs/client.go:115` — KickByStreams 同步 HTTP 无并发无超时
3. `pkg/jwt.go:57` — 历史密钥 + Redis 不可用 = blacklist 绕过
4. `mute_service.go:40` — DeleteByUserID 失败仍 notifyExpired
5. `repository/*` — gorm.ErrRecordNotFound 系统性用 `==`（Round 1 已报）
6. `auth_service.go:67` — bcrypt CompareHashAndPassword error 时 needChange 错

### 🔵 Nit（可改进）
1. `signalEmit` 返回 `Promise<any>` — 类型丢失
2. `idb-cache.ts:49` — 无 DB 版本迁移
3. `voiceChatStore.ts:63` — loadPersistedState error throw
4. `cluster/leader.go:61` — strings.Contains 分支死代码
5. `livekit/client.go:88` — NOT_FOUND 用 SFU_ERROR 码
6. `agora/provider.go:67` — 每次 NewRESTClient
