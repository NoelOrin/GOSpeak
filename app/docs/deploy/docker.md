# 单容器部署

单容器部署通过一条 `docker run` 命令运行应用，不使用 Docker Compose。

## 适用场景

- 快速预览应用效果
- 服务器上已有 LiveKit，仅需部署应用
- 内网机器，无需复杂编排

## 拉取镜像

官方镜像托管在 GitHub 容器镜像仓库：

```bash
docker pull ghcr.io/noelorin/gospeak:1
```

标签有以下几种写法：

| 标签 | 含义 |
|------|------|
| `1` | 主版本 1 的最新版（随更新变化） |
| `1.2` | 1.2.x 的最新版 |
| `1.2.3` | 精确版本，固定不变 |

生产环境建议使用精确版本，避免某次更新导致服务异常。

镜像提供 `linux/amd64` 和 `linux/arm64` 两种架构，Docker 会自动选择匹配的架构。

## 运行容器

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

启动后访问 `http://服务器IP:8998` 即为应用界面。

### 参数说明

| 参数 | 作用 |
|------|------|
| `--restart unless-stopped` | 崩溃后自动重启，手动停止的容器不重启 |
| `--env-file` | 读取配置文件。先执行 `cd deploy && ./init.sh` 生成 |
| `-v gospeak-db:/app/db` | 数据库数据落盘，容器删除后数据仍然保留 |
| `-v gospeak-uploads:/app/uploads` | 上传文件落盘 |

**两个卷不能省略。** 不挂载卷时，容器重建后数据会全部丢失。

### 不使用 env 文件

直接通过 `-e` 传递环境变量：

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

## 语音服务配置

**该镜像不包含 LiveKit。**

`LIVEKIT_HOST` 必须指向一个应用容器能够访问的 LiveKit 实例：

- LiveKit 已在其他机器上运行 → 填写该机器的地址
- LiveKit 与应用部署在同一台机器 → 另起一个容器，并与应用放入同一个 Docker 网络：

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

在同一网络内，应用的 `LIVEKIT_HOST` 填写 `ws://livekit:7880`（使用服务名，而非 `localhost`——容器内的 localhost 指容器自身）。

注意上面的命令**没有映射 7880 端口**。这是有意为之：`/livekit` 由应用自身做同源反向代理，LiveKit 的信令端口只需应用容器能够访问，不必暴露到公网（compose 中也仅绑定回环地址 `127.0.0.1:7880` 供本机调试）。媒体端口（7882 tcp/udp、3478/udp）仍然需要映射，因为浏览器与 LiveKit 之间是直连的。

`LIVEKIT_PUBLIC_URL` 保持留空，应用会根据请求 Host 自动推导出 `wss://<你的域名>/livekit`。

若希望省去手动编排，可以使用 [Docker Compose](/deploy/compose)，它已将这些步骤串联完成。

## 自行构建镜像

需要修改代码或不使用官方镜像时：

```bash
# 仓库根目录
docker build -t gospeak .
```

该 Dockerfile 会在容器内构建前端并打包进 Go 二进制，最终镜像只包含一个可执行文件。

运行时将镜像名替换为自建镜像：

```bash
docker run -d ... gospeak
```

## 日常操作

```bash
# 查看日志
docker logs -f gospeak

# 停止 / 启动
docker stop gospeak
docker start gospeak

# 进入容器
docker exec -it gospeak sh

# 升级（数据卷保留不变）
docker pull ghcr.io/noelorin/gospeak:1
docker rm -f gospeak
# 重新执行上述 docker run
```

## 网络缓慢导致拉取失败

国内网络拉取 `ghcr.io` 可能速度较慢，可采用以下两种方式：

**配置镜像加速器**

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

**自行构建**

```bash
docker build -t gospeak .
```

基础镜像能够拉取时，该方式即可成功。

## 常见问题

**容器启动后立即退出**
执行 `docker logs gospeak` 查看原因。常见原因是配置缺失（`JWT_KEY` 为空）或数据库路径无权限。

**重启容器后数据丢失**
容器未挂载卷。启动命令中补上 `-v`。

**无法连接 LiveKit**
容器内的 `localhost` 指向容器自身。应使用宿主机 IP，或使用 `--network host`，或改用 compose。

**页面正常但语音无法连接**
确认浏览器获取的 `serverUrl` 是 `wss://<你的域名>/livekit`，并且反向代理已放行 `/livekit` 的 WebSocket 升级。

**端口 8998 被占用**
更换端口映射：`-p 9000:8998`。

**时区修改方法**
```bash
-e TZ=Asia/Shanghai
```

## 下一步

- [Docker Compose](/deploy/compose) — 推荐方案，将 LiveKit 一并管理
- [配置项](/guide/configuration) — 全部环境变量
