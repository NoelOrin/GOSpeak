# service 模块

业务逻辑层，协调 repository 和外部服务完成核心业务。

## 文件说明

| 文件 | 职责 |
|------|------|
| auth_service.go | 认证服务：登录验证（bcrypt 比对）、注册（bcrypt 哈希存储）、Token 刷新（支持 JWT 携带 UUID） |
| user_service.go | 用户服务：获取资料（支持 UUID 查询）、列表、删除 |
| room_service.go | 房间服务：房间 CRUD 操作 |
| oauth_service.go | OAuth 第三方登录服务：获取授权 URL、回调处理（新用户注册 / 已有用户登录）、提供商配置管理 CRUD |

## 依赖关系

service 依赖 repository 进行数据操作，通过 pkg 包处理错误和生成 Token。
handler 层通过构造函数注入 service 实例。
