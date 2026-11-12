# 数据库

GOSpeak 支持三种数据库，覆盖从零配置到生产级的各种场景，具体选择取决于使用规模。

## 选型

| 使用场景 | 数据库 |
|---------|--------|
| 个人使用、几个人规模、开发测试 | SQLite |
| 小团队、几十人规模 | SQLite（开 WAL）或 PostgreSQL |
| 上百人规模、需要多实例 | PostgreSQL |
| 已有 MySQL 运维体系 | MySQL |

表结构在三种数据库之间通用，切换时不需要改代码，只需要修改环境变量。

## SQLite

默认数据库，零配置。

```ini
DB_TYPE=SQLite
DB_PATH=db/app.db
DB_WAL=true
```

数据库对应**一个文件**，备份操作即为复制该文件。

### WAL 模式建议打开

```ini
DB_WAL=true
```

不开 WAL 时，读写操作会互相阻塞，用户量较大时容易出现 `database is locked`。

开启 WAL 之后，读和写可以并发，但写仍然串行——因此该模式依然不适合高并发写场景。

### 更换数据库的时机

出现这些信号说明 SQLite 已经不适合当前负载：

- 频繁出现 `database is locked`
- 写入明显变慢
- 需要部署多个实例

## PostgreSQL

生产环境的推荐选择。

```ini
DB_TYPE=PostgreSQL
DB_HOST=postgres
DB_PORT=5432
DB_USER=gospeak
DB_PASSWORD=强密码
```

使用 Compose 时，还需要把 `postgres` 加入档位：

```ini
COMPOSE_PROFILES=app,livekit,postgres
POSTGRES_PASSWORD=<与 DB_PASSWORD 相同>
```

`DB_PASSWORD` 和 `POSTGRES_PASSWORD` 必须一致——前者供应用使用，后者供容器初始化使用。这是最常见的配置失误点。

### 自行安装 PostgreSQL

```bash
# Ubuntu
apt install postgresql
sudo -u postgres createuser gospeak -P
sudo -u postgres createdb gospeak -O gospeak
```

```ini
DB_TYPE=PostgreSQL
DB_HOST=localhost
DB_PORT=5432
DB_USER=gospeak
DB_PASSWORD=<设置的密码>
```

## MySQL

```ini
DB_TYPE=MySQL
DB_HOST=mysql
DB_PORT=3306
DB_USER=gospeak
DB_PASSWORD=强密码
```

需要预先创建数据库，字符集使用 `utf8mb4`（用于存储 emoji）：

```sql
CREATE DATABASE gospeak CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
```

## 从 SQLite 换到 PostgreSQL

**数据不会自动迁移。** GORM 只负责建表，不负责迁移数据。

步骤：

```bash
# 1. 停止服务
docker compose down

# 2. 导出 SQLite
sqlite3 db/app.db .dump > backup.sql

# 3. 修改配置
#    DB_TYPE=PostgreSQL 等

# 4. 启动服务（GORM 自动建表）
docker compose up -d

# 5. 手动将数据导入新库
```

第 5 步最为繁琐——SQLite 的 dump 语法和 PostgreSQL 不兼容，需要清理。常见做法是使用工具转换：

- `pgloader`：专门处理这类迁移的工具，支持 SQLite → PostgreSQL
- 或者编写脚本读取 SQLite 并写入 PostgreSQL

数据量不大时，最简单的办法是：新库建好之后，重新注册账号、重新建域和频道，不需要的旧消息直接丢弃。

## 表结构自动迁移

启动时 GORM 会执行 `AutoMigrate`：

- 表不存在 → 创建
- 表存在但缺字段 → 添加字段
- 字段类型变了 → 尽量修改（有风险）

因此升级版本不需要手动执行 SQL。

**但自动迁移不删字段。** 某个字段在新版本中被删除后，数据库里对应的列仍然保留，不影响运行。

## 备份

### SQLite

```bash
# 直接复制文件（WAL 模式下需先 checkpoint）
sqlite3 db/app.db "PRAGMA wal_checkpoint(TRUNCATE);"
cp db/app.db backup-$(date +%F).db
```

Docker 环境下：

```bash
docker run --rm -v gospeak-db:/data -v $(pwd):/backup alpine \
  tar czf /backup/db-$(date +%F).tar.gz -C /data .
```

### PostgreSQL

```bash
docker compose exec -T postgres pg_dump -U gospeak gospeak | gzip > backup-$(date +%F).sql.gz
```

恢复：

```bash
gunzip -c backup-2026-11-08.sql.gz | docker compose exec -T postgres psql -U gospeak gospeak
```

## 常见问题

**`database is locked`**
SQLite 并发写冲突。开启 `DB_WAL=true`，问题仍然存在则更换 PostgreSQL。

**无法连接 PostgreSQL**
```bash
docker compose exec postgres pg_isready -U gospeak
```
不通时查看 postgres 容器日志，原因多为密码不一致或数据库未创建。

**换库后无法登录**
用户数据未迁移。数据库更换后数据不会随之转移。

**迁移时报字段不存在**
旧版本的字段在新版本中被删除。可以手动加列，或者直接忽略（不影响运行）。

## 下一步

- [部署到服务器](/deploy/) — 整体部署流程
- [配置项](/guide/configuration) — 全部数据库相关变量
