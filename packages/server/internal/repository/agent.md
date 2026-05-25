# repository 模块

数据访问层，封装数据库操作，提供 CRUD 接口。

## 文件说明

| 文件 | 职责 |
|------|------|
| db.go | 数据库初始化：支持 SQLite/PostgreSQL/MySQL，自动建表，连接健康检查，WAL 模式可选 |
| user_repo.go | 用户仓库：按 ID/UUID/Name 查询、列表分页、创建/更新/删除 |
| room_repo.go | 房间仓库：按 ID/UUID 查询、列表、创建/更新/删除 |
| oauth_provider_repo.go | OAuth 提供商配置仓库：按 name 查询、列表、创建/更新/删除 |
| oauth_account_repo.go | 第三方账号关联仓库：按 provider+uid 查询、按 user_id 查询、创建/更新 |

## 注意事项

- GetBy* 方法在查询失败时返回 nil 指针 + error，不要忽略 error 检查
- SQLite 默认路径为 `db/app.db`（相对于服务器工作目录）
- SQLite WAL 模式通过环境变量 `DB_WAL=true` 开启（默认关闭 DELETE 模式）
