# 单二进制部署

GOSpeak 从 v0.1 开始支持**前端打进二进制**的纯单文件分发。一个二进制文件包含完整的 Go 后端 + 前端 SPA，无需 Node.js、无需 pnpm、无需复制静态目录。

## 适用场景

- **裸机部署**：直接下载，`chmod +x` 即可运行
- **边缘设备**：树莓派、OpenWrt 路由器、NAS 等资源受限环境
- **内网穿透**：二进制复制到内网机器，零依赖启动
- **CI/CD Pipeline**：产出物是单一文件，容器镜像也更小

## 获取二进制

### 从 GitHub Release 下载

每次 Release 自动产出以下平台的单文件二进制：

```
gospeak-linux-amd64
gospeak-linux-arm64
gospeak-linux-armv7          # 树莓派 2/3 32 位
gospeak-darwin-amd64
gospeak-darwin-arm64          # Apple Silicon
gospeak-freebsd-amd64
gospeak-freebsd-arm64
gospeak-openbsd-amd64
gospeak-windows-amd64.exe
gospeak-windows-arm64.exe     # Snapdragon X / Surface Pro
```

### 自行编译

```bash
cd app/server

# 先构建前端
make web

# 单个平台
make linux-amd64-bin

# 一组平台
make linux
make darwin
make windows
make freebsd

# 全平台
make all
```

产物在 `app/server/build/` 目录下。

### Docker 镜像

Dockerfile 在容器内构建前端并 embed，最终镜像也只有一个二进制：

```bash
docker build -t gospeak .
```

## 运行

```bash
# Linux / macOS
chmod +x gospeak-linux-amd64
./gospeak-linux-amd64 server -e prod

# Windows
./gospeak-windows-amd64.exe server -e prod
```

前端在二进制启动后自动提供：`http://<host>:<port>/` 直接打开页面。

## 运行时的前端覆盖

仍可通过 `STATIC_DIR` 环境变量外挂自定义静态目录，覆盖内嵌的前端：

```bash
STATIC_DIR=/path/to/custom/static ./gospeak-linux-amd64 server -e prod
```

优先级：`STATIC_DIR` > `/app/static` > `./static` > **二进制内嵌前端**。

## 技术实现

- 前端构建产物在编译前复制到 `internal/webui/dist/`
- 通过 Go 标准库 `go:embed` 在编译期打进二进制
- 运行时 `serveSPA` 回退到 `embed.FS`，峰值性能无额外开销
- 内嵌资源约 **4MB**（前端 gzip 后约 1.5MB）
