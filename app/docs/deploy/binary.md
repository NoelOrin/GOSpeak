# 单二进制部署

不想装 Docker？GOSpeak 可以编成一个文件，前端已经打进去了，拷过去就能跑。

## 适合什么场景

- 裸机部署，不想装 Docker
- 树莓派、NAS、软路由这类资源有限的小机器
- 内网机器，装不了容器
- 想做成 systemd 服务

## 不适合什么场景

单二进制**只包含应用本身**，不含 LiveKit。所以：

- 只想聊天 → 够了
- 要语音 → 还得另外部署 LiveKit，然后配 `LIVEKIT_HOST` 指过去

如果嫌麻烦，用 [Docker Compose](/deploy/compose)，它把 LiveKit 一起管了。

## 下载

去 [Releases](https://github.com/NoelOrin/GOSpeak/releases) 找 `gospeak-` 开头的文件：

| 文件 | 平台 |
|------|------|
| `gospeak-linux-amd64` | Linux x86_64 |
| `gospeak-linux-arm64` | Linux ARM64（ARM 服务器、部分 NAS） |
| `gospeak-linux-armv7` | Linux ARMv7（树莓派 2/3） |
| `gospeak-darwin-amd64` | macOS Intel |
| `gospeak-darwin-arm64` | macOS Apple Silicon |
| `gospeak-windows-amd64.exe` | Windows x64 |
| `gospeak-windows-arm64.exe` | Windows on ARM |
| `gospeak-freebsd-*` | FreeBSD |
| `gospeak-openbsd-amd64` | OpenBSD |

## 跑起来

```bash
chmod +x gospeak-linux-amd64
./gospeak-linux-amd64 server -e prod
```

默认监听 8998。浏览器打开 `http://服务器IP:8998` 就是界面。

前端已经内嵌在二进制里，不用另外放静态文件。

### 参数

| 参数 | 作用 |
|------|------|
| `server` | 启动服务（默认 `prod` 环境） |
| `server -e dev` | 开发环境，读 `.env.dev` |
| `version` | 打印版本号 |

启动时会在**当前工作目录**读 `.env` 文件。所以要么在二进制旁边放一个 `.env`，要么用环境变量。

## 配成 systemd 服务

这是最省心的方式——开机自启、崩溃自动重启、日志进 journald。

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

# 环境变量也可以写在这里，或者放 /opt/gospeak/.env
Environment=DB_TYPE=SQLite
Environment=DB_PATH=/opt/gospeak/data/app.db
Environment=SERVER_PORT=8998

[Install]
WantedBy=multi-user.target
```

准备目录和用户：

```bash
sudo useradd -r -s /sbin/nologin gospeak
sudo mkdir -p /opt/gospeak/data
sudo cp gospeak-linux-amd64 /opt/gospeak/gospeak
sudo chown -R gospeak:gospeak /opt/gospeak
```

启动：

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now gospeak
sudo systemctl status gospeak
```

看日志：

```bash
journalctl -u gospeak -f
```

## 用外部前端目录

一般不需要，但如果你想改前端又不重新编译二进制，可以外挂一个静态目录：

```bash
STATIC_DIR=/var/www/gospeak ./gospeak server -e prod
```

优先级：`STATIC_DIR` > `/app/static` > `./static` > 二进制内嵌。

## 编译自己的

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

产物在 `app/server/build/`。

## 常见问题

**启动报 `no such file or directory` 找不到 .env**
二进制在别的目录找不到配置文件。用 `WorkingDirectory` 指定，或者直接把 `.env` 放二进制旁边。

**SQLite 报权限错误**
运行用户对 `DB_PATH` 所在目录没有写权限。`chown` 一下。

**改了 .env 不生效**
重启进程。systemd 的话 `sudo systemctl restart gospeak`。

**内存占用慢慢涨**
Go 程序正常现象，会稳定。一直涨不回落的话提 issue。

**想换端口**
`SERVER_PORT=9000 ./gospeak server -e prod`，或者写进 systemd 的 Environment。

## 下一步

- [部署到服务器](/deploy/) — Docker 方案
- [配置项](/guide/configuration) — 全部环境变量
