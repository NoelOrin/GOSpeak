# repository 模块

数据访问层，封装数据库操作，提供 CRUD 接口。

## 文件说明

| 文件 | 职责 |
|------|------|
| db.go | 数据库初始化：SQLite/PostgreSQL/MySQL，AutoMigrate 注册全部模型，健康检查，WAL 模式可选 |
| user_repo.go | 用户：按 ID/UUID/Name 查询、列表分页、创建/更新/删除、TokenVersion 更新 |
| room_repo.go | 房间：按 ID/UUID 查询、列表、创建/更新/删除 |
| oauth_provider_repo.go | OAuth 提供商：按 name 查询、列表、创建/更新/删除 |
| oauth_account_repo.go | 账号绑定：按 provider+uid、按 user_id 查询、创建/更新 |
| role_repo.go | 角色：CRUD + 缓存加载 |
| permission_repo.go | 权限与角色权限关联：CRUD、按角色查权限 |
| mute_repo.go | 禁言：CRUD、按用户查有效禁言 |
| bot_token_repo.go | Bot 令牌：CRUD、按 UUID 查询、吊销 |
| email_config_repo.go | 邮箱配置：读取/更新（密钥加解密）|
| email_verification_code_repo.go | 验证码：CRUD、场景查询、尝试计数 |
| storage_config_repo.go | 存储配置：读取/更新 |
| sfu_config_repo.go | SFU 配置：按 provider 读写、激活 provider 读写 |

## 注意事项

- GetBy* 失败返回 nil 指针 + error，不要忽略 error 检查
- SQLite 默认路径 `db/app.db`（相对于服务器工作目录）
- SQLite WAL 模式通过环境变量 `DB_WAL=true` 开启（开发环境建议开启；默认 DELETE）
- glebarez DSN 必须用 `_pragma=busy_timeout(...)/journal_mode(...)`，旧 `_busy_timeout` 无效
- SQLite 连接池固定 `MaxOpenConns(1)`，避免同进程多连接写锁竞争
