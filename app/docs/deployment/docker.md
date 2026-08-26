# 单容器 Docker 部署

适合快速验证、边缘设备或内网机器：一个容器运行完整的 GOSpeak（Go 后端 + 前端 SPA），无需 docker-compose 编排。

> 生产环境或需要同时部署 SFU（SRS / LiveKit）、PostgreSQL、RustFS 时，建议使用 [Docker Compose](/deployment/docker-compose) 一键编排。

## 镜像源

官方发布镜像托管在 GitHub 容器镜像仓库 `ghcr.io`，每个 Release 自动构建并推送 `linux/amd64` 与 `linux/arm64` 双架构镜像。

镜像地址为 `ghcr.io/noelorin/gospeak`，tag 由 Release 版本号推导：

| tag | 含义 | 示例 |
|-----|------|------|
| `x.y.z` | 精确版本 | `ghcr.io/noelorin/gospeak:1.2.3` |
| `x.y` | 次版本（自动跟随小版本更新）| `ghcr.io/noelorin/gospeak:1.2` |
| `x` | 主版本（自动跟随全部更新）| `ghcr.io/noelorin/gospeak:1` |

拉取镜像：

```bash
docker pull ghcr.io/noelorin/gospeak:1
```

无法访问 `ghcr.io` 时，可改用下方的「本地构建」，或在守护进程配置「镜像加速」后再拉取。

## 本地构建

从仓库根目录自行构建一体镜像（Go 后端 + 前端 SPA）：

```bash
docker build -t gospeak .
```

## 镜像加速（registry mirror）

部分网络环境下 `docker pull` 官方源速度慢或不稳定，可在 Docker 守护进程配置镜像加速器作为拉取代理。编辑 `/etc/docker/daemon.json`（不存在则新建）：

```json
{
  "registry-mirrors": [
    "https://<你的镜像加速地址>"
  ]
}
```

重启守护进程后生效：

```bash
sudo systemctl daemon-reload
sudo systemctl restart docker
```

> 镜像加速器只代理 `docker pull` 的镜像拉取、不改变镜像内容；请仅使用你信任的加速源。

## 运行

最小可运行实例（SQLite + 内存 NATS），数据通过命名卷持久化：

```bash
docker run -d \
  --name gospeak \
  --restart unless-stopped \
  -p 8998:8998 \
  --env-file deploy/env/app.srs.env \
  -v gospeak-db:/app/db \
  -v gospeak-uploads:/app/uploads \
  ghcr.io/noelorin/gospeak:1
```

- 首次访问：`http://<host>:8998/`；`/ping` 为健康检查端点（容器内已内置 HEALTHCHECK）。
- 数据目录：`/app/db`（SQLite）、`/app/uploads`（对象存储），建议用卷持久化，避免容器重建丢数据。
- 环境变量说明见 [环境变量参考](/guide/configuration)。

若改用本地构建的 `gospeak` 镜像，把最后一行镜像名替换为 `gospeak` 即可。

### 环境配置

`--env-file` 指向 `deploy/env/app.srs.env`（需先 `cp env/app.srs.env.example env/app.srs.env` 并修改 `JWT_KEY` / `SRS_SECRET`）。也可直接透传关键变量：

```bash
docker run -d \
  --name gospeak \
  --restart unless-stopped \
  -p 8998:8998 \
  -e JWT_KEY="$(openssl rand -hex 32)" \
  -e SFU_PROVIDER=srs \
  -e SRS_HOST=srs \
  -e SRS_SECRET="$(openssl rand -hex 32)" \
  -v gospeak-db:/app/db \
  -v gospeak-uploads:/app/uploads \
  ghcr.io/noelorin/gospeak:1
```

> 单容器内不含 SFU；`SRS_HOST` / `LIVEKIT_HOST` 必须指向容器外可达的 SFU 实例，否则语音功能不可用。

## 常用操作

```bash
# 查看日志
docker logs -f gospeak

# 停止 / 启动
docker stop gospeak
docker start gospeak

# 升级：拉取新 tag 后重建容器（数据卷保留）
docker pull ghcr.io/noelorin/gospeak:1
docker rm -f gospeak
docker run -d --name gospeak --restart unless-stopped -p 8998:8998 \
  --env-file deploy/env/app.srs.env \
  -v gospeak-db:/app/db -v gospeak-uploads:/app/uploads \
  ghcr.io/noelorin/gospeak:1

# 卸载（保留数据卷）
docker rm -f gospeak
```
