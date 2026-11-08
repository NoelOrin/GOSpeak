# 五分钟跑起来

这篇只做一件事：让你在本地看到一个能说话、能聊天的 GOSpeak。

## 先装这几个东西

| 工具 | 版本 | 干什么用的 |
|------|------|-----------|
| Go | 1.26+ | 编译后端 |
| Node.js | 20+ | 编译前端 |
| pnpm | 10+ | 装依赖（仓库是 pnpm 工作区） |
| Docker | 最新 | 跑 LiveKit 语音服务，没有的话语音功能用不了 |
| gcc | 任意 | SQLite 要编译 C 代码，Windows 上装 MinGW |

没有 Docker 也能启动，只是进语音频道会失败——文字功能不受影响。

## 第一步：拿代码、装依赖

```bash
git clone https://github.com/NoelOrin/GOSpeak.git
cd GOSpeak
pnpm install
```

## 第二步：准备配置文件

```bash
cp app/server/.env.example app/server/.env.dev
```

打开 `app/server/.env.dev`，先只改下面四行就够跑起来：

```ini
DB_TYPE="SQLite"
SFU_PROVIDER="livekit"
LIVEKIT_HOST="ws://localhost:7880"
LIVEKIT_KEY="devkey"
LIVEKIT_SECRET="devsecret"
```

`LIVEKIT_KEY` / `LIVEKIT_SECRET` 随便填，但要和下一步 LiveKit 那边的配置一致。

## 第三步：起语音服务

```bash
cd deploy
./init.sh livekit
docker compose up -d
```

`init.sh` 会生成 `deploy/.env`（含随机密钥）并把密钥同步写进 LiveKit 配置，所以不用手动对齐。

检查一下有没有起来：

```bash
curl -s http://localhost:7880
```

有响应就说明 LiveKit 在跑了。

## 第四步：起应用

回仓库根目录：

```bash
pnpm dev
```

这条命令同时起后端和前端。等两个都就绪：

- 打开 `http://localhost:3000` 就是界面
- 后端在 `http://localhost:8998`
- 接口文档在 `http://localhost:8998/swagger/index.html`

## 第五步：登录进去

首次启动会自动创建管理员账号：

| 用户名 | 密码 |
|--------|------|
| `admin` | `admin123` |

登录后系统会提示你改密码，改掉就好。

> 这个默认账号只在库里没有 `admin` 时创建一次。**放到公网前一定要改密码。**

## 第六步：试试语音

1. 左侧栏点「域」，建一个语音域
2. 在域里建一个语音频道
3. 点进频道，浏览器会问麦克风权限，允许
4. 看到自己的头像亮起来、说话时有绿圈，就通了

想验证多人效果，开一个无痕窗口用另一个账号登进来，进同一个频道。

## 卡住了看这里

**界面白屏 / 接口全 404**
后端没起来。看 `pnpm dev` 的输出里 server 那部分有没有报错，多半是 `.env.dev` 里数据库或端口配置有问题。

**能登录但进语音频道没声音**
LiveKit 没跑或者密钥对不上。先 `docker compose ps` 看 livekit 容器在不在，再核对 `.env.dev` 里的 `LIVEKIT_KEY` / `LIVEKIT_SECRET` 和 `deploy/.env` 是否一致。

**端口被占用**
后端默认 8998，前端默认 3000。改后端端口用 `SERVER_PORT`，前端会在 3000 被占时自动顺延到下一个可用端口。

**Windows 上编译报 CGO 错误**
SQLite 需要 C 编译器。装 MinGW-w64 并把 `gcc` 加进 PATH。

## 下一步

- 想让局域网或公网的朋友连进来 → [部署到服务器](/deploy/)
- 想用桌面客户端 → [桌面客户端](/guide/desktop)
- 想调参数 → [配置项](/guide/configuration)
- 想知道每个按钮干什么 → [界面导览](/guide/tour)
