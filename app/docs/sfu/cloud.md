# LiveKit Cloud

不想管服务器就用这个。注册完拿到三个值填进去，语音就能用。

## 什么时候值得用

- 没有服务器运维经验
- 用户分布在不同地区，需要全球节点
- 不想处理 NAT 穿透、TURN、证书这些事
- 用量小，按量付费比租服务器便宜

## 什么时候不值得

- 用户都在国内，Cloud 的节点在境外，延迟会高
- 用量大，按量计费可能比自建贵不少
- 数据不能出境

## 开通

1. 去 [LiveKit Cloud](https://cloud.livekit.io) 注册
2. 建一个项目
3. 在项目设置的 Keys 页面拿到三个值：
   - **WebSocket URL**：`wss://你的项目.livekit.cloud`
   - **API Key**：`API` 开头
   - **API Secret**

## 配置

`deploy/.env`：

```ini
SFU_PROVIDER=livekit
LIVEKIT_HOST=wss://你的项目.livekit.cloud
# Cloud 是外部服务，不经过应用的同源 /livekit 反代，必须显式填浏览器侧地址
LIVEKIT_PUBLIC_URL=wss://你的项目.livekit.cloud
LIVEKIT_KEY=APIxxxxxxxx
LIVEKIT_SECRET=你的secret
```

重启应用，语音就能用了。

> `LIVEKIT_PUBLIC_URL` 留空是**自建 LiveKit** 的默认形态（走应用内置的同源 `/livekit` 反代，地址按请求自动推导）。用 Cloud 时请显式填写，否则浏览器会被指到你的站点域名而不是 Cloud。

**本地不用起 Docker。** 这是云服务最直接的好处——开发机上只需要 Go 和 Node。

## 最小开发环境

```ini
APP_ENV=dev
DB_TYPE=SQLite
JWT_KEY=dev-secret
SFU_PROVIDER=livekit
LIVEKIT_HOST=wss://你的项目.livekit.cloud
LIVEKIT_PUBLIC_URL=wss://你的项目.livekit.cloud
LIVEKIT_KEY=APIxxxxxxxx
LIVEKIT_SECRET=你的secret
STORAGE_TYPE=local
```

```bash
pnpm dev
```

就这样，不用 `docker compose up`。

## 和自建的差异

| | 自建 | Cloud |
|---|---|---|
| 部署 | 要跑容器、开端口 | 填三个值 |
| 数据 | 全在自己手里 | 音频经过 LiveKit 服务器 |
| 延迟 | 取决于你的服务器位置 | 取决于最近的 Cloud 节点 |
| 费用 | 服务器固定成本 | 按音频分钟数计费 |
| 扩容 | 自己加机器 | 自动 |
| TURN | 自己配 | 内置 |
| 国内访问 | 好 | 看情况，可能不稳 |

## 计费要注意什么

按**参与者音频分钟数**算，不是按房间数。具体价格看官网。

省钱的做法：

- 空房间要及时关（GOSpeak 不会自动关空房间，需要前端在最后一个人离开时删房间，或者配一个定时清理）
- 别开视频，音频便宜得多
- 开发测试用自建，生产用 Cloud 或者反过来

## 国内访问的现实情况

LiveKit Cloud 的节点主要在境外。国内用户连接可能：

- 延迟偏高（100ms+）
- 偶尔丢包导致声音断续
- 极端情况下连不上

如果用户主要在国内，自建一台国内服务器通常体验更好。

## 迁移

从 Cloud 换回自建，或者反过来，只改三个环境变量：

```ini
LIVEKIT_HOST=...
LIVEKIT_KEY=...
LIVEKIT_SECRET=...
```

重启即可。已经建立的连接会断，用户重进频道就好。

## 常见问题

**连接超时**
检查 `LIVEKIT_HOST` 是不是 `wss://` 开头，注意不要漏了协议。

**提示 token 无效**
Key 和 Secret 不配对。去 Cloud 控制台重新复制一遍，注意别把 Secret 的空格带进去。

**能连上但没声音**
Cloud 一般不会有媒体端口问题。看浏览器控制台，可能是麦克风权限没给。

**费用涨得比预期快**
去 Cloud 控制台的用量页面看是哪个项目在烧。多半是空房间没关，一直有参与者挂在那儿。

## 下一步

- [语音服务](/sfu/) — 整体架构
- [自建 LiveKit](/sfu/selfhost) — 想自己控制的话
