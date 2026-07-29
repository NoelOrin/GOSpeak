# 数据模型

所有模型在 `repository/db.go` 的 `AutoMigrate` 中注册，服务启动时自动建表；新增字段/模型重启即生效，无需手动 DDL。

## User（用户）— `users`

| 字段 | 类型 | 说明 |
|------|------|------|
| ID | uint | 主键 |
| UUID | string | 自动生成（UUIDv4）|
| Name | string | 用户名（唯一）|
| DisplayName | string | 显示名 |
| Avatar | string | 头像 URL |
| Email | string | 邮箱（索引）|
| EmailVerified | bool | 邮箱是否已验证 |
| IsBot | bool | 是否为 Bot 账号 |
| Password | string | 密码（`json:"-"` 不返回）|
| Role | string | 角色：`admin` / `user` / `ban` |
| TokenVersion | uint | Token 版本，改密/重置后递增使旧 token 失效 |
| CreatedAt / UpdatedAt | time.Time | 时间戳 |

## Room（房间）— `room`

| 字段 | 类型 | 说明 |
|------|------|------|
| ID | uint | 主键 |
| UUID | string | 唯一标识 |
| Name | string | 房间名 |
| Password | string | 房间密码 |
| Description | string | 描述 |
| Limit | uint | 人数上限 |
| Type | string | 房间类型：`voice`（语音，默认）/ `text`（文字聊天）|
| AudioOnly | bool | 仅音频 |
| AllowAudience | bool | 允许观众席 |
| CreatedBy | string | 创建者标识 |
| CreatedAt / UpdatedAt | time.Time | 时间戳 |

## Message（消息）— `messages`

| 字段 | 类型 | 说明 |
|------|------|------|
| ID | uint | 主键 |
| UUID | string | 唯一标识（UUIDv4）|
| RoomUUID | string | 所属房间 UUID |
| AuthorID | string | 发送者用户 UUID |
| Content | string | 消息内容 |
| ReplyTo | string | 回复的目标消息 UUID |
| EditedAt | *time.Time | 编辑时间（nil = 未编辑）|
| DeletedAt | gorm.DeletedAt | 软删除 |
| CreatedAt | time.Time | 创建时间 |
| UpdatedAt | time.Time | 更新时间 |

## MessageReaction（消息反应）— `message_reactions`

| 字段 | 类型 | 说明 |
|------|------|------|
| ID | uint | 主键 |
| MessageUUID | string | 消息 UUID |
| UserID | string | 用户 UUID |
| Emoji | string | 表情符号 |
| CreatedAt | time.Time | 创建时间 |

唯一约束：`(message_uuid, user_id, emoji)` 联合唯一。

## UserGroup（用户组）— `user_groups`

| 字段 | 类型 | 说明 |
|------|------|------|
| ID | uint | 主键 |
| UserID | uint | 用户 ID |
| GroupName | string | 组名 |
| CreatedAt / UpdatedAt | time.Time | 时间戳 |

## Role（角色）— `roles`

| 字段 | 类型 | 说明 |
|------|------|------|
| ID | uint | 主键 |
| Name | string | 角色名（种子：`admin` / `user` / `ban`）|
| CreatedAt / UpdatedAt | time.Time | 时间戳 |

## Permission（权限）— `permissions`

| 字段 | 类型 | 说明 |
|------|------|------|
| ID | uint | 主键 |
| Code | string | 权限码（唯一，对应 `permcode` 常量）|
| Name | string | 权限名 |
| Description | string | 描述 |
| CreatedAt | time.Time | 时间戳 |

## RolePermission（角色-权限关联）— `role_permissions`

| 字段 | 类型 | 说明 |
|------|------|------|
| ID | uint | 主键 |
| RoleName | string | 角色名（索引）|
| PermissionID | uint | 权限 ID（索引）|

## Mute（禁言）— `mutes`

| 字段 | 类型 | 说明 |
|------|------|------|
| ID | uint | 主键 |
| UUID | string | 唯一标识 |
| UserID | uint | 被禁言用户 |
| MuterID | uint | 操作者 |
| Duration | int64 | 禁言秒数，0 = 永久 |
| Permanent | bool | 是否永久 |
| ExpiresAt | *time.Time | 过期时间（nil = 永久）|
| Reason | string | 原因 |
| CreatedAt / UpdatedAt | time.Time | 时间戳 |

## OAuthProvider（OAuth 提供商配置）— `oauth_providers`

| 字段 | 类型 | 说明 |
|------|------|------|
| ID | uint | 主键 |
| Name | string | 提供商名（唯一，如 `github`）|
| DisplayName | string | 显示名 |
| IconURL | string | 图标 |
| ClientID / ClientSecret | string | OAuth 凭据 |
| AuthURL / TokenURL / UserInfoURL | string | 端点 |
| RedirectURL | string | 回调地址 |
| Scopes | string | 权限范围 |
| UIDField / UsernameField / AvatarField / EmailField | string | 用户信息 JSON 字段映射（支持自定义 provider）|
| Enabled | bool | 是否启用 |
| CreatedAt / UpdatedAt | time.Time | 时间戳 |

## OAuthAccount（OAuth 账号绑定）— `oauth_accounts`

| 字段 | 类型 | 说明 |
|------|------|------|
| ID | uint | 主键 |
| UserID | uint | 本地用户 |
| Provider | string | 提供商 |
| ProviderUID | string | 第三方平台用户 ID |
| AccessToken / RefreshToken | string | 凭据（`json:"-"` 不返回）|
| CreatedAt / UpdatedAt | time.Time | 时间戳 |

## BotToken（Bot 令牌）— `bot_tokens`

| 字段 | 类型 | 说明 |
|------|------|------|
| ID | uint | 主键 |
| UUID | string | 唯一标识 |
| Name | string | 令牌名（唯一）|
| UserUUID | string | 所有者 |
| Permissions | []string | 权限白名单（JWT 中携带）|
| Revoked | bool | 是否已吊销 |
| ExpiresAt | time.Time | 过期时间 |
| CreatedAt / UpdatedAt | time.Time | 时间戳 |

## EmailConfig（邮箱配置）— `email_configs`

| 字段 | 类型 | 说明 |
|------|------|------|
| ID | uint | 主键 |
| Enabled | bool | 是否启用 |
| SMTPHost / SMTPPort / SMTPUsername / SMTPPassword | string | SMTP 配置（Password `json:"-"`）|
| SMTPFrom / SMTPFromName | string | 发件人 |
| EmailCodeTTL / EmailSendCooldown | string | 验证码 TTL / 冷却 |
| EmailCodeSecret | string | 签名密钥（`json:"-"`）|
| CreatedAt / UpdatedAt | time.Time | 时间戳 |

## EmailVerificationCode（邮箱验证码）— `email_verification_codes`

| 字段 | 类型 | 说明 |
|------|------|------|
| ID | uint | 主键 |
| Email / Scene | string | 邮箱 / 场景（索引）|
| CodeHash | string | 验证码哈希（`json:"-"`）|
| UserID | *uint | 关联用户 |
| IPAddress | string | 请求 IP |
| ExpiresAt / UsedAt | time.Time | 过期 / 使用时间 |
| AttemptCount | int | 尝试次数 |
| CreatedAt / UpdatedAt | time.Time | 时间戳 |

## StorageConfig（对象存储配置）— `storage_configs`

| 字段 | 类型 | 说明 |
|------|------|------|
| ID | uint | 主键 |
| ProviderType | string | `local` / `s3` |
| Endpoint / Bucket / Region | string | S3 配置 |
| AccessKey / SecretKey | string | 凭据（`json:"-"`）|
| PublicBaseURL | string | 公开访问基础 URL |
| PathPrefix | string | 上传路径前缀 |
| MaxFileSize | int | 单文件大小上限（MB）|
| AllowedTypes | string | 允许的文件 MIME |
| CreatedAt / UpdatedAt | time.Time | 时间戳 |

## SFUConfig（SFU 配置）— `sfu_configs`

每个 provider 一行，以 `Provider` 为主键；切换 provider 互不覆盖配置。字段覆盖 LiveKit / Agora / MediaSoup / SRS / Daily / Cloudflare 的全部连接参数（host、key、secret、证书、端口、WHIP URL、STUN 等）。

## SFUActiveProvider（当前激活 provider）— `sfu_active_provider`

| 字段 | 类型 | 说明 |
|------|------|------|
| ID | uint | 主键 |
| Provider | string | 当前激活的 SFU provider |

## 内存模型（无持久化）

### Signal 房间（信令层）

由 `signal.Hub` 管理，只存在于内存中：

```go
type RoomInfo struct {
    ID            uint         `json:"id"`
    UUID          string       `json:"uuid"`
    Name          string       `json:"name"`
    HasPassword   bool         `json:"hasPassword"`
    Description   string       `json:"description"`
    Limit         uint         `json:"limit"`
    Type          string       `json:"type"`
    AudioOnly     bool         `json:"audioOnly"`
    AllowAudience bool         `json:"allowAudience"`
    Members       []MemberInfo `json:"members"`
    Count         int          `json:"count"`
    CreatedAt     int64        `json:"createdAt"`
}

type MemberInfo struct {
    ID          string `json:"id"`
    Identity    string `json:"identity"`
    Name        string `json:"name"`
    DisplayName string `json:"displayName"`
    Avatar      string `json:"avatar"`
    IsMuted     bool   `json:"isMuted"`
    IsMicMuted  bool   `json:"isMicMuted"`
    JoinedAt    int64  `json:"joinedAt"`
    Stream      string `json:"stream,omitempty"`
}
```
