# API 参考

## 基本约定

所有接口前缀 `/api/v1`。

### 响应格式

成功：

```json
{ "code": 0, "msg": "success", "data": { ... } }
```

失败时 `data` 恒为 `null`：

```json
{ "code": 1011, "msg": "user not found", "data": null }
```

### 认证

受保护的接口要在请求头带令牌：

```
Authorization: Bearer <access_token>
```

没带、过期、被吊销都会返回认证类错误码。

### 错误码

| 范围 | 含义 |
|------|------|
| `0` | 成功 |
| `1001`-`1014` | 认证与权限 |
| `2001` | 参数错误 |
| `3001`-`3002` | 资源不存在 / 已存在 |
| `5001` | 服务器内部错误 |
| `6001`-`6002` | SFU 相关 |
| `7001`-`7004` | OAuth 相关 |
| `8001`-`8008` | 邮箱相关 |

常用几个：

| 码 | 含义 |
|----|------|
| `1001` | 未登录 / 令牌缺失 |
| `1013` | 权限不足 |
| `1014` | 令牌已吊销 |
| `2001` | 参数校验失败 |
| `3001` | 资源不存在 |

完整列表见 [错误码](/reference/errors)。

## 认证

| 方法 | 路径 | 认证 | 说明 |
|------|------|------|------|
| POST | `/auth/login` | 否 | 用户名密码登录 |
| POST | `/auth/register` | 否 | 注册 |
| POST | `/auth/refresh_token` | 否 | 用 refresh_token 换新的 |
| POST | `/auth/refresh` | 是 | 刷新当前会话 |
| POST | `/auth/reset_password` | 否 | 重置密码 |
| POST | `/auth/change_password` | 是 | 修改密码 |
| POST | `/auth/first_change_password` | 是 | 首次登录强制改密 |
| POST | `/auth/logout` | 是 | 登出（令牌进黑名单） |
| POST | `/auth/guest` | 否 | 用邀请码换访客身份（限流 60 次/小时） |

登录请求：

```json
{ "username": "demo", "password": "123456" }
```

登录响应里的 `data` 包含 `access_token`、`refresh_token`、`expires_in`、`user`。

## 用户

| 方法 | 路径 | 权限 | 说明 |
|------|------|------|------|
| POST | `/user/profile` | 登录 | 自己的资料 |
| POST | `/user/info` | 登录 | 按用户名查 |
| GET | `/user/preset-avatars` | 登录 | 预置头像列表 |
| POST | `/user/update-profile` | 登录 | 改自己的资料 |
| POST | `/user/list` | `user:read` | 用户列表 |
| POST | `/user/get` | `user:read` | 按 ID 查 |
| POST | `/user/delete` | `user:delete` | 删用户 |
| POST | `/user/update-role` | `user:update` | 改角色 |

头像没有独立上传接口，走存储的预签名流程后把 URL 传给 `update-profile`。

## 房间

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/room/create` | 建房间（需要平台管理员或域成员） |
| POST | `/room/list` | 房间列表 |
| POST | `/room/get` | 房间详情 |
| POST | `/room/update` | 更新 |
| POST | `/room/delete` | 删除 |

建房间可以带：

| 字段 | 说明 |
|------|------|
| `name` | 房间名 |
| `type` | `voice`（默认）或 `text` |
| `password` | 进房密码 |
| `limit` | 人数上限 |
| `audio_only` | 是否只开音频 |
| `allow_audience` | 是否允许观众席 |

## 消息

发消息走 WebSocket，HTTP 只有查询和反应：

| 方法 | 路径 | 权限 | 说明 |
|------|------|------|------|
| POST | `/room/messages/list` | `message:read` | 历史消息（游标分页） |
| POST | `/room/messages/search` | `message:read` | 搜索 |
| POST | `/room/messages/react` | `message:send` | 加表情反应 |

## 私聊

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/conversation/list` | 会话列表 |
| POST | `/conversation/messages` | 消息列表 |
| POST | `/conversation/send` | 发消息 |
| POST | `/conversation/mark-read` | 标记已读 |

## 域

| 方法 | 路径 | 权限 | 说明 |
|------|------|------|------|
| POST | `/domain/create` | `domain:create` | 建域（限流 10 次/分钟） |
| POST | `/domain/get` | 域成员 | 详情 |
| POST | `/domain/list` | `domain:read` | 全部域 |
| POST | `/domain/list-public` | 否 | 公开域（不登录也能看） |
| POST | `/domain/my-domains` | 登录 | 我加入的 |
| POST | `/domain/update` | 域成员 | 改设置 |
| POST | `/domain/delete` | 域成员 | 删除 |
| POST | `/domain/join` | 登录 | 加入 |
| POST | `/domain/leave` | 域成员 | 退出 |
| POST | `/domain/kick` | 域成员 | 踢人 |
| POST | `/domain/members` | 域成员 | 成员列表 |
| POST | `/domain/preview` | 否 | 预览（看邀请码对应哪个域） |
| POST | `/domain/reset-invite` | 域成员 | 重置邀请码 |

### 域角色

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/domain/roles/list` | 角色列表 |
| POST | `/domain/roles/create` | 建角色 |
| POST | `/domain/roles/update` | 改角色 |
| POST | `/domain/roles/delete` | 删角色 |

### 访客

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/domain/guest/config` | 读/写访客开关 |
| POST | `/domain/guest/ban` | 封禁访客 |
| POST | `/domain/guest/unban` | 解封 |
| POST | `/domain/guest/ban-list` | 封禁列表 |
| POST | `/domain/guest/cleanup` | 清理过期访客 |
| POST | `/auth/guest/renew` | 续期访客身份 |

## 角色与权限

| 方法 | 路径 | 权限 | 说明 |
|------|------|------|------|
| POST | `/role/list` | `role:read` | 角色列表 |
| POST | `/role/create` | `role:manage` | 建角色 |
| POST | `/role/update` | `role:manage` | 改角色 |
| POST | `/role/delete` | `role:manage` | 删角色 |
| POST | `/permission/list` | `role:read` | 全部权限定义 |
| POST | `/permission/role` | `role:read` | 某角色的权限 |
| POST | `/permission/sync` | `role:manage` | 同步角色权限 |

## 禁言

| 方法 | 路径 | 权限 | 说明 |
|------|------|------|------|
| POST | `/mute/create` | `mute:manage` | 创建禁言 |
| POST | `/mute/cancel` | `mute:manage` | 取消 |
| POST | `/mute/status` | `mute:manage` | 查状态 |
| POST | `/mute/list` | `mute:manage` | 列表 |

## 信令与语音

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/signal/token` | 拿进房令牌 |
| GET | `/signal/rooms` | 当前房间 |
| GET | `/signal/participants` | 房间成员 |
| GET | `/signal/ws-endpoint` | 信令地址 |
| POST | `/signal/webhook` | LiveKit 回调（公开，靠签名校验） |

## SFU 配置

| 方法 | 路径 | 权限 | 说明 |
|------|------|------|------|
| POST | `/sfu/config` | `sfu:manage` | 当前配置 |
| POST | `/sfu/config/:provider` | `sfu:manage` | 指定 provider 的配置 |
| POST | `/sfu/update-config` | `sfu:manage` | 改配置并激活 |
| POST | `/sfu/switch-provider` | `sfu:manage` | 只切换，不改配置 |
| POST | `/sfu/providers` | `sfu:manage` | 列出已配置的 |

## 存储

| 方法 | 路径 | 权限 | 说明 |
|------|------|------|------|
| POST | `/storage/presign` | 登录 | 拿上传地址 |
| POST | `/storage/confirm` | 登录 | 确认上传 |
| POST | `/storage/upload` | 登录 | 直传 |
| POST | `/storage/delete` | `storage:delete` | 删对象 |
| POST | `/storage/config` | `storage:read` | 读配置 |
| POST | `/storage/update-config` | `storage:manage` | 改配置 |
| POST | `/storage/test-config` | `storage:read` | 测连通性 |

## 邮件

| 方法 | 路径 | 权限 | 说明 |
|------|------|------|------|
| POST | `/email/send_code` | 否 | 发验证码（限流 5 次/分钟） |
| POST | `/email/verify_code` | 否 | 校验验证码 |
| POST | `/email/config` | `email_config:read` | 读配置 |
| POST | `/email/update-config` | `email_config:manage` | 改配置 |

## OAuth

| 方法 | 路径 | 权限 | 说明 |
|------|------|------|------|
| GET | `/oauth/providers` | 否 | 可用的登录方式 |
| GET | `/oauth/login/:provider` | 否 | 发起授权 |
| GET | `/oauth/callback/:provider` | 否 | 平台回调 |
| GET | `/oauth/admin/providers` | `oauth:read` | 后台列表 |
| POST | `/oauth/admin/providers` | `oauth:manage` | 新增 |
| PUT | `/oauth/admin/providers` | `oauth:manage` | 修改 |
| DELETE | `/oauth/admin/providers/:id` | `oauth:manage` | 删除 |

## 用户组

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/user-group/list` | 列表 |
| POST | `/user-group/create` | 创建 |
| POST | `/user-group/update` | 更新 |
| POST | `/user-group/delete` | 删除 |

## 审计与系统

| 方法 | 路径 | 权限 | 说明 |
|------|------|------|------|
| POST | `/audit/list` | `audit:read` | 审计日志 |
| GET | `/system/stream` | 登录 | 服务状态流（SSE） |

## 健康检查

| 路径 | 说明 |
|------|------|
| `GET /ping` | 进程活着就返回 200，不碰数据库 |
| `GET /readyz` | 会 ping 数据库，挂了返回 503 |
| `GET /metrics` | Prometheus 指标，设了 `METRICS_TOKEN` 要带 Bearer |

## WebSocket

信令走 `/ws`，不是 HTTP 接口。事件列表见 [信令事件](/reference/events)。

## 交互式文档

服务跑起来后访问 `/swagger/index.html`，能直接在页面上试接口。

## 下一步

- [错误码](/reference/errors) — 完整错误码表
- [信令事件](/reference/events) — WebSocket 事件
- [权限说明](/guide/permissions) — 权限码含义
