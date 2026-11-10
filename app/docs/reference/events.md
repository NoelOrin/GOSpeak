# 信令事件

除了 HTTP 接口，前端还通过一条 WebSocket 长连接接收实时事件——谁进来了、谁在说话、有新消息了。

## 连接

```
ws(s)://你的域名/ws
```

连接后要带上登录令牌做认证。具体参数看前端实现（`app/web/src/socket/`）。

连接建立时会收到 `connection` 事件。

## 事件表

方向说明：`C→S` 是客户端发给服务端，`S→C` 是服务端推给客户端。

### 房间

| 事件 | 方向 | 说明 |
|------|------|------|
| `room:create` | C→S | 建房间 |
| `room:created` | S→C | 房间建好了 |
| `room:join` | C→S | 加入房间 |
| `room:leave` | C→S | 离开房间 |
| `room:left` | S→C | 已离开 |
| `room:list` | C→S | 要房间列表 |
| `room:list:result` | S→C | 列表结果 |
| `room:updated` | S→C | 房间信息变了 |
| `room:kick` | C→S | 踢人 |
| `room:kicked` | S→C | 你被踢了 |
| `room:join:sfu` | C→S | 通知服务端要连 SFU |
| `room:active-speakers` | S→C | 当前谁在说话（批量） |

### 成员

| 事件 | 方向 | 说明 |
|------|------|------|
| `member:joined` | S→C | 有人进来了 |
| `member:left` | S→C | 有人走了 |
| `member:updated` | S→C | 成员信息变了 |
| `member:speaking` | S→C | 某人在说话 |
| `member:mic-state` | S→C | 麦克风开关状态变了 |
| `member:muted` | S→C | 被禁言 |
| `member:unmuted` | S→C | 解除禁言 |

### 消息

| 事件 | 方向 | 说明 |
|------|------|------|
| `message:send` | C→S | 发消息 |
| `message:edit` | C→S | 编辑 |
| `message:delete` | C→S | 删除 |
| `message:react` | C→S | 加表情 |
| `message:unreact` | C→S | 移除表情 |
| `message:created` | S→C | 新消息广播 |
| `message:updated` | S→C | 消息被编辑 |
| `message:deleted` | S→C | 消息被删除 |
| `message:reaction` | S→C | 反应变了 |
| `message:ack` | S→C | 发送确认（带消息 ID） |
| `message:error` | S→C | 发送失败 |

### 私聊

| 事件 | 方向 | 说明 |
|------|------|------|
| `private:send` | C→S | 发私聊 |
| `private:new` | S→C | 收到私聊 |

### 状态

| 事件 | 方向 | 说明 |
|------|------|------|
| `typing` | 双向 | 正在输入 |
| `user:muted` | S→C | 你被禁言了 |
| `user:unmuted` | S→C | 你被解除禁言 |
| `sfu:provider-changed` | S→C | 语音服务切了，需要重连 |
| `disconnect` | S→C | 连接断开 |

## 一次进房间的完整流程

```
前端                          后端                    LiveKit
 │                             │                        │
 ├─ POST /signal/token ───────►│                        │
 │◄──── 令牌 + 地址 ───────────┤                        │
 │                             │                        │
 ├─ ws 连接 ──────────────────►│                        │
 │                             │                        │
 ├─ room:join ────────────────►│                        │
 │                             ├─ 检查密码/人数/权限     │
 │                             │                        │
 │◄──── member:joined × N ─────┤                        │
 │◄──── room:active-speakers ──┤                        │
 │                             │                        │
 ├─ 用令牌连 LiveKit ──────────────────────────────────►│
 │◄───────────────────── 音频流 ───────────────────────┤
 │                             │                        │
 │◄──── member:speaking ───────┤◄─── active speaker ────┤
```

关键点：**信令和媒体是两条独立的连接**。信令管状态同步，媒体管音频传输。

## 为什么要两条连接

- 信令要可靠（不能丢），走 TCP
- 媒体要低延迟（能容忍少量丢包），走 UDP

混在一起会互相拖累：音频卡顿不应该导致状态同步延迟。

## 断线重连

前端会自动重连，重连后要重新走一遍 `room:join`，把状态补齐。

重连期间状态可能不一致，服务端在 `room:join` 的响应里会返回完整成员列表，前端以这个为准覆盖本地状态。

## 常见问题

**一直显示「已断开」**
反代没放行 WebSocket 升级（`Upgrade` / `Connection` 头）。见 [反代与 HTTPS](/deploy/nginx)。

**能连上但收不到事件**
令牌过期了。WebSocket 连接建立后不会自动刷新令牌，需要重连。

**消息发出去没反应**
看有没有收到 `message:ack`。没有的话是发送失败，检查连接状态。收到 `message:error` 说明服务端拒绝了，看错误内容。

**成员列表不更新**
`member:joined` / `member:left` 没收到。多半是连接断了但界面没感知到，刷新页面。

## 下一步

- [API 参考](/reference/api) — HTTP 接口
- [错误码](/reference/errors) — 错误含义
