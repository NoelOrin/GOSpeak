# 单容器部署

一个 `docker run` 就把应用跑起来，不搞 compose 那套。

## 什么时候这样用

- 只想快速看看长什么样
- 服务器上已经有 LiveKit 了，只差应用
- 内网机器，用不着复杂编排

## 先拉镜像

官方镜像在 GitHub 容器镜像仓库：

```bash
docker pull ghcr.io/noelorin/gospeak:1
```

标签有几种写法：

| 标签 | 意思 |
|------|------|
| `1` | 主版本 1 的最新版（会跟着更新） |
| `1.2` | 1.2.x 的最新版 |
| `1.2.3` | 精确版本，不会变 |

生产环境建议用精确版本，免得某次更新把服务搞挂。

镜像有 `linux/amd64` 和 `linux/arm64` 两个架构，Docker 会自动选对的。

## 跑

```bash
docker run -d \
  --name gospeak \
  --restart unless-stopped \
  -p 8998:8998 \
  --env-file deploy/.env \
  -v gospeak-db:/app/db \
  -v gospeak-uploads:/app/uploads \
  ghcr.io/noelorin/gospeak:1
```

打开 `http://服务器IP:8998` 就是界面。

### 这几个参数在干什么

| 参数 | 作用 |
|------|------|
| `--restart unless-stopped` | 崩了自动重启，手动停的不重启 |
| `--env-file` | 读配置。先 `cd deploy && ./init.sh` 生成 |
| `-v gospeak-db:/app/db` | 数据库落盘，容器删了数据还在 |
| `-v gospeak-uploads:/app/uploads` | 上传的文件落盘 |

**两个卷不能省。** 不挂卷的话容器一重建数据全没。

### 不想用 env 文件

直接传环境变量：

```bash
docker run -d \
  --name gospeak \
  --restart unless-stopped \
  -p 8998:8998 \
  -e JWT_KEY="$(openssl rand -hex 32)" \
  -e SFU_PROVIDER=livekit \
  -e LIVEKIT_HOST=ws://你的LiveKit地址:7880 \
  -e LIVEKIT_KEY=你的key \
  -e LIVEKIT_SECRET=你的secret \
  -v gospeak-db:/app/db \
  -v gospeak-uploads:/app/uploads \
  ghcr.io/noelorin/gospeak:1
```

## 语音怎么办

**这个镜像里没有 LiveKit。**

`LIVEKIT_HOST` 必须指向一个应用容器能访问到的 LiveKit：

- 已经在别的机器上跑着 → 填那台的地址
- 想在同一台机器上跑 → 另起一个容器，并和应用放进同一个 Docker 网络：

```bash
docker network create gospeak

docker run -d \
  --name livekit \
  --network gospeak \
  --restart unless-stopped \
  -p 7882:7882 \
  -p 7882:7882/udp \
  -p 3478:3478/udp \
  -v $(pwd)/deploy/livekit/livekit.yaml:/etc/livekit.yaml \
  livekit/livekit-server:latest \
  --config /etc/livekit.yaml

docker run -d \
  --name gospeak \
  --network gospeak \
  --restart unless-stopped \
  -p 8998:8998 \
  --env-file deploy/.env \
  -v gospeak-db:/app/db \
  -v gospeak-uploads:/app/uploads \
  ghcr.io/noelorin/gospeak:1
```

同一网络内，应用的 `LIVEKIT_HOST` 填 `ws://livekit:7880`（用服务名，不是 `localhost`——容器里的 localhost 是容器自己）。

注意上面**没有映射 7880**。这是有意的：`/livekit` 由应用自己同源反代，LiveKit 的信令端口只需要应用容器能访问，不必暴露到公网（compose 里也只绑回环 `127.0.0.1:7880` 供本机调试）。媒体端口（7882 tcp/udp、3478/udp）仍然要映射，因为浏览器和 LiveKit 是直连的。

`LIVEKIT_PUBLIC_URL` 保持留空，应用会按请求 Host 自动推导出 `wss://<你的域名>/livekit`。

嫌麻烦就用 [Docker Compose](/deploy/compose)，它把这些都串好了。

## 自己构建镜像

想改代码或者不用官方镜像：

```bash
# 仓库根目录
docker build -t gospeak .
```

这个 Dockerfile 会在容器里构建前端并打进 Go 二进制，最终镜像只有一个可执行文件。

跑的时候把镜像名换掉：

```bash
docker run -d ... gospeak
```

## 日常操作

```bash
# 看日志
docker logs -f gospeak

# 停 / 起
docker stop gospeak
docker start gospeak

# 进容器看看
docker exec -it gospeak sh

# 升级（数据卷保留）
docker pull ghcr.io/noelorin/gospeak:1
docker rm -f gospeak
# 重新跑一遍上面的 docker run
```

## 网络慢拉不下来

国内网络拉 `ghcr.io` 可能很慢。两个办法：

**配镜像加速器**

编辑 `/etc/docker/daemon.json`：

```json
{
  "registry-mirrors": ["https://你的加速地址"]
}
```

```bash
sudo systemctl daemon-reload
sudo systemctl restart docker
```

**自己构建**

```bash
docker build -t gospeak .
```

只要基础镜像能拉到，这个就能成。

## 常见问题

**容器起来马上退出**
`docker logs gospeak` 看原因。常见的是配置缺失（`JWT_KEY` 为空）或数据库路径没权限。

**重启容器后数据没了**
没挂卷。加上 `-v`。

**连不上 LiveKit**
容器里的 `localhost` 指容器自己。要用宿主机 IP，或者用 `--network host`，或者干脆用 compose。

**语音连不上但页面正常**
确认浏览器拿到的 `serverUrl` 是 `wss://<你的域名>/livekit`，并且反代放行了 `/livekit` 的 WebSocket 升级。

**端口 8998 被占**
换一个：`-p 9000:8998`。

**怎么改时区**
```bash
-e TZ=Asia/Shanghai
```

## 下一步

- [Docker Compose](/deploy/compose) — 推荐方案，把 LiveKit 一起管
- [配置项](/guide/configuration) — 全部环境变量
