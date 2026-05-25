# middleware 模块

Gin 中间件，处理请求拦截和鉴权。

## 文件说明

| 文件 | 职责 |
|------|------|
| auth.go | JWT 鉴权中间件：解析 Authorization 头，验证 Token 有效性，将用户名注入上下文 |

## 使用方式

```go
protected := api.Group("")
protected.Use(middleware.JWTAuth())
```
