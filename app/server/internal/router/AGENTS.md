# router 模块

路由注册层，按模块拆分路由定义。

## 目录结构

```
router/
├── router.go          # 主入口：组合各模块路由
└── routes/
    ├── auth/routes.go     # 认证路由（公开 + 受保护）
    ├── user/routes.go     # 用户路由（需鉴权）
    ├── signal/routes.go   # 信令路由（公开）
    ├── swagger/routes.go  # Swagger UI 路由（中文）
    └── oauth/routes.go    # OAuth 路由（公开 + 受保护管理端）
```

## 路由分组

| 前缀 | 中间件 | 用途 |
|------|--------|------|
| `/api/v1/auth` | 无 | 登录、注册、刷新 Token |
| `/api/v1/signal` | 无 | 信令消息交换 |
| `/api/v1/oauth` | 无 | OAuth 第三方登录跳转与回调 |
| `/api/v1/auth` | JWT | 登出、刷新 Token |
| `/api/v1/user` | JWT | 用户资料、列表、管理 |
| `/api/v1/oauth/admin` | JWT | OAuth 提供商配置管理 CRUD |
| `/api/v1/sfu` | JWT + 权限 | SFU provider 配置管理 |

> Swagger 使用 `locale: "zh-CN"` 显示中文界面
