# API 参考

## 基础信息

- **Base URL**: `/api/v1`
- **响应格式**:

```json
// 成功
{ "code": 0, "msg": "success", "data": { ... } }

// 错误
{ "code": 1011, "msg": "user not found", "data": null }
```

## 认证与鉴权（RBAC）

- 公开路由无需 Token；受保护路由必须在 `Authorization: Bearer <token>` 中携带 access_token。
- `protected` 路由组统一经过 `JWTAuth()` + `BanCheck()`。
- 管理类路由额外使用 `RequirePermission(permcode)`，权限码见 [权限模型](/guide/permissions)。
- 部分路由支持「资源所有者或权限」放行（`RequireOwnerOrPermission`）。

## 认证 API

### 登录
`POST /api/v1/auth/login` — `{ "username": "demo", "password": "123456" }`

### 注册
`POST /api/v1/auth/register` — `{ "username": "demo", "password": "123456" }`

### 刷新 Token
`POST /api/v1/auth/refresh_token`（公开，用 refresh_token 换 access_token）

### 修改密码
`POST /api/v1/auth/change_password`（JWT）— `{ "old_password": "...", "new_password": "..." }`

### 首次修改密码
`POST /api/v1/auth/first_change_password`（JWT）— `{ "new_password": "..." }`

### 登出
`POST /api/v1/auth/logout`（JWT）— 将 JTI 写入 Redis 黑名单

### 重置密码
`POST /api/v1/auth/reset_password`（公开）

## 用户 API（JWT）

| 方法 | 路径 | 权限 | 说明 |
|------|------|------|------|
| POST | `/api/v1/user/profile` | — | 当前用户资料 |
| POST | `/api/v1/user/info` | — | 按用户名查资料 |
| POST | `/api/v1/user/update-profile` | — | 更新资料 |
| POST | `/api/v1/user/upload-avatar` | — | 上传头像 |
| POST | `/api/v1/user/list` | `user:read` | 用户列表 |
| POST | `/api/v1/user/get` | `user:read` | 按 ID 查用户 |
| POST | `/api/v1/user/delete` | `user:delete` | 删除用户 |
| POST | `/api/v1/user/update-role` | `user:update` | 更新用户角色 |

## 房间 API（JWT + `room:*`）

| 方法 | 路径 | 权限 | 说明 |
|------|------|------|------|
| POST | `/api/v1/room/create` | `room:create` | 创建房间（密码、人数上限、音视频、观众席）|
| POST | `/api/v1/room/list` | `room:read` | 房间列表 |
| POST | `/api/v1/room/get` | `room:read` | 房间详情 |
| POST | `/api/v1/room/update` | `room:update` | 更新房间 |
| POST | `/api/v1/room/delete` | `room:delete` | 删除房间 |

## 角色 API（JWT + `role:*`）

| 方法 | 路径 | 权限 | 说明 |
|------|------|------|------|
| POST | `/api/v1/role/list` | `role:read` | 角色列表 |
| POST | `/api/v1/role/create` | `role:manage` | 创建角色 |
| POST | `/api/v1/role/update` | `role:manage` | 更新角色 |
| POST | `/api/v1/role/delete` | `role:manage` | 删除角色 |

## 权限 API（JWT + `role:*`）

| 方法 | 路径 | 权限 | 说明 |
|------|------|------|------|
| POST | `/api/v1/permission/list` | `role:read` | 全部权限定义 |
| POST | `/api/v1/permission/role` | `role:read` | 某角色的权限 |
| POST | `/api/v1/permission/sync` | `role:manage` | 同步角色-权限关联 |

## 禁言 API（JWT + `mute:manage`）

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/v1/mute/create` | 创建用户级禁言（永久/限时）|
| POST | `/api/v1/mute/cancel` | 取消禁言 |
| POST | `/api/v1/mute/status` | 查询禁言状态 |
| POST | `/api/v1/mute/list` | 禁言列表 |

## 对象存储 API（JWT）

| 方法 | 路径 | 权限 | 说明 |
|------|------|------|------|
| POST | `/api/v1/storage/presign` | — | 预签名上传 |
| POST | `/api/v1/storage/confirm` | — | 确认上传 |
| POST | `/api/v1/storage/upload` | — | 直传 |
| POST | `/api/v1/storage/delete` | `storage:delete` | 删除对象 |
| POST | `/api/v1/storage/config` | `storage:read` | 存储配置 |
| POST | `/api/v1/storage/update-config` | `storage:manage` | 更新存储配置 |

## Bot API（JWT + `bot:manage`）

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/v1/bot/create` | 签发 Bot 令牌（权限白名单）|
| POST | `/api/v1/bot/list` | Bot 列表 |
| POST | `/api/v1/bot/revoke` | 吊销 Bot 令牌 |

## 邮箱 API（JWT）

| 方法 | 路径 | 权限 | 说明 |
|------|------|------|------|
| POST | `/api/v1/email/send_code` | — | 发送验证码 |
| POST | `/api/v1/email/verify_code` | — | 校验验证码 |
| POST | `/api/v1/email/config` | `email_config:read` | 邮箱配置 |
| POST | `/api/v1/email/update-config` | `email_config:manage` | 更新邮箱配置 |

## 信令 API

### 获取加入 Token
`POST /api/v1/signal/token`（JWT）

```json
// Request
{ "room": "lobby", "identity": "user-xxx" }
// Response
{ "code": 0, "data": { "token": "eyJ...", "serverUrl": "wss://...", "room": "lobby", "identity": "user-xxx" } }
```

SRS 模式下还包含：`whipUrl`、`stream` 等 WHIP/WHEP 字段。

### 信令消息
`POST /api/v1/signal/signal`

### 列出房间 / 参与者
`GET /api/v1/signal/rooms` · `GET /api/v1/signal/participants?room=lobby`

### LiveKit Webhook
`POST /api/v1/signal/webhook`

### Cloudflare 媒体会话（JWT）
`POST /api/v1/signal/cloudflare/sessions/:sessionId/tracks/new` ·
`PUT /api/v1/signal/cloudflare/sessions/:sessionId/renegotiate` ·
`PUT /api/v1/signal/cloudflare/sessions/:sessionId/tracks/close` ·
`DELETE /api/v1/signal/cloudflare/sessions/:sessionId`

## SFU 配置 API（JWT + `sfu:manage`）

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/v1/sfu/config` | 当前激活 provider 配置 |
| POST | `/api/v1/sfu/config/:provider` | 指定 provider 配置 |
| POST | `/api/v1/sfu/update-config` | 更新配置 |
| POST | `/api/v1/sfu/switch-provider` | 切换激活 provider |
| POST | `/api/v1/sfu/providers` | 列出全部 provider 配置 |

## OAuth API

### 登录跳转 / 回调
`GET /api/v1/oauth/login/:provider` · `GET /api/v1/oauth/callback/:provider?code=xxx&state=yyy`

### 启用提供商列表（公开）
`GET /api/v1/oauth/providers`

### 管理员：提供商管理（JWT + `oauth:*`）

| 方法 | 路径 | 权限 |
|------|------|------|
| GET | `/api/v1/oauth/admin/providers` | `oauth:read` |
| POST | `/api/v1/oauth/admin/providers` | `oauth:manage` |
| PUT | `/api/v1/oauth/admin/providers` | `oauth:manage` |
| DELETE | `/api/v1/oauth/admin/providers/:id` | `oauth:manage` |

## SRS 回调
`POST /api/v1/srs/callback`（SRS `http_hooks` 调用，公开）

## 系统 API
`GET /api/v1/system/stream`（健康检查流，公开）

## 其他

### 健康检查
`GET /ping`

### Swagger 文档
`GET /swagger/index.html`

### 静态资源
`GET /uploads/*`（头像/上传文件）

## WebSocket 事件

**端点**: `ws://<host>/socket.io/`

### 客户端 → 服务端
| 事件 | 数据 | 说明 |
|------|------|------|
| `room:create` | `{ room }` | 创建房间 |
| `room:join` | `{ room, identity }` | 加入房间 |
| `room:leave` | `{ room }` | 离开房间 |
| `room:list` | — | 请求房间列表 |

### 服务端 → 客户端
| 事件 | 数据 | 说明 |
|------|------|------|
| `room:created` | `RoomInfo` | 房间已创建 |
| `room:joined` | `{ room, members[] }` | 已加入房间 |
| `room:left` | `{ room, identity }` | 已离开房间 |
| `room:updated` | `RoomInfo` | 房间更新 |
| `member:joined` | `MemberInfo` | 成员加入 |
| `member:left` | `{ identity }` | 成员离开 |
| `member:updated` | `MemberInfo` | 成员更新 |
| `user:muted` / `user:unmuted` | 禁言状态变更 | 用户级仅收听切换 |
| `room:list:result` | `{ rooms[] }` | 房间列表 |

## 错误码

| Code | 说明 |
|------|------|
| 0 | 成功 |
| 1001-1014 | 认证错误（Token 不存在/错误/过期/撤销/版本失效等）|
| 1010 | 密码错误 |
| 1011 | 用户不存在 |
| 1012 | 用户名已存在 |
| 1013 | 禁止访问（角色封禁 / 权限不足）|
| 2001 | 参数错误 |
| 3001 | 资源不存在 |
| 3002 | 资源已存在 |
| 5001 | 服务器内部错误 |
| 6001-6002 | SFU 错误 |
| 7001-7004 | OAuth 错误 |
