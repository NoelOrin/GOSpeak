# redis 模块

可选 Redis 客户端，未配置时优雅跳过，不影响主流程。

## 文件说明

| 文件 | 职责 |
|------|------|
| redis.go | Redis 客户端初始化（`InitRedis()`）；`REDIS_HOST` 为空时跳过连接；`IsConnected()` 检查连接状态 |
| jwt_key.go | JWT 签名密钥管理：`GetOrRotateSigningKey()` 从 Redis 读取当前密钥，TTL 到期后自动生成新随机密钥（密钥轮换），Redis 未连接时退化为静态 `JWT_KEY` 环境变量 |
| blacklist.go | Token 黑名单：`BlacklistToken(jti, remaining)` 将 JTI 写入 Redis 并设置 TTL；`IsBlacklisted(jti)` 检查是否已注销；Redis 未连接时为 no-op |

## 环境变量

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `REDIS_HOST` | `""` | 为空则不接入 Redis |
| `REDIS_PORT` | `6379` | Redis 端口 |
| `REDIS_PASSWORD` | `""` | 认证密码 |
| `REDIS_DB` | `0` | 数据库编号 |
| `JWT_KEY_TTL` | `24h` | 签名密钥轮换周期（Go duration 格式，如 `24h`、`168h`）；仅 Redis 连接时生效 |

## 设计原则

- Redis 未连接时所有功能退化为安全默认值，**不 panic、不报错**
- `BlacklistToken` / `IsBlacklisted` 以 `jwt:blacklist:<jti>` 为 key，TTL = 令牌剩余有效期
- `GetOrRotateSigningKey` 以 `jwt:signing_key` 为 key，TTL = `JWT_KEY_TTL`；TTL 到期即视为密钥轮换，所有旧 Token 自动失效

## 初始化位置

在 `server/gin.go` 的 `StartGin()` 中，数据库初始化之后调用 `redis.InitRedis()`。
