# 单二进制部署

GOSpeak 支持编译为单文件二进制部署形态：前端资源已内嵌其中，将二进制复制到目标机器即可直接运行。

## 适用场景

- 裸机部署，且未安装 Docker
- 树莓派、NAS、软路由等资源有限的小型设备
- 无法安装容器的内网机器
- 需要注册为 systemd 服务

## 不适用场景

单二进制**只包含应用本身**，不含 LiveKit。因此：

- 仅文字聊天 → 单二进制已可满足需求
- 需要语音 → 另行部署 LiveKit，并通过 `LIVEKIT_HOST` 指向该实例

若希望减少部署步骤，可使用 [Docker Compose](/deploy/compose)，该方案会一并管理 LiveKit。

## 下载

从 [Releases](https://github.com/NoelOrin/GOSpeak/releases) 页面下载 `gospeak-server-v` 开头的版本：对应二进制位于各 release 页面底部的 **Assets** 列表中，文件名形如 `gospeak-server-v<版本>-<系统>-<架构>`：

| 文件 | 平台 |
|------|------|
| `gospeak-server-v1.0.0-linux-amd64` | Linux x86_64 |
| `gospeak-server-v1.0.0-linux-arm64` | Linux ARM64（ARM 服务器、部分 NAS） |
| `gospeak-server-v1.0.0-linux-armv7` | Linux ARMv7（树莓派 2/3） |
| `gospeak-server-v1.0.0-darwin-amd64` | macOS Intel |
| `gospeak-server-v1.0.0-darwin-arm64` | macOS Apple Silicon |
| `gospeak-server-v1.0.0-windows-amd64.exe` | Windows x64 |
| `gospeak-server-v1.0.0-windows-arm64.exe` | Windows on ARM |
| `gospeak-server-v1.0.0-freebsd-*` | FreeBSD |
| `gospeak-server-v1.0.0-openbsd-amd64` | OpenBSD |

同一页面还提供 `SHA256SUMS-gospeak-server-v1.0.0.txt` 校验和文件，用于校验完整性：

```bash
sha256sum -c SHA256SUMS-gospeak-server-v1.0.0.txt --ignore-missing
```

## 运行

```bash
chmod +x gospeak-server-v1.0.0-linux-amd64
./gospeak-server-v1.0.0-linux-amd64 server -e prod
```

服务默认监听 8998 端口，浏览器访问 `http://服务器IP:8998` 即进入 Web 界面。

前端资源已内嵌于二进制，无需额外部署静态文件。

### 参数

| 参数 | 作用 |
|------|------|
| `server` | 启动服务（默认 `prod` 环境） |
| `server -e dev` | 开发环境，读 `.env.dev` |
| `version` | 打印版本号 |

启动时会在**当前工作目录**读取 `.env` 文件，因此需在二进制所在目录放置 `.env`，或改用环境变量提供配置。

## 配置 systemd 服务

systemd 方式提供开机自启、崩溃自动重启与 journald 日志管理。

`/etc/systemd/system/gospeak.service`：

```ini
[Unit]
Description=GOSpeak
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=gospeak
Group=gospeak
WorkingDirectory=/opt/gospeak
ExecStart=/opt/gospeak/gospeak server -e prod
Restart=always
RestartSec=5
LimitNOFILE=65536

# 环境变量也可写在此处，或置于 /opt/gospeak/.env
Environment=DB_TYPE=SQLite
Environment=DB_PATH=/opt/gospeak/data/app.db
Environment=SERVER_PORT=8998

[Install]
WantedBy=multi-user.target
```

准备目录与用户：

```bash
sudo useradd -r -s /sbin/nologin gospeak
sudo mkdir -p /opt/gospeak/data
sudo cp gospeak-server-v1.0.0-linux-amd64 /opt/gospeak/gospeak
sudo chown -R gospeak:gospeak /opt/gospeak
```

启动：

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now gospeak
sudo systemctl status gospeak
```

查看日志：

```bash
journalctl -u gospeak -f
```

## 外部前端目录

该方式通常无需使用；需要在不重新编译二进制的前提下更新前端时，可外挂一个静态目录：

```bash
STATIC_DIR=/var/www/gospeak ./gospeak-server-v1.0.0-linux-amd64 server -e prod
```

优先级：`STATIC_DIR` > `/app/static` > `./static` > 二进制内嵌。

## 自行编译

```bash
cd app/server

# 先构建前端（会同步到 internal/webui/dist）
make web

# 单个平台
make linux-amd64-bin

# 一个系统下的所有架构
make linux
make darwin
make windows

# 全部
make all
```

编译产物位于 `app/server/build/`。

## 常见问题

**启动报 `no such file or directory` 找不到 .env**
进程的工作目录中不存在配置文件。通过 `WorkingDirectory` 指定工作目录，或将 `.env` 放置在二进制所在目录。

**SQLite 报权限错误**
运行用户对 `DB_PATH` 所在目录没有写权限，需通过 `chown` 修正目录属主。

**修改 .env 后未生效**
重启进程即可生效；systemd 部署下执行 `sudo systemctl restart gospeak`。

**内存占用持续增长**
Go 运行时的正常现象，内存占用会趋于稳定。若持续增长且不回落，可在项目中提交 issue。

**更换端口**
使用 `SERVER_PORT=9000 ./gospeak server -e prod` 启动，或将该变量写入 systemd 单元的 `Environment` 配置。

## 下一步

- [部署到服务器](/deploy/) — Docker 方案
- [配置项](/guide/configuration) — 全部环境变量
