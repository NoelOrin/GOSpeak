# model 模块

数据模型定义层，定义数据库表结构与 JSON 序列化规则。所有模型在 `repository/db.go` 的 `AutoMigrate` 中注册，服务启动时自动建表。

## 文件说明

| 文件 | 职责 |
|------|------|
| user.go | 用户模型：UUIDv4、Name、DisplayName、Avatar、Email、EmailVerified、IsBot、Password(`json:"-"`)... 
| room.go | 房间模型：UUIDv4、Name、Password、Description、Limit、AudioOnly、AllowAudience、CreatedBy |
| user_group.go | 用户分组模型：UserID、GroupName |
| role.go | 角色模型（种子：admin/user/ban）+ 角色缓存与校验 |
| permission.go | Permission（权限定义）+ RolePermission（角色-权限关联）|
| mute.go | 禁言记录（用户级，全局生效，支持永久/限时）|
| oauth_provider.go | OAuth 提供商配置（端点、密钥、字段映射、Enabled）|
| oauth_account.go | 第三方账号绑定（user_id、provider、provider_uid）|
| bot_token.go | Bot 令牌（权限白名单、过期、吊销）|
| email_config.go | 邮箱 SMTP 配置 |
| email_verification_code.go | 邮箱验证码（场景、哈希、冷却、尝试次数）|
| storage_config.go | 对象存储配置（local/s3）|
| sfu_config.go | 每个 SFU provider 一行配置 + 当前激活 provider |

## 共同特征

- 使用 GORM ORM 映射
- UUID 字段使用 UUIDv4 生成（User / Room / Mute / BotToken）
- 敏感字段（Password、AccessToken、RefreshToken、SecretKey、EmailCodeSecret、CodeHash）使用 `json:"-"` 不序列化
- 密码/密钥写入加密存储、读取解密（StorageConfig、EmailConfig）
