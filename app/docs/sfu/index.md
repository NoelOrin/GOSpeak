# 语音服务

GOSpeak 自身不实现音频转发，由 LiveKit 承担。

## 拆分的原因

音频转发是 IO 密集型工作：一人发言，音频需同时推送给房间内其他所有成员。放在业务进程内会拖慢整个应用。

拆分后的职责边界：

- 应用负责签发令牌、管理房间成员、处理业务逻辑
- LiveKit 只负责转发音频
- 两者独立扩容、独立重启

## 数据流向

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
   wss://<你的域名>/livekit/rtc/v1?access_token=...
   （同源反代，应用内部再转到 livekit:7880）
        │
5. 音频在浏览器和 LiveKit 之间直连
   应用完全不参与
```

关键在第三步：**令牌由应用签发，LiveKit 只验签**。应用由此控制谁能进入哪个房间，LiveKit 无需了解业务逻辑。

## 两种用法

| | 自建 | LiveKit Cloud |
|---|---|---|
| 部署 | 自行运行 `livekit-server` | 注册即可使用 |
| 数据 | 数据完全自持 | 经过第三方 |
| 费用 | 服务器成本 | 按用量计费 |
| 运维 | 自行管理升级与扩容 | 免运维 |
| 延迟 | 取决于服务器位置 | 全球边缘节点 |
| NAT 穿透 | 自行配置 TURN | 内置 |

小规模团队自建即可满足需求且成本最低。无运维条件时选用云服务，两者配置仅差三行。

## 配置

两种方式下应用侧使用相同的变量：

```ini
SFU_PROVIDER=livekit
LIVEKIT_HOST=ws://livekit:7880
LIVEKIT_KEY=你的key
LIVEKIT_SECRET=你的secret
# LIVEKIT_PUBLIC_URL 默认留空
```

`LIVEKIT_HOST` 是**后端访问** LiveKit 的地址，走容器内网，与浏览器无关：

| 场景 | `LIVEKIT_HOST` |
|------|---------------|
| 本地开发（LiveKit 在宿主机） | `ws://localhost:7880` |
| Docker Compose 内 | `ws://livekit:7880` |
| LiveKit Cloud | `wss://xxx.livekit.cloud` |

浏览器侧地址由 `LIVEKIT_PUBLIC_URL` 决定，**默认留空**：留空时应用按请求 Host 自动推导出 `wss://<你的域名>/livekit`，信令走应用内置的同源反代，无需为 LiveKit 准备域名或证书。

仅当 LiveKit 使用独立域名对外、或使用 LiveKit Cloud 时，才显式填写 `LIVEKIT_PUBLIC_URL`。

## 应用与 LiveKit 的职责划分

| 事项 | 负责方 |
|------|--------|
| 谁能进入哪个房间 | 应用（签令牌时决定） |
| 房间内有谁 | 两方均有，应用维护自己的状态 |
| 踢人 | 应用调用 LiveKit 管理 API |
| 禁言 | 应用调用 LiveKit 管理 API |
| 音频转发 | LiveKit |
| 谁在说话 | LiveKit 的 active speaker 事件，应用转发给前端 |
| 房间生命周期 | 应用决定，通知 LiveKit |

## 管理后台配置

管理后台「SFU 管理」中可直接修改，保存至数据库后立即生效。

适用于临时切换场景——例如自建服务不可用时切换至云服务应急。数据库配置的优先级高于环境变量。

## 相关接口

| 方法 | 路径 | 权限 | 用途 |
|------|------|------|------|
| POST | `/api/v1/signal/token` | 登录即可 | 获取进房令牌 |
| GET | `/api/v1/signal/rooms` | 登录即可 | 列出当前房间 |
| GET | `/api/v1/signal/participants` | 登录即可 | 列出房间成员 |
| GET | `/api/v1/signal/ws-endpoint` | 登录即可 | 获取信令地址 |
| POST | `/api/v1/signal/webhook` | LiveKit 调用 | 接收房间/成员事件 |
| POST | `/api/v1/sfu/config` | `sfu:manage` | 读取当前配置 |
| POST | `/api/v1/sfu/update-config` | `sfu:manage` | 修改配置并激活 |
| POST | `/api/v1/sfu/switch-provider` | `sfu:manage` | 仅切换 provider，不修改配置 |
| POST | `/api/v1/sfu/providers` | `sfu:manage` | 列出已配置的 provider |

## 下一步

- [自建 LiveKit](/sfu/selfhost) — 使用自有服务器运行
- [LiveKit Cloud](/sfu/cloud) — 使用云服务
- [故障排查](/deploy/troubleshooting) — 语音问题排查
