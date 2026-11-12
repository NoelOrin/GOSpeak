# 五分钟跑起来

本页说明如何在本地启动 GOSpeak，完成语音与文字功能的验证。

## 前置依赖

| 工具 | 版本 | 用途 |
|------|------|------|
| Go | 1.26+ | 编译后端 |
| Node.js | 20+ | 编译前端 |
| pnpm | 10+ | 安装依赖（仓库为 pnpm 工作区） |
| Docker | 最新 | 运行 LiveKit 语音服务 |
| gcc | 任意 | SQLite 需编译 C 代码，Windows 安装 MinGW |

未安装 Docker 时仍可启动，文字功能不受影响，语音频道无法使用。

## 第一步：获取代码并安装依赖

```bash
git clone https://github.com/NoelOrin/GOSpeak.git
cd GOSpeak
pnpm install
```

## 第二步：生成配置文件

```bash
cd deploy && ./init.sh && cd ..
```

打开 `deploy/.env`，本地开发只需确认以下几行（本地开发浏览器与后端同机，`LIVEKIT_PUBLIC_URL` 保持留空）：

```ini
DB_TYPE="SQLite"
SFU_PROVIDER="livekit"
LIVEKIT_HOST="ws://localhost:7880"
LIVEKIT_KEY="devkey"
LIVEKIT_SECRET="devsecret"
```

`LIVEKIT_KEY` / `LIVEKIT_SECRET` 可任意填写，但需与 LiveKit 容器侧配置一致。

> 本地开发时应用运行在宿主机，需连接 `localhost:7880`。compose 中 `livekit` 的 7880 已绑定到宿主机回环地址（`127.0.0.1:7880`），可直接连接，无需修改端口映射。该端口仅对本机可见，不对公网开放。

## 第三步：启动语音服务

```bash
cd deploy
./init.sh
docker compose up -d
```

`init.sh` 生成 `deploy/.env`（含随机密钥），compose 将密钥注入 LiveKit 容器，`livekit.yaml` 不含任何凭据，因此无需手动对齐两侧 key。

检查 LiveKit 状态：

```bash
docker compose ps livekit
```

`Up` 状态表示 LiveKit 已运行。7880 为信令端口，绑定在宿主机回环地址上，本机 `curl localhost:7880` 可通，公网无法访问——浏览器经应用同源的 `/livekit` 访问。

## 第四步：启动应用

回到仓库根目录：

```bash
pnpm dev
```

该命令同时启动后端与前端。两者就绪后：

- 界面地址 `http://localhost:3000`
- 后端地址 `http://localhost:8998`
- 接口文档 `http://localhost:8998/swagger/index.html`

## 第五步：登录

首次启动自动创建管理员账号：

| 用户名 | 密码 |
|--------|------|
| `admin` | `admin123` |

登录后系统提示修改密码。

> 该默认账号仅在库中不存在 `admin` 时创建一次。公网部署前必须修改密码。

## 第六步：验证语音

1. 左侧栏点「域」，创建一个语音域
2. 在域中创建一个语音频道
3. 进入频道，浏览器请求麦克风权限，允许
4. 头像亮起、说话时显示绿色光环，表示语音已接通

验证多人效果时，另开一个无痕窗口使用其他账号登录并进入同一频道。

## 常见问题

**界面白屏 / 接口全 404**
后端未启动。查看 `pnpm dev` 输出中 server 部分的报错，多为 `deploy/.env` 中数据库或端口配置问题。

**能登录但进语音频道没有声音**
LiveKit 未运行或无法连接。先执行 `docker compose ps` 查看 livekit 容器状态，再执行 `docker compose logs livekit` 查看报错。`LIVEKIT_PUBLIC_URL` 保持留空，应用自动推导同源的 `/livekit` 地址。

**端口被占用**
后端默认 8998，前端默认 3000。通过 `SERVER_PORT` 修改后端端口，前端在 3000 被占时自动顺延至下一个可用端口。

**Windows 编译报 CGO 错误**
SQLite 需要 C 编译器。安装 MinGW-w64 并将 `gcc` 加入 PATH。

## 下一步

- 局域网或公网部署 → [部署到服务器](/deploy/)
- 桌面客户端安装与使用 → [桌面客户端](/guide/desktop)
- 服务端配置项 → [配置项](/guide/configuration)
- 界面各区域说明 → [界面导览](/guide/tour)
