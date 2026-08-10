# 数据库演进

GOSpeak 支持**渐进式数据库**方案：从零配置的 SQLite 到生产级的 PostgreSQL / MySQL。

## 三档演进

```
A 档: SQLite ────────────── 开箱即用、零外部服务
    │
B 档: PostgreSQL ────────── 更高并发、支持多写
    │
C 档: PostgreSQL + NATS KV ── Token 黑名单 + 密钥轮换 + 跨实例状态
```

## A 档 — SQLite

**适用**：个人/小团队、开发环境、50 人以下并发。

```env
DB_TYPE="SQLite"
DB_PATH="/app/db/app.db"    # 默认路径
```

### 特点

- ✅ 零外部依赖，无需安装任何数据库服务
- ✅ 自动创建数据库文件
- ✅ GORM 自动迁移，表结构自动同步
- ❌ 并发写性能有限
- ❌ 不适合分布式部署

## B 档 — PostgreSQL

**适用**：生产环境、200+ 用户。

### Docker Compose 配置

```env
DB_TYPE="PostgresSQL"
DB_HOST="postgres"
DB_PORT="5432"
DB_USER="gospeak"
DB_PASSWORD="gospeak"        # ⚠️ 生产环境改强密码
```

```bash
docker compose --profile postgres --profile srs --profile app up -d
```

### 自建 PostgreSQL

```bash
# Ubuntu
apt install postgresql
sudo -u postgres createuser gospeak -P
sudo -u postgres createdb gospeak -O gospeak

# 环境变量
DB_TYPE="PostgresSQL"
DB_HOST="localhost"
DB_PORT="5432"
DB_USER="gospeak"
DB_PASSWORD="your-password"
```

### 从 SQLite 迁移

GORM 不提供跨数据库迁移工具，建议手动迁移：

1. 导出 SQLite 数据为 SQL
2. 在 PostgreSQL 中创建表（GORM 自动建表）
3. 导入数据

```bash
# 导出 SQLite
sqlite3 db/app.db .dump > backup.sql

# 导入 PostgreSQL（需要清理 SQLite 特有的语法）
psql -U gospeak -d gospeak < cleaned_backup.sql
```

## C 档 — PostgreSQL + 多实例状态共享（NATS KV）

**适用**：需要 Token 黑名单、JWT 密钥轮换和跨实例房间状态的生产环境。

```env
DB_TYPE="PostgresSQL"
DB_HOST="postgres"
DB_PORT="5432"
DB_USER="gospeak"
DB_PASSWORD="gospeak"

STATE_STORE="nats"
JWT_KEY_TTL="24h"            # 密钥轮换周期
```

### NATS KV 带来的功能

| 功能 | 说明 |
|------|------|
| Token 黑名单 | 登出后将 JWT 加入黑名单（TTL=剩余有效期）|
| JWT 密钥轮换 | 周期性更换签名密钥，旧 Token 自动失效 |
| 跨实例房间状态 | 成员/stream 映射经 JetStream KV 多实例共享 |
| Graceful Degradation | 未配置 NATS 时降级为进程内状态 + 静态密钥 |

> 未配置外部 NATS 时，内嵌 NATS 自动启动；多副本部署请使用同一个外部 `NATS_URL`。

## 数据库类型对比

| 特性 | SQLite | PostgreSQL |
|------|--------|------------|
| 部署复杂度 | 零配置 | 需安装服务 |
| 并发读 | ✅ | ✅ |
| 并发写 | ❌ 锁冲突 | ✅ MVCC |
| 备份 | 文件拷贝 | pg_dump |
| 存储限制 | 文件大小 | 磁盘空间 |
| 自动迁移 | ✅ GORM | ✅ GORM |
| 推荐场景 | 开发/个人 | 生产/团队 |
