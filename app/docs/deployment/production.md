# 生产部署指南

## 发布前 Checklist

- [ ] **修改默认密钥**：`JWT_KEY`、`SRS_SECRET`、DB 密码、RustFS 密码
- [ ] **设置公网地址**：`SRS_CANDIDATE` 设为公网 IP（SRS 模式）
- [ ] **配置 HTTPS**：Nginx 配置 TLS 证书
- [ ] **选择数据库**：生产环境推荐 PostgreSQL
- [ ] **配置状态共享**：多实例设 `STATE_STORE=nats` + 外部 `NATS_URL`
- [ ] **防火墙放行**：开放必要端口
- [ ] **配置备份**：数据库定时备份

## 端口清单

| 端口 | 服务 | 公网 | 说明 |
|------|------|------|------|
| 80/443 | Nginx | ✅ 必需 | 公网入口 |
| 8998 | GOSpeak | 仅调试 | API + SPA + WebSocket |
| 1985 | SRS API | ❌ 不暴露 | 管理接口，经 Nginx 暴露 `/rtc/v1` |
| 8000/udp+tcp | SRS 媒体 | ✅ 必需 | WebRTC 直连 |
| 7880-7882 | LiveKit | ✅ 视情况 | 控制/媒体 |
| 3478/udp | LiveKit TURN | ✅ 视情况 | TURN 中继 |
| 5432 | PostgreSQL | ❌ 不暴露 | 数据库 |
| 9000/9001 | RustFS | ❌ 按需 | 对象存储 |

## HTTPS 配置

GOSpeak 需要 HTTPS 上下文才能使用浏览器麦克风（WebRTC 安全要求）。

### 方案 A：Nginx 终结 TLS

1. 获取证书（Let's Encrypt / 云厂商证书）
2. 在 `deploy/nginx-docker.conf` 基础上添加 SSL 配置：

```nginx
server {
    listen 443 ssl;
    server_name gospeak.example.com;

    ssl_certificate     /etc/nginx/ssl/cert.pem;
    ssl_certificate_key /etc/nginx/ssl/key.pem;

    # ... 其余与 nginx-docker.conf 相同
}

server {
    listen 80;
    return 301 https://$host$request_uri;
}
```

3. 更新 `app.srs.env`：

```env
SRS_PUBLIC_HOST=https://gospeak.example.com
```

### 方案 B：Caddy 自动 TLS（推荐）

Caddy 自动申请和续签 Let's Encrypt 证书。使用 `Caddyfile`：

```caddyfile
gospeak.example.com {
    reverse_proxy gospeak:8998

    @srs_api {
        path /rtc/v1/*
    }
    reverse_proxy @srs_api srs:1985
}
```

## 防火墙配置

以 Ubuntu + UFW 为例：

```bash
# SSH
ufw allow ssh

# Nginx 公网入口
ufw allow 80/tcp
ufw allow 443/tcp

# SRS 媒体端口
ufw allow 8000/udp
ufw allow 8000/tcp

# 如果使用 LiveKit
ufw allow 7881/udp
ufw allow 7882/udp
ufw allow 7882/tcp
ufw allow 3478/udp
```

## 系统优化

### 系统参数

```bash
# sysctl 配置 (/etc/sysctl.d/99-gospeak.conf)

# 网络优化
net.core.rmem_max = 134217728
net.core.wmem_max = 134217728
net.ipv4.udp_mem = 65536 131072 262144

# 连接跟踪（SRS 大量 UDP 连接）
net.netfilter.nf_conntrack_max = 1048576
net.netfilter.nf_conntrack_udp_timeout = 30

# 文件描述符
fs.file-max = 1048576
```

### Docker 资源限制

```yaml
services:
  gospeak:
    deploy:
      resources:
        limits:
          memory: 512M
        reservations:
          memory: 256M
  srs:
    deploy:
      resources:
        limits:
          memory: 256M
```

## 备份策略

### SQLite

```bash
# 定时任务：每天凌晨备份
0 3 * * * cp /app/db/app.db /backup/gospeak-$(date +\%Y\%m\%d).db
```

### PostgreSQL

```bash
# 定时 pg_dump
0 3 * * * pg_dump -U gospeak gospeak | gzip > /backup/gospeak-$(date +\%Y\%m\%d).sql.gz
```

## 监控

### 健康检查端点

```bash
# 应用健康检查
curl -s http://localhost:8998/ping

# Docker healthcheck（已内置）
docker inspect --format='{{.State.Health.Status}}' gospeak-app
```

### 日志管理

```bash
# Docker 日志
docker compose logs --tail=100 gospeak
docker compose logs --tail=50 srs

# 日志轮转（Docker 默认已配）
# /etc/docker/daemon.json
{
  "log-driver": "json-file",
  "log-opts": {
    "max-size": "10m",
    "max-file": "3"
  }
}
```

## 升级流程

```bash
# 1. 拉取最新代码
cd /path/to/GOSpeak
git pull

# 2. 重新构建并启动
cd deploy
docker compose --profile srs --profile app up -d --build

# 3. 验证
curl -s http://localhost/ping
```
