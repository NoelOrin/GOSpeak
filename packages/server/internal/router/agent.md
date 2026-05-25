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
    └── swagger/routes.go  # Swagger UI 路由（中文）
```

## 设计原则

- 公开路由：注册、登录、信令、Swagger
- 受保护路由：用户管理、Token 刷新/登出（需 JWTAuth 中间件）
- Swagger 使用 `locale: "zh-CN"` 显示中文界面
