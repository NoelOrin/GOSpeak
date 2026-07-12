# 数据模型

## User（用户）

| 字段 | 类型 | 说明 |
|------|------|------|
| ID | uint | 主键 |
| UUID | string | 自动生成的唯一标识 |
| Name | string | 用户名 |
| Password | string | 密码（JSON 序列化时隐藏）|
| Role | string | 角色：`user` / `admin` |
| CreatedAt | time.Time | 创建时间 |
| UpdatedAt | time.Time | 更新时间 |

## Room（房间）

| 字段 | 类型 | 说明 |
|------|------|------|
| ID | uint | 主键 |
| UUID | string | 唯一标识 |
| Name | string | 房间名 |
| Limit | int | 人数上限 |
| CreatedAt | time.Time | 创建时间 |
| UpdatedAt | time.Time | 更新时间 |

## UserGroup（用户组）

| 字段 | 类型 | 说明 |
|------|------|------|
| ID | uint | 主键 |
| UserID | uint | 用户 ID |
| GroupName | string | 组名 |
| CreatedAt | time.Time | 创建时间 |
| UpdatedAt | time.Time | 更新时间 |

## OAuthProvider（OAuth 提供商配置）

| 字段 | 类型 | 说明 |
|------|------|------|
| ID | uint | 主键 |
| Name | string | 提供商名（`github` / `google` / `qq`）|
| ClientID | string | OAuth Client ID |
| ClientSecret | string | OAuth Client Secret |
| AuthURL | string | 授权端点 |
| TokenURL | string | Token 交换端点 |
| UserInfoURL | string | 用户信息端点 |
| RedirectURL | string | 回调地址 |
| Scopes | string | 权限范围 |
| Enabled | bool | 是否启用 |

## OAuthAccount（OAuth 账号绑定）

| 字段 | 类型 | 说明 |
|------|------|------|
| ID | uint | 主键 |
| UserID | uint | 绑定的本地用户 |
| Provider | string | 提供商 |
| ProviderUID | string | 第三方平台用户 ID |
| AccessToken | string | OAuth Access Token |
| RefreshToken | string | OAuth Refresh Token |

## 内存模型（无持久化）

### Signal 房间（信令层）

由 `signal.Hub` 管理，只存在于内存中：

```go
type RoomInfo struct {
    Room      string       `json:"room"`
    Members   []MemberInfo `json:"members"`
    CreatedAt time.Time    `json:"created_at"`
}

type MemberInfo struct {
    Identity string `json:"identity"`
    UserID   string `json:"user_id"`
    Username string `json:"username"`
    JoinAt   time.Time `json:"join_at"`
}
```

## 自动迁移

GORM AutoMigrate 在服务启动时自动同步表结构：

```go
// repository/db.go
db.AutoMigrate(
    &model.User{},
    &model.Room{},
    &model.UserGroup{},
    &model.OAuthProvider{},
    &model.OAuthAccount{},
)
```

新增字段或模型后重启服务即可，无需手动执行 DDL。<
