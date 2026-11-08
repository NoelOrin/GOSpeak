# 语音服务

GOSpeak 自己不做音频转发，交给 LiveKit 干。

## 为什么拆出去

音频转发是 IO 密集型工作：一个人说话，要把音频同时推给房间里其他所有人。放在业务进程里会让整个应用被拖慢。

拆出去之后：

- 应用只负责签发令牌、管房间成员、处理业务逻辑
- LiveKit 只负责转发音频
- 两者独立扩容、独立重启

## 数据是怎么流的

```
1. 用户进频道
        │
2. 前端问应用要令牌
   POST /api/v1/signal/token
        │
3. 应用用 LIVEKIT_SECRET 签一个 JWT
   里面写清楚：能进哪个房间、身份是谁
        │
4. 前端拿令牌连 LiveKit
   wss://livekit/rtc?access_token=...
        │
5. 音频在浏览器和 LiveKit 之间直连
   应用完全不参与
```

关键在第三步：**令牌由应用签发，LiveKit 只验签**。这样应用控制谁能进哪个房间，LiveKit 不用知道业务逻辑。

## 两种用法

| | 自建 | LiveKit Cloud |
|---|---|---|
| 部署 | 自己跑 `livekit-server` | 注册就能用 |
| 数据 | 全在你手里 | 经过第三方 |
| 费用 | 服务器成本 | 按用量计费 |
| 运维 | 自己管升级、扩容 | 不用管 |
| 延迟 | 看服务器位置 | 全球边缘节点 |
| NAT 穿透 | 自己配 TURN | 内置 |

小团队自建够用，也最省钱。不想运维就用云服务，配置只差三行。

## 配置

不管哪种，应用侧就这四个变量：

```ini
SFU_PROVIDER=livekit
LIVEKIT_HOST=ws://localhost:7880
LIVEKIT_KEY=你的key
LIVEKIT_SECRET=你的secret
```

`LIVEKIT_HOST` 的写法：

| 场景 | 写法 |
|------|------|
| 本地开发 | `ws://localhost:7880` |
| Docker Compose 内 | `ws://livekit:7880` |
| 生产（HTTPS） | `wss://voice.example.com` |
| LiveKit Cloud | `wss://xxx.livekit.cloud` |

**页面是 HTTPS 的时候，这里必须是 `wss://`**，否则浏览器会拦掉。

## 应用管什么，LiveKit 管什么

| 事情 | 谁做 |
|------|------|
| 谁能进哪个房间 | 应用（签令牌时决定） |
| 房间里有谁 | 两边都有，应用存自己的状态 |
| 踢人 | 应用调 LiveKit 的管理 API |
| 禁言 | 应用调 LiveKit 的管理 API |
| 音频转发 | LiveKit |
| 谁在说话 | LiveKit 的 active speaker 事件，应用转发给前端 |
| 房间生命周期 | 应用决定，通知 LiveKit |

## 在后台改配置

管理后台「SFU 管理」里能直接改，改完存数据库，立即生效。

这适合临时切换——比如自建挂了，先切到云服务应急。数据库里的配置优先级高于环境变量。

## 相关接口

| 方法 | 路径 | 权限 | 用途 |
|------|------|------|------|
| POST | `/api/v1/signal/token` | 登录即可 | 拿进房令牌 |
| GET | `/api/v1/signal/rooms` | 登录即可 | 列当前房间 |
| GET | `/api/v1/signal/participants` | 登录即可 | 列房间成员 |
| GET | `/api/v1/signal/ws-endpoint` | 登录即可 | 拿信令地址 |
| POST | `/api/v1/signal/webhook` | LiveKit 调用 | 接收房间/成员事件 |
| POST | `/api/v1/sfu/config` | `sfu:manage` | 读当前配置 |
| POST | `/api/v1/sfu/update-config` | `sfu:manage` | 改配置并激活 |
| POST | `/api/v1/sfu/switch-provider` | `sfu:manage` | 只切 provider，不改配置 |
| POST | `/api/v1/sfu/providers` | `sfu:manage` | 列出已配置的 provider |

## 下一步

- [自建 LiveKit](/sfu/selfhost) — 自己的服务器跑
- [LiveKit Cloud](/sfu/cloud) — 用云服务
- [故障排查](/deploy/troubleshooting) — 语音不通怎么查
