# signal 模块

WebSocket 信令层，基于 Socket.IO 实现实时通信。

## 文件说明

| 文件 | 职责 |
|------|------|
| events.go | 事件名常量（14 个）|
| types.go | `RoomRequest`、`MemberInfo`、`RoomInfo` 等结构 |
| hub.go | 信令中心：连接、房间成员、事件处理、SFU 分发 |

## Hub struct

```
Hub.sfuProvider      — SFU provider 接口
Hub.sfuProviderName  — 初始化时从 `Provider.ProviderName()` 获取；dispatch 已改为按 `ErrSFUNotSupported` 优雅降级，不再依赖此字段做硬编码分支
Hub.rooms            — map[string]*Room, roomName → 在线房间（有 WS 连接的）
```

### 房间查询方法

| 方法 | 返回 |
|------|------|
| `GetSFURooms()` | 仅内存活跃房间（有 WS 连接的），供 `room:list` 广播 |
| `GetRooms()` | DB 持久化房间 + 内存活跃房间合并 |
| `GetRoomMembers(room)` | 指定房间的在线成员 |

### SFU dispatch 逻辑

`OnRoomKick` 信令层始终先处理（删 Members + 广播），随后由 `Hub.removeParticipantSafe` 直接调用 `sfuProvider.RemoveParticipant(room, identity)`。Hub **不再硬编码 provider 名**，仅在 provider 返回 `pkg.ErrSFUNotSupported` 时静默跳过，因此「踢人是否真正到达 SFU」由各 provider 自身是否实现 `RemoveParticipant` 决定：

| provider | `sfuProvider.RemoveParticipant` 调用 | 实现状态 |
|----------|------------------------------------|----------|
| livekit  | ✅ LiveKit gRPC SDK | 原始完整实现 |
| srs      | ✅ SRS REST API (`DELETE /clients/{id}`) | 原始完整实现 |
| mediasoup| ✅ bridge `CloseParticipant` | 补全实现（历史文档误标为跳过） |
| daily    | ✅ list → 按 session id `RemoveParticipant` | 补全实现（历史文档误标为跳过） |
| agora    | ❌ 跳过（返回 `ErrSFUNotSupported`，无单用户踢人 REST API） | 未实现，仅 ban 语义 |
| cloudflare | ❌ 跳过（返回 `ErrSFUNotSupported`，WHIP/WHEP 媒体无单用户踢人 REST） | 未实现 |

Mute/ListParticipants 不由信令层触发 SFU 调用——靠前端 Socket.IO 事件协作。

## 语义约束

- 禁言是用户级限制，不是房间级静音。
- `user:muted` / `user:unmuted` 表示用户被切换为仅收听 / 恢复可发布状态。
- `room:mute` / `member:muted` 旧房间级静音链路已移除。
- SFU 房间 ≠ 业务房间，两者状态不互相 fallback。

## 依赖

使用 `github.com/googollee/go-socket.io` 实现 WebSocket 通信。
