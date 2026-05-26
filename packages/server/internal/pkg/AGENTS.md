# pkg 模块

公共工具包，提供跨模块复用的基础能力。

## 文件说明

| 文件 | 职责 |
|------|------|
| errors.go | 业务错误码定义（ErrCode）和 AppError 错误类型 |
| jwt.go | JWT Token 生成与解析；每个 Token 携带唯一 ULID 作为 JTI；签名密钥由 `redis.GetOrRotateSigningKey()` 提供（支持密钥轮换） |
| response.go | 统一响应封装：Success/Fail/HandleError |

## 子包

| 包 | 说明 |
|------|------|
| oauth/ | OAuth 第三方登录协议实现（GitHub/Google/QQ），提供 Provider 接口和默认端点配置 |

## 错误码范围

| 范围 | 类别 |
|------|------|
| 0 | 成功 |
| 1xxx | 认证相关（含 `TOKEN_REVOKED=1014` 黑名单注销） |
| 2xxx | 参数校验 |
| 3xxx | 资源相关 |
| 5xxx | 服务端内部错误 |
| 6xxx | LiveKit 相关 |
| 7xxx | OAuth 相关 |
