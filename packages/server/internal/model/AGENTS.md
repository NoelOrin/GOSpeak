# model 模块

数据模型定义层，定义数据库表结构和 JSON 序列化规则。

## 文件说明

| 文件 | 职责 |
|------|------|
| user.go | 用户模型：ID、UUID、Name、Password、时间戳，创建前自动生成 UUIDv4 |
| room.go | 房间模型：ID、UUID、Name、Limit、时间戳，创建前自动生成 UUIDv4 |
| user_group.go | 用户分组模型：ID、UserID、GroupName、时间戳 |
| oauth_provider.go | OAuth 提供商配置模型：name、client_id/secret、各端点 URL、scopes、enabled |
| oauth_account.go | 第三方账号关联模型：user_id、provider、provider_uid、access_token |

## 共同特征

- 使用 GORM ORM 映射
- UUID 字段使用 UUIDv4 生成
- 密码字段 `json:"-"` 不序列化到响应
