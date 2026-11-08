# 自托管 LiveKit 选型

**评估日期**: 2026-07-05
**目标**: 说明 GOSpeak 自托管场景下选择 LiveKit 的原因与部署形态

---

## 1. 选型结论

GOSpeak 的 `sfu.Provider` 后端统一为 **LiveKit**。LiveKit 在自托管 SFU 中提供最完整的 Provider 接口覆盖，且与项目 Go 技术栈一致。

| 项目 | |
|------|--|
| 仓库 | github.com/livekit/livekit |
| 语言 | Go |
| License | Apache 2.0 |
| 推荐度 | ★★★★★ |

---

## 2. LiveKit 自托管

**部署**: Docker Compose（livekit-server），端口 7880(REST/WS) / 7881-7882(WebRTC) / 3478(TURN)

**API 完整度**: 完整实现 Provider 接口全部方法。
- 完整 Token 体系（access token + admin token）
- 房间 CRUD、参与者列表、track 级 Mute/Kick
- Webhook 事件通知
- Egress/Ingress 录制推流

**优点**:
- 最成熟的 Go 语言 SFU，社区活跃
- gRPC 高性能，Go SDK 可直接嵌入
- 支持集群模式（redis-based）
- 功能最全：录制、Webhook、Simulcast、SIP 集成
- 与项目 Go 技术栈完全一致

**缺点**:
- 架构较重，小规模部署有一定 overhead

**Provider 实现**: LiveKit SDK 直接调用 gRPC 接口

```go
// internal/sfu/providers/livekit/ — 唯一实现
client.GenerateToken(room, identity)      // ✅
client.ListRooms()                         // ✅
client.ListParticipants(room)             // ✅
client.MuteParticipant(room, id, sid, b)  // ✅
```

---

## 3. 自建 vs LiveKit Cloud

| 维度 | 自建 | LiveKit Cloud |
|------|------|---------------|
| 数据控制 | 完全自主 | 数据经过第三方 |
| 部署 | Docker Compose | 零运维 |
| 扩展 | 自行配置集群 | 弹性伸缩 |
| 计费 | 服务器成本 | 按用量计费 |
| TURN | 内置（`turn.enabled=true`）| 自带中继 |

---

## 4. 部署检查清单

1. **Token 体系**：`LIVEKIT_KEY` / `LIVEKIT_SECRET` 与 `livekit/livekit.yaml` 的 `keys` 一致
2. **公网地址**：生产环境将 `rtc.use_external_ip` 设为 `true`
3. **UDP 端口**：放行 7881/7882 与 3478/udp
4. **TURN**：自建默认启用，无需额外部署
5. **Webhook**：确认 `webhook.urls` 指向 `/api/v1/signal/webhook`
6. **密钥轮换**：`init.sh` 会同步轮换 `.env` 与 `livekit.yaml` 的密钥

---

**参考文档**:
- [LiveKit 自建](/sfu/livekit-selfhost)
- [LiveKit 云服务](/sfu/livekit-cloud)
- `sfu-provider-maturity.md` - LiveKit Provider 实现完整度
- `deployment-guide.md` - 环境变量配置
