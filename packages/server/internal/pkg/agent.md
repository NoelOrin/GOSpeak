# pkg 模块

公共工具包，提供跨模块复用的基础能力。

## 文件说明

| 文件 | 职责 |
|------|------|
| errors.go | 业务错误码定义（ErrCode）和 AppError 错误类型 |
| jwt.go | JWT Token 生成与解析，支持 access_token 和 refresh_token |
| response.go | 统一响应封装：Success/Fail/HandleError |

## 错误码范围

| 范围 | 类别 |
|------|------|
| 0 | 成功 |
| 1xxx | 认证相关 |
| 2xxx | 参数校验 |
| 3xxx | 资源相关 |
| 5xxx | 服务端内部 |
| 6xxx | LiveKit 相关 |
