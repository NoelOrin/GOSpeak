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

## 依赖

使用 `github.com/googollee/go-socket.io` 实现 WebSocket 通信。
