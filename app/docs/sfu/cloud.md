# LiveKit Cloud

LiveKit Cloud 为免运维方案，注册后填入三个值即可启用语音。

## 适用场景

- 无服务器运维经验
- 用户分布在不同地区，需要全球节点
- 不希望处理 NAT 穿透、TURN、证书等事项
- 用量较小，按量付费低于租用服务器成本

## 不适用场景

- 用户集中在国内，Cloud 节点在境外，延迟较高
- 用量大，按量计费成本可能显著高于自建
- 数据不允许出境

## 开通

1. 在 [LiveKit Cloud](https://cloud.livekit.io) 注册
2. 创建一个项目
3. 在项目设置的 Keys 页面获取三个值：
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

重启应用后语音即可使用。

> `LIVEKIT_PUBLIC_URL` 留空是**自建 LiveKit** 的默认形态（走应用内置的同源 `/livekit` 反代，地址按请求自动推导）。使用 Cloud 时须显式填写，否则浏览器会被指向站点域名而非 Cloud。

**本地无需启动 Docker。** 这是云服务最直接的优势——开发机仅需 Go 与 Node。

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

即配置完成，无需 `docker compose up`。

## 与自建的差异

| | 自建 | Cloud |
|---|---|---|
| 部署 | 需运行容器、开放端口 | 填三个值 |
| 数据 | 数据完全自持 | 音频经过 LiveKit 服务器 |
| 延迟 | 取决于服务器位置 | 取决于最近的 Cloud 节点 |
| 费用 | 服务器固定成本 | 按音频分钟数计费 |
| 扩容 | 自行增加机器 | 自动 |
| TURN | 自行配置 | 内置 |
| 国内访问 | 良好 | 视情况，可能不稳定 |

## 计费说明

按**参与者音频分钟数**计费，与房间数无关。具体价格见官网。

成本控制方式：

- 空房间及时关闭（GOSpeak 不会自动关闭空房间，需前端在最后一人离开时删除房间，或配置定时清理）
- 不启用视频，音频成本远低于视频
- 开发测试用自建、生产用 Cloud，或按需组合

## 国内访问情况

LiveKit Cloud 的节点主要在境外。国内用户连接可能出现：

- 延迟偏高（100ms+）
- 偶发丢包导致声音断续
- 极端情况下无法连接

用户主要在国内时，自建一台国内服务器通常体验更好。

## 迁移

Cloud 与自建互切只需修改三个环境变量：

```ini
LIVEKIT_HOST=...
LIVEKIT_KEY=...
LIVEKIT_SECRET=...
```

重启后生效。已建立的连接会中断，用户重新进入频道即可。

## 常见问题

**连接超时**
检查 `LIVEKIT_HOST` 是否以 `wss://` 开头，注意协议不可省略。

**提示 token 无效**
Key 与 Secret 不配对。在 Cloud 控制台重新复制，注意 Secret 不要混入空格。

**能连接但无声音**
Cloud 一般不存在媒体端口问题。查看浏览器控制台，可能为麦克风权限未授予。

**费用增长超出预期**
在 Cloud 控制台的用量页面定位消耗来源，通常为未关闭的空房间持续占用参与者时长。

## 下一步

- [语音服务](/sfu/) — 整体架构
- [自建 LiveKit](/sfu/selfhost) — 自主可控方案
