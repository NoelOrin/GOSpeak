# middleware 模块

Gin 中间件，处理请求拦截和鉴权。所有中间件位于 `internal/middleware/auth.go`。

## 文件说明

| 函数 | 职责 |
|------|------|
| `JWTAuth()` | 解析 Authorization 头，校验 Token（签名、过期、TokenVersion、黑名单 JTI），注入 `username` / `display_name` / `user_uuid` / `role` / `claims` / `auth_type` 到 context |
| `RequireRole(roles ...string)` | 旧式角色校验（基于 `role` claim），不匹配返回 `FORBIDDEN` (1013) |
| `RequirePermission(permCode)` | 基于权限码的鉴权：从 context 取 `role`（缺失返回 `TOKEN_NOT_EXIST` 1001），经 `permissionChecker.HasPermission(role, permCode)` 判定，命中放行，否则返回 `FORBIDDEN` (1013) |
| `RequireOwnerOrPermission(ownerContextKey, permCode)` | 资源所有者（owner 字段等于 `username`）或持有权限均放行 |
| `BanCheck()` | 拦截 `ban` 角色用户，返回 `FORBIDDEN` (1013) |
| `CORS()` | 跨域与 OPTIONS 预检 |

## 鉴权检查顺序（JWTAuth）

1. Header 存在 → `TOKEN_NOT_EXIST` (1001)
2. 签名有效 → `TOKEN_WRONG` (1002)
3. 未过期 → `TOKEN_EXPIRED` (1003)
4. TokenVersion 与当前用户一致 → 否则 `TOKEN_REVOKED` (1014)
5. JTI 未登出黑名单 → 否则 `TOKEN_REVOKED` (1014)
6. 注入 claims 到 context

## RBAC

- 权限码集中在 `internal/permcode/permcode.go`（如 `user:read`、`room:create`、`sfu:manage`、`mute:manage`）。
- 用户权限 = 其 `role` 经 `role_permissions` 解析。
- 启动时为 `permissionChecker` / `tokenVersionChecker` 注入实现（`SetPermissionChecker` / `SetTokenVersionChecker`）。
