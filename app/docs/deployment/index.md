# 部署概述

GOSpeak 支持从本地开发到生产环境的**渐进式部署**方案，根据需求逐步增加组件：

```
最小栈 (SQLite + SRS)
    │
    ├─ 加 Nginx → 公网同源反代
    │
    ├─ 换 PostgreSQL → 更高并发
    │
    ├─ 加外部 NATS → 跨实例状态共享 + JWT 黑名单/密钥轮换
    │
    ├─ 加 RustFS → S3 对象存储
    │
    └─ 加 Coturn → TURN 中继穿透
```

## 部署方式对比

| 方式 | 用途 | 难度 |
|------|------|------|
| Docker Compose（推荐）| 生产部署、本地测试 | ⭐⭐ |
| [Docker 单容器](/deployment/docker) | 快速验证 | ⭐ |
| 二进制 + 系统服务 | 裸机部署 | ⭐⭐⭐ |
| K8s（需自行编排）| 大规模集群 | ⭐⭐⭐⭐ |

## 网络架构

```
公网用户
    │
    ▼
Nginx :80/:443（可选）
    ├── /api /ws    → GOSpeak:8998
    ├── /rtc/v1/* (SRS 时) → SRS:1985
    └── / → 静态文件 / SPA 分发

GOSpeak (Go)
    ├── DB: SQLite/PG
    ├── NATS KV (可选): JWT黑名单、密钥轮换、房间状态
    └── SFU: LiveKit/SRS/Agora/Cloudflare

浏览器 ──WebRTC──► SFU 媒体端口（直连）
```

## 内容导航

- [Docker Compose 部署](/deployment/docker-compose) — 完整编排方案
- [单容器 Docker 部署](/deployment/docker) — 镜像源与 docker run 快速验证
- [生产部署](/deployment/production) — 生产环境 Checklist
- [数据库演进](/deployment/database) — 从 SQLite 到 PostgreSQL
- [Nginx 配置](/deployment/nginx) — 反代与 HTTPS 配置
