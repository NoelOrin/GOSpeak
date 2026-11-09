# livekit 模块

LiveKit 服务封装，负责与 LiveKit WebRTC 服务通信。

## 文件说明

| 文件 | 职责 |
|------|------|
| client.go | LiveKit 客户端服务：生成 Token、管理房间、查询参与者 |

## 环境变量

| 变量 | 说明 |
|------|------|
| LIVEKIT_HOST | 后端访问 LiveKit 的地址（admin API / RoomService），容器内填 `ws://livekit:7880` |
| LIVEKIT_PUBLIC_URL | 下发给浏览器的连接地址，必须浏览器可达；留空回退到 `LIVEKIT_HOST` |
| LIVEKIT_KEY | API Key |
| LIVEKIT_SECRET | API Secret |

`GetHost()` 返回的是 `LIVEKIT_PUBLIC_URL`（浏览器侧地址），**不是** `LIVEKIT_HOST`。
容器部署时二者不同：浏览器无法解析 `ws://livekit:7880` 这类容器服务名。
