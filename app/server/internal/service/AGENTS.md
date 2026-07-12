# service 模块

业务逻辑层，协调 repository 和外部服务完成核心业务。

## 文件说明

| 文件 | 职责 |
|------|------|
| auth_service.go | 登录（bcrypt 比对）、注册（bcrypt 哈希）、Token 刷新、登出、改密、首次改密、重置密码、Token 版本管理 |
| user_service.go | 资料、列表、增删改、角色更新 |
| room_service.go | 房间 CRUD（密码校验、创建者记录）|
| oauth_service.go | 授权 URL、回调（新用户注册/已有登录）、提供商配置 CRUD、通用 OAuth 流程 |
| role_service.go | 角色管理 + 角色缓存 |
| permission_service.go | 权限列表、角色权限读写、同步 |
| mute_service.go | 禁言创建/取消/状态，发布 `user:muted`/`user:unmuted` 事件 |
| storage_service.go | 本地/S3 上传、预签名、配置加解密 |
| bot_service.go | Bot 令牌签发（权限白名单）、吊销 |
| email_service.go / email_verification_service.go | 验证码发送/校验、冷却、尝试限制 |
| email_config_service.go | 邮箱配置读取/更新 |
| sfu_config_service.go | SFU 配置持久化、provider 切换、激活 provider 解析 |
| sfu_service.go | SFU 动态 provider 解析与分发 |
| cloudflare_media_service.go | Cloudflare Realtime 媒体会话管理 |

## 业务语义

- 禁言是**用户级禁言**，不是房间级静音。用户级禁言：允许收听，但不能发布本地音轨。
- service 层只使用“禁言/仅收听”语义，不要表述为本地播放静音。

## 依赖关系

service 依赖 repository 进行数据操作，通过 pkg 包处理错误和生成 Token。
handler 层通过构造函数注入 service 实例。
