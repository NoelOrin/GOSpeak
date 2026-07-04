# middleware 模块

Gin 中间件，处理请求拦截和鉴权。

## 文件说明

| 文件 | 职责 |
|------|------|
| auth.go | JWT 鉴权中间件：解析 Authorization 头，验证 Token 有效性，检查黑名单（`redis.IsBlacklisted`），将用户名注入上下文 |

## 使用方式

```go
protected := api.Group("")
protected.Use(middleware.JWTAuth())
```

## 鉴权检查顺序

1. Header 是否存在 → `TOKEN_NOT_EXIST`
2. 签名验证 → `TOKEN_WRONG`
3. 是否过期 → `TOKEN_EXPIRED`
4. JTI 是否在黑名单（已登出）→ `TOKEN_REVOKED`
