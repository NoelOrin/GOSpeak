# 信令事件

除 HTTP 接口外，前端还通过一条 WebSocket 长连接接收实时事件，包括成员加入、发言状态与新消息等。

## 连接

```
ws(s)://你的域名/ws
```

连接建立后需携带登录令牌完成认证。具体参数见前端实现（`app/web/src/socket/`）。

连接建立时会收到 `connection` 事件。

## 事件表

方向说明：`C→S` 为客户端发给服务端，`S→C` 为服务端推送给客户端。

### 房间

| 事件 | 方向 | 说明 |
|------|------|------|
| `room:create` | C→S | 创建房间 |
| `room:created` | S→C | 房间创建完成 |
| `room:join` | C→S | 加入房间 |
| `room:leave` | C→S | 离开房间 |
| `room:left` | S→C | 已离开 |
| `room:list` | C→S | 请求房间列表 |
| `room:list:result` | S→C | 房间列表结果 |
| `room:updated` | S→C | 房间信息变更 |
| `room:kick` | C→S | 移出成员 |
| `room:kicked` | S→C | 当前连接被移出房间 |
| `room:join:sfu` | C→S | 通知服务端即将连接 SFU |
| `room:active-speakers` | S→C | 当前正在发言的成员（批量） |

### 成员

| 事件 | 方向 | 说明 |
|------|------|------|
| `member:joined` | S→C | 成员加入 |
| `member:left` | S→C | 成员离开 |
| `member:updated` | S→C | 成员信息变更 |
| `member:speaking` | S→C | 某成员正在发言 |
| `member:mic-state` | S→C | 麦克风开关状态变更 |
| `member:muted` | S→C | 已被禁言 |
| `member:unmuted` | S→C | 禁言已解除 |

### 消息

| 事件 | 方向 | 说明 |
|------|------|------|
| `message:send` | C→S | 发送消息 |
| `message:edit` | C→S | 编辑消息 |
| `message:delete` | C→S | 删除消息 |
| `message:react` | C→S | 添加表情反应 |
| `message:unreact` | C→S | 移除表情反应 |
| `message:created` | S→C | 新消息广播 |
| `message:updated` | S→C | 消息已编辑 |
| `message:deleted` | S→C | 消息已删除 |
| `message:reaction` | S→C | 表情反应变更 |
| `message:ack` | S→C | 发送确认（带消息 ID） |
| `message:error` | S→C | 发送失败 |

### 私聊

| 事件 | 方向 | 说明 |
|------|------|------|
| `private:send` | C→S | 发送私聊消息 |
| `private:new` | S→C | 收到私聊消息 |

### 状态

| 事件 | 方向 | 说明 |
|------|------|------|
| `typing` | 双向 | 正在输入 |
| `user:muted` | S→C | 当前用户被禁言 |
| `user:unmuted` | S→C | 当前用户禁言被解除 |
| `sfu:provider-changed` | S→C | 语音服务已切换，需要重连 |
| `disconnect` | S→C | 连接断开 |

## 进房完整流程

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

关键点：**信令和媒体是两条独立的连接**。信令负责状态同步，媒体负责音频传输。

## 信令与媒体分离的原因

- 信令要求可靠（不能丢），走 TCP
- 媒体要求低延迟（能容忍少量丢包），走 UDP

两者混用会互相影响：音频卡顿不应导致状态同步延迟。

## 断线重连

前端会自动重连，重连后需要重新执行 `room:join` 以补齐状态。

重连期间状态可能不一致。服务端在 `room:join` 的响应中会返回完整成员列表，前端以该列表为准覆盖本地状态。

## 常见问题

**界面持续显示「已断开」**
反向代理未放行 WebSocket 升级（`Upgrade` / `Connection` 头）。见 [反代与 HTTPS](/deploy/nginx)。

**连接建立但收不到事件**
令牌已过期。WebSocket 连接建立后不会自动刷新令牌，需要重连。

**消息发出后无响应**
检查是否收到 `message:ack`。未收到即发送失败，需检查连接状态。收到 `message:error` 表示服务端拒绝了该消息，详见错误内容。

**成员列表不更新**
未收到 `member:joined` / `member:left`。通常为连接已断开但界面未感知，刷新页面可恢复。

## 下一步

- [API 参考](/reference/api) — HTTP 接口
- [错误码](/reference/errors) — 错误含义
