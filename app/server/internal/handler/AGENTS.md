# handler 模块

HTTP 请求处理层，负责接收请求、参数校验、调用 service 层并返回统一响应。

## 文件说明

| 文件 | 职责 |
|------|------|
| auth_handler.go | 登录、注册、刷新 Token、登出（JTI 写入 Redis 黑名单）、改密、首次改密、重置密码 |
| user_handler.go | 资料、按名查询、更新资料、上传头像、列表、详情、删除、更新角色 |
| signal_handler.go | 加入 Token、信令消息、房间列表、参与者列表、Webhook |
| oauth_handler.go | 第三方登录跳转、回调、启用提供商列表、提供商管理 CRUD |
| role_handler.go | 角色列表/创建/更新/删除 |
| permission_handler.go | 权限列表、角色权限查询、角色权限同步 |
| mute_handler.go | 禁言创建/取消/状态/列表 |
| room_handler.go | 房间 CRUD |
| storage_handler.go | 预签名/确认/上传/删除/配置 |
| bot_handler.go | Bot 令牌创建/列表/吊销 |
| email_verification_handler.go | 发送/校验邮箱验证码 |
| email_config_handler.go | 邮箱配置读取/更新 |
| sfu_config_handler.go | SFU 配置读取/更新/切换/列出 provider |
| srs_callback_handler.go | SRS HTTP Hooks 回调 |
| cloudflare_handler.go | Cloudflare Realtime 会话 tracks 管理 |
| monitor_handler.go | 健康检查流 |
| guild_handler.go | Guild 创建/查询/列表/加入/离开/踢出/成员 |
| conversation_handler.go | 会话列表/消息历史/标记已读 |
| message_handler.go | 消息发送/编辑/删除/表情回应 |
| plugin_handler.go | 插件列表/详情/更新 |

## signal_handler 特殊逻辑

`SignalHandler` 同时持有 `sfuProvider` 和 `signal.Hub`，区分 SFU 层与信令层职责：

| 端点 | SFU 层 | 失败时行为 |
|------|--------|-----------|
| `POST /signal/token` | `sfuProvider.GenerateToken()` | 透传错误，不可降级 |
| `GET /signal/rooms` | `sfuProvider.ListRooms()` | 返回 `[]`（SFU 房间 ≠ 业务房间）|
| `GET /signal/participants` | `sfuProvider.ListParticipants()` | 返回 `[]`（SFU 参与者 ≠ WS 在线成员）|

SFU ListRooms/ListParticipants 失败时不 fallback 到信令层数据，
因为 SFU 媒体节点状态与信令层在线成员是两个独立概念。

## 依赖关系

handler → service → repository → model

handler 不直接操作数据库，仅通过 service 层完成业务逻辑。
