# LiveKit 云服务

不想自己搭建 LiveKit 服务器？使用 [LiveKit Cloud](https://livekit.io/cloud) 开箱即用，无需运维基础设施。

## 创建 LiveKit Cloud 项目

1. 注册 [LiveKit Cloud](https://livekit.io/cloud)
2. 创建一个新项目
3. 在项目设置中获取：
   - **Server URL**: `wss://<project>.livekit.cloud`
   - **API Key**: 形如 `APIxxxxxxxx`
   - **API Secret**: 对应密钥

## 环境变量配置

```env
SFU_PROVIDER=livekit
LIVEKIT_HOST=wss://your-project.livekit.cloud
LIVEKIT_KEY=APIxxxxxxxxxxxxxxxx
LIVEKIT_SECRET=your-api-secret
```

> 注意：云服务使用 `wss://` 前缀（WebSocket Secure）

## 无需本地 Redis

LiveKit Cloud 自带云 Redis 管理，不需要本地部署 Redis。

## 示例：最小化开发

```env
# 仅后端 + 数据库 + LiveKit Cloud
DB_TYPE="SQLite"
SFU_PROVIDER="livekit"
LIVEKIT_HOST="wss://gamespeak-xxxxxxxx.livekit.cloud"
LIVEKIT_KEY="API7ar5gCTyVkfY"
LIVEKIT_SECRET="xtpW2PGUKPugyVzEvX4OaPaHtNGm5bRUmrIXe6NMIqb"
```

```bash
# 启动 GOSpeak，不依赖任何本地 Docker 服务
pnpm dev:server
pnpm dev:web
```

## 限制

| 对比项 | 自建 | 云服务 |
|--------|------|--------|
| 数据 | 完全自控 | 经过第三方服务器 |
| 费用 | 仅服务器成本 | 按带宽/分钟计费 |
| Custom domain | ✅ | ✅（付费套餐）|
| TURN 中继 | 需自配 | ✅ 内置 |
| Webhook | ✅ | ✅ |

## 生产建议

- 云服务适合**不想运维**的场景，按量计费
- 国内用户注意 LiveKit Cloud 的网络延迟，建议选择最近的区域
- 大规模使用注意带宽费用
