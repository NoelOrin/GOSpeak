# 权限模型（RBAC）

GOSpeak 使用**基于权限（permission-based）的 RBAC**，在角色之上叠加细粒度权限控制。

## 核心概念

- **角色（Role）**：`roles` 表，启动时种子写入 `admin` / `user` / `ban`。`ban` 角色被 `BanCheck()` 中间件拦截，永远返回 403。
- **权限（Permission）**：`permissions` 表，每行是一个权限码（如 `user:read`、`room:create`）。
- **角色-权限关联（RolePermission）**：`role_permissions` 表，把角色名映射到权限 ID。
- **Bot 令牌（BotToken）**：JWT 中直接携带 `permissions` 列表；其权限受 `model.BotScopedPermissions` 白名单约束，无法触达平台级管理面。

## 权限码（`internal/permcode`）

| 权限码 | 含义 |
|--------|------|
| `user:read` / `user:update` / `user:delete` | 用户查询 / 更新 / 删除 |
| `role:read` / `role:manage` | 角色读取 / 管理（同时控制权限同步）|
| `room:create` / `room:read` / `room:update` / `room:delete` | 房间全生命周期 |
| `message:read` / `message:send` / `message:delete_others` | 文字消息读取 / 发送 / 删除他人消息 |
| `mute:manage` | 禁言管理 |
| `sfu:manage` | SFU provider 配置管理 |
| `storage:read` / `storage:manage` / `storage:delete` | 对象存储读取 / 配置 / 删除 |
| `bot:manage` | Bot 令牌管理 |
| `email_config:read` / `email_config:manage` | 邮箱配置读取 / 管理 |
| `oauth:read` / `oauth:manage` | OAuth 提供商读取 / 管理 |
| `signal:kick` | 信令踢人 |
| `plugin:read` / `plugin:manage` | 插件读取 / 管理 |

## 鉴权中间件

| 中间件 | 行为 |
|--------|------|
| `JWTAuth()` | 校验签名、过期、TokenVersion、黑名单 JTI，并向 context 注入 `username` / `role` / `permissions` 等 |
| `RequirePermission(permCode)` | 用户按 `role → 权限` 映射解析；Bot 用 Token 中的 `permissions` |
| `RequireOwnerOrPermission(key, perm)` | 资源所有者或持有权限均放行 |
| `BanCheck()` | 拦截 `ban` 角色 |

## Token 版本

`User.TokenVersion` 写入 JWT。修改密码或重置密码会递增该版本，使所有此前签发的 token 因版本不匹配而被拒绝（`TOKEN_REVOKED`）。

## 使用方式

路由在 `protected` 组中声明所需权限：

```go
r.POST("/delete", middleware.RequirePermission(permcode.PermUserDelete), h.Delete)
```

前端可参考 [API 参考](/architecture/api) 中各接口的「权限」列。
