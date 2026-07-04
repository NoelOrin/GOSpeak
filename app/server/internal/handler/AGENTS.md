# handler 模块

HTTP 请求处理层，负责接收请求、参数校验、调用 service 层并返回响应。

## 文件说明

| 文件 | 职责 |
|------|------|
| auth_handler.go | 认证相关接口：登录、注册、刷新 Token、登出（将 Token JTI 写入 Redis 黑名单，立即使令牌失效） |
| user_handler.go | 用户管理接口：获取资料（通过 JWT UUID 查询）、列表、详情、删除 |
| signal_handler.go | 信令相关接口：获取加入 Token、交换信令消息、房间列表、参与者列表 |
| oauth_handler.go | OAuth 第三方登录接口：跳转第三方授权页、回调处理、提供商配置 CRUD |

## signal_handler 特殊逻辑

`SignalHandler` 同时持有 `sfuProvider` 和 `signal.Hub`，区分 SFU 层与信令层职责：

| 端点 | SFU 层 | 失败时行为 |
|------|--------|-----------|
| `POST /signal/token` | `sfuProvider.GenerateToken()` | 透传错误，不可降级 |
| `GET /signal/rooms` | `sfuProvider.ListRooms()` | 返回 `[]`（SFU 房间 ≠ 业务房间） |
| `GET /signal/participants` | `sfuProvider.ListParticipants()` | 返回 `[]`（SFU 参与者 ≠ WS 在线成员） |

SFU ListRooms/ListParticipants 失败时不 fallback 到信令层数据，
因为 SFU 媒体节点状态与信令层在线成员是两个独立概念。

## 依赖关系

handler → service → repository → model

handler 不直接操作数据库，仅通过 service 层完成业务逻辑。
