# router 模块

路由注册层，按模块拆分路由定义。`protected` 组统一应用 `JWTAuth()` + `BanCheck()`，权限路由额外 `RequirePermission(permcode.X)`。

## 目录结构

```
router/
├── router.go          # 主入口：组合各模块路由 + Socket.IO 注册
└── routes/
    ├── auth/          # 认证（公开 + 受保护）
    ├── user/          # 用户（受保护）
    ├── signal/        # 信令（公开 + 受保护，含 Cloudflare tracks）
    ├── oauth/         # OAuth（公开 + 管理员）
    ├── role/          # 角色（受保护）
    ├── permission/    # 权限（受保护）
    ├── mute/          # 禁言（受保护）
    ├── room/          # 房间（受保护）
    ├── storage/       # 对象存储（受保护）
    ├── bot/           # Bot 令牌（受保护）
    ├── email/         # 邮箱验证（受保护）
    ├── email_config/  # 邮箱配置（受保护）
    ├── sfu_config/    # SFU 配置（受保护，sfu:manage）
    ├── srs/           # SRS 回调（公开）
    ├── system/        # 健康检查流（公开）
    └── swagger/       # Swagger UI（中文）
```

## 路由分组

| 前缀 | 中间件 | 用途 |
|------|--------|------|
| `/api/v1/auth` | 无 / JWT | 登录、注册、刷新、登出、改密 |
| `/api/v1/user` | JWT | 用户资料、列表、管理 |
| `/api/v1/signal` | 无 / JWT | 信令消息、Token、房间、参与者、Cloudflare |
| `/api/v1/oauth` | 无 / JWT | 第三方登录、提供商管理 |
| `/api/v1/role` | JWT + perm | 角色管理 |
| `/api/v1/permission` | JWT + perm | 权限管理 |
| `/api/v1/mute` | JWT + perm | 禁言管理 |
| `/api/v1/room` | JWT + perm | 房间管理 |
| `/api/v1/storage` | JWT + perm | 对象存储 |
| `/api/v1/bot` | JWT + perm | Bot 令牌 |
| `/api/v1/email` | JWT + perm | 邮箱验证/配置 |
| `/api/v1/sfu` | JWT + `sfu:manage` | SFU provider 配置 |
| `/api/v1/srs` | 无 | SRS HTTP Hooks 回调 |
| `/api/v1/system` | 无 | 健康检查流 |

> Swagger 使用 `locale: "zh-CN"` 显示中文界面
