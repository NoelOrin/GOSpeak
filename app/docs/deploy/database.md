# 数据库

GOSpeak 支持三种数据库，从零配置到生产级都能覆盖。选哪个取决于你有多少人用。

## 怎么选

| 你的情况 | 用哪个 |
|---------|--------|
| 自己用、几个人、开发测试 | SQLite |
| 小团队、几十人 | SQLite（开 WAL）或 PostgreSQL |
| 上百人、要多实例 | PostgreSQL |
| 已有 MySQL 运维体系 | MySQL |

表结构三种数据库通用，切换时不用改代码，只改环境变量。

## SQLite

默认就是这个，零配置。

```ini
DB_TYPE=SQLite
DB_PATH=db/app.db
DB_WAL=true
```

数据库就是**一个文件**，备份就是拷文件。

### WAL 模式建议打开

```ini
DB_WAL=true
```

不打开的话，读写会互相阻塞，人多的时候容易看到 `database is locked`。

开了 WAL 之后，读和写可以并发，但写还是串行的——所以它依然不适合高并发写。

### 什么时候该换掉

出现这些信号说明 SQLite 撑不住了：

- 频繁 `database is locked`
- 写入明显变慢
- 想部署多个实例

## PostgreSQL

生产环境推荐。

```ini
DB_TYPE=PostgreSQL
DB_HOST=postgres
DB_PORT=5432
DB_USER=gospeak
DB_PASSWORD=强密码
```

用 Compose 的话，还要把 `postgres` 加进档位：

```ini
COMPOSE_PROFILES=app,livekit,postgres
POSTGRES_PASSWORD=和上面那个DB_PASSWORD一样
```

`DB_PASSWORD` 和 `POSTGRES_PASSWORD` 必须一致——前者给应用用，后者给容器初始化用。这是新手最容易踩的坑。

### 自己装 PostgreSQL

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
DB_PASSWORD=你刚设的密码
```

## MySQL

```ini
DB_TYPE=MySQL
DB_HOST=mysql
DB_PORT=3306
DB_USER=gospeak
DB_PASSWORD=强密码
```

需要先建好数据库，字符集用 `utf8mb4`（要存 emoji）：

```sql
CREATE DATABASE gospeak CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
```

## 从 SQLite 换到 PostgreSQL

**数据不会自动搬。** GORM 只负责建表，不负责迁移数据。

步骤：

```bash
# 1. 停服务
docker compose down

# 2. 导出 SQLite
sqlite3 db/app.db .dump > backup.sql

# 3. 改配置
#    DB_TYPE=PostgreSQL 等

# 4. 起服务（GORM 自动建表）
docker compose up -d

# 5. 手动把数据导进新库
```

第 5 步最麻烦——SQLite 的 dump 语法和 PostgreSQL 不兼容，要清理。常见做法是用工具转：

- `pgloader`：专门干这个的，支持 SQLite → PostgreSQL
- 或者写脚本读 SQLite 写 PostgreSQL

数据量不大的话，最简单的办法是：新库建好之后，重新注册账号、重新建域和频道，旧消息如果不要就丢了。

## 表结构会自动迁移

启动时 GORM 会跑 `AutoMigrate`：

- 表不存在 → 建
- 表存在但缺字段 → 加字段
- 字段类型变了 → 尽量改（有风险）

所以升级版本不用手动跑 SQL。

**但自动迁移不删字段。** 某个字段在新版本里删掉了，数据库里那列还在，不影响运行。

## 备份

### SQLite

```bash
# 直接拷文件（WAL 模式下要先 checkpoint）
sqlite3 db/app.db "PRAGMA wal_checkpoint(TRUNCATE);"
cp db/app.db backup-$(date +%F).db
```

Docker 里的话：

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
SQLite 并发写冲突。开 `DB_WAL=true`，还不行就换 PostgreSQL。

**连不上 PostgreSQL**
```bash
docker compose exec postgres pg_isready -U gospeak
```
不通的话看 postgres 容器日志，八成是密码不一致或者库没建。

**换库后登录不了**
用户数据没迁过去。数据库换了数据不会跟过来。

**迁移时报字段不存在**
旧版本的字段新版本删了。可以手动加列，或者直接忽略（不影响运行）。

## 下一步

- [部署到服务器](/deploy/) — 整体部署流程
- [配置项](/guide/configuration) — 全部数据库相关变量
