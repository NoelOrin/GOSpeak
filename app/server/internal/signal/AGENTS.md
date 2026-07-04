# signal 模块

WebSocket 信令层，基于 Socket.IO 实现实时通信。

## 文件说明

| 文件 | 职责 |
|------|------|
| hub.go | 信令中心：管理连接、房间成员、事件处理 |

## 事件处理

| 事件 | 说明 |
|------|------|
| OnConnect | 客户端连接，记录日志 |
| OnDisconnect | 客户端断开，清理房间成员 |

## Hub struct

```
Hub.sfuProvider      — SFU provider 接口
Hub.sfuProviderName  — 初始化时从 Provider.ProviderName() 获取，供 dispatch 判断
Hub.rooms            — map[string]*Room, roomName → 在线房间（有 WS 连接的）
```

### 房间查询方法命名

| 方法 | 返回内容 |
|------|---------|
| `GetSFURooms()` | 仅内存活跃房间（有 WS 连接的），用于 `room:list` 广播 |
| `GetRooms()` | DB 持久化房间 + 内存活跃房间合并（带 DB 字段），供业务列表 |
| `GetRoomMembers(room)` | 指定房间的在线成员 |

### SFU dispatch 逻辑

`OnRoomKick` 信令层始终先处理（删 Members + 广播），然后按 provider 能力分发：

| provider | `sfuProvider.RemoveParticipant` 调用 |
|----------|------------------------------------|
| livekit  | ✅ LiveKit gRPC SDK |
| srs      | ✅ SRS REST API (`DELETE /clients/{id}`) |
| agora    | ❌ 跳过（无单用户踢人 REST API） |
| daily    | ❌ 跳过（无单用户踢人 REST API） |
| mediasoup| ❌ 跳过（无 media-level 管理能力） |

Mute/ListParticipants 不由信令层触发 SFU 调用——靠前端 Socket.IO 事件协作。

## 语义约束

- `signal` 层维护的是连接、房间成员、房间信令和部分用户状态广播。
- 当前产品语义中，**禁言是用户级限制**，不是房间级静音。
- `user:muted` / `user:unmuted` 表示用户被切换为仅收听 / 恢复可发布状态。
- `room:mute` / `member:muted` 旧房间级静音链路已移除，不应在新逻辑中重新引入。
- SFU 房间 ≠ 业务房间。SFU ListRooms/ListParticipants 返回的是媒体节点状态，
  与信令层在线成员无关，不应互相 fallback。

## 依赖

使用 `github.com/googollee/go-socket.io` 实现 WebSocket 通信。
