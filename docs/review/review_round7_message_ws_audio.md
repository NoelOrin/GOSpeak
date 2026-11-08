# GOSpeak Caveman Review — Round 7 (Message Service / WS Fanout / Frontend Audio / OAuth)

Generated: 2026-08-05
Scope: service/message_service.go, ws/fanout.go, ws/handler.go, frontend handler_audio/*, handler/oauth_handler.go

---

## 验证结论

| # | Severity | File:Line | Finding | Verified |
|---|----------|-----------|---------|----------|
| M1 | 🟡 | `message_service.go:154` | `enrichAuthorInfo` 循环调用 `GetByName` 每个 author 一次，N+1 查询 | ✅ 确认 |
| M2 | ✅ | `message_service.go:enrichAuthorInfo` | author 查不到时不 panic，返回原字段 | ✅ 确认（fail-open）|
| W1 | ✅ | `ws/fanout.go:76` | BroadcastToRoom 正确拷贝成员指针后释放锁，避免持锁 Send | ✅ 确认（正确）|
| W2 | 🔵 | `ws/handler.go:53` | Dispatch 中 dataStr 空值处理 `"null"` vs `""` | ✅ 确认（"null" 是字符串，"" 是空字符串，逻辑正确）|
| F1 | ✅ | `handler_audio/index.ts:effectiveVolume` | masterMuted 时返回 0，逻辑正确 | ✅ 确认 |
| F2 | 🔵 | `handler_audio/index.ts:30` | `track.setVolume(effectiveVolume(identity))` 在 masterMuted 时静默设置 volume=0，视觉上无声 | ✅ 确认（正确）|
| O1 | ✅ | `oauth_handler.go:43` | OAuth state 使用 HttpOnly SameSite=Lax cookie，CSRF 保护正确 | ✅ 确认 |
| O2 | 🔵 | `oauth_handler.go:130` | `randomState` 使用 `crypto/rand` 生成高熵 state | ✅ 确认（安全）|
| A1 | 🔵 | `ws/fanout.go:Remove` | `Remove` 返回的是本 client 被移除的房间列表，不含已清空的房间（空 members 被 delete 后不再加入返回列表）| ⚠️ 需确认 |

---

## 🟡 Risk — message_service.go:154: enrichAuthorInfo N+1 查询

**代码：**
```go
func (s *MessageService) enrichAuthorInfo(items []MessageDTO) {
    authorIDs := make(map[string]struct{})
    for _, item := range items {
        if item.AuthorID != "" {
            authorIDs[item.AuthorID] = struct{}{}
        }
    }
    // 收集完成
    users := make(map[string]*model.User)
    for id := range authorIDs {  // N 次 DB 查询！
        if user, err := s.userRepo.GetByName(id); err == nil && user != nil {
            users[id] = user
        }
    }
    // ...
}
```

**问题：** N 条消息涉及 M 个不同 author，执行 M 次 `GetByName` 查询。N=100、M=10 时产生 10 次 DB 往返。消息列表查询（通常 N=50-200）会产生等量 DB 查询。

**修复：** 已有 `GetByNames(names []string)` 在 user_repo.go，改为批量调用：
```go
names := make([]string, 0, len(authorIDs))
for id := range authorIDs { names = append(names, id) }
users, err := s.userRepo.GetByNames(names)
```

---

## 🔵 Nit

**W1 — ws/handler.go:53: dataStr 空值处理的边界情况**

```go
if dataStr == "null" || dataStr == `""` { dataStr = "" }
```

当服务端 handler 期望 `data string` 且收到 `null` JSON 时（Go 中为字符串 `"null"`），此检查正确将其转为空字符串。`dataStr == ""` 处理的是空 JSON 字符串 `""`（即 Go 字符串 `""`）。

但若客户端发送 `{}`（空对象），`dataStr` 为 `"{}"`，handler 收到 `{}` 而非空字符串。可能被某些 handler 视为非法 payload。

→ 注释说明哪些事件期望空字符串，哪些期望 `{}`，避免歧义。

**A1 — ws/fanout.go:Remove 返回值语义**

```go
func (f *Fanout) Remove(clientID string) []string {
    // ...
    for room, members := range f.rooms {
        if _, ok := members[clientID]; ok {
            delete(members, clientID)
            rooms = append(rooms, room)  // ← 空房也被加入返回值
            if len(members) == 0 {
                delete(f.rooms, room)    // ← 但立即被删除
                // 空的 room 仍被包含在返回值中！
            }
        }
    }
    return rooms
}
```

当 client 是房间最后一个成员时，`Remove` 返回的 `rooms` 列表仍包含该房间名（因为 `append` 在 `delete` 之前执行）。但 `Hub.OnDisconnect` 调用 `Remove` 后遍历返回值做 `deleteRoomIfEmptyLocked`，房间已不在 `f.rooms` 中，`deleteRoomIfEmptyLocked` 直接 return false，无副作用。

**实际影响：无，但返回值语义不精确**——返回的是"client 曾加入的房间"而非"client 离开后仍有成员的房间"。建议在 `len == 0` 后 continue，不加入返回值。

---

## ✅ 已验证非问题

| # | 描述 | 验证 |
|---|------|------|
| W2 | fanout.BroadcastToRoom 正确拷贝成员指针后释放锁再 Send | ✅ 确认 |
| F1 | effectiveVolume masterMuted 返回 0 实现静音 | ✅ 确认 |
| O1 | OAuth state HttpOnly SameSite=Lax cookie CSRF 保护 | ✅ 确认 |
| O2 | randomState 使用 crypto/rand 生成高熵 state | ✅ 确认 |
| M2 | enrichAuthorInfo author 查不到时 fail-open | ✅ 确认 |
| A1 | fanout.Remove 返回空房间列表无副作用（Hub 侧安全）| ✅ 确认 |

---

## 新增：Message Service 广播-持久化解耦设计确认

`message_service.go` 的 "broadcast-first + job queue" 模式：
1. 先 `bus.PublishRoom` 广播给在线客户端
2. 再异步 `queue.Publish` 持久化
3. queue 失败时同步回退 DB

这个模式设计合理，但 `broadcast` 和 `persist` 之间无原子性保证：若广播成功但持久化失败（且回退 DB 也失败），客户端收到消息但服务器重启后消息丢失。这是可选 trade-off，应在设计文档中注明。

---

## 待修复项（Round 7 新增）

| # | Severity | 描述 |
|---|----------|------|
| R-M1 | 🟡 | message_service.go enrichAuthorInfo N+1 查询，改为批量 GetByNames |
| N-A1 | 🔵 | ws/fanout.go Remove 返回值包含已清空的房间（语义不精确）|
