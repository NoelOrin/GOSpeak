# 自托管 SFU Provider 选型分析

**评估日期**: 2026-07-05
**目标**: 评估可自托管的 SFU（Selective Forwarding Unit）方案，作为 GOSpeak 的 `sfu.Provider` 后端

---

## 1. 现有 Provider 概览

项目当前 5 个 Provider，自托管支持情况：

| Provider | 自托管 | 维护状态 | 部署模型 | API 类型 | Provider 完整度 |
|----------|--------|----------|----------|----------|----------------|
| LiveKit | ✅ | 活跃 | Docker | gRPC + REST | 9/9 ✅ |
| SRS | ✅ | 活跃 | Docker | HTTP WHIP/WHEP | 6/9 |
| MediaSoup | ✅ | 活跃 | Node.js 进程 | 自定义 bridge | 5/9 |
| Agora | ❌ Cloud-only | 活跃 | N/A | REST | 6/9 |
| Daily | ❌ Cloud-only | 活跃 | N/A | REST | 5/9 |

---

## 2. 候选自托管 SFU 调研

### 2.1 LiveKit（已集成）

| 项目 | |
|------|--|
| 仓库 | github.com/livekit/livekit |
| 语言 | Go |
| License | Apache 2.0 |
| 推荐度 | ★★★★★ |

**部署**: Docker compose（livekit-server + redis），端口 7880(REST) / 7881(gRPC) / 5349(TURN)

**API 完整度**: 自托管 SFU 中唯一完整实现 Provider 接口全部 9 个方法的方案。
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
- 资源消耗相对较大（依赖 Redis）
- 架构较重，小规模部署有 overhead

**Provider 实现**: LiveKit SDK 直接调用 gRPC 接口

```go
// internal/sfu/providers/livekit/ — 唯一完整实现
client.GenerateToken(room, identity)      // ✅
client.ListRooms()                         // ✅
client.ListParticipants(room)             // ✅
client.MuteParticipant(room, id, sid, b)  // ✅
```

---

### 2.2 SRS（已集成）

| 项目 | |
|------|--|
| 仓库 | github.com/ossrs/srs |
| 语言 | C++ |
| License | MIT |
| 推荐度 | ★★★★☆ |

**部署**: Docker compose，HTTP API 端口 1985，RTC 端口 8000(udp)

**特点**: WHIP（推流）+ WHEP（拉流）标准协议，HTTP API 管理

**Provider 覆盖**:

| 方法 | 状态 | 备注 |
|------|------|------|
| `GenerateToken` | ✅ | Vhost 级别鉴权 |
| `GenerateAdminToken` | ✅ | API Key 直接验证 |
| `ListRooms` | ✅ | GET /api/v1/streams |
| `ListParticipants` | ❌ | WHIP 无参与者概念 |
| `MuteParticipant` | ❌ | 无轨道静音 API |
| `MuteRoomParticipant` | ❌ | 同上 |
| `RemoveParticipant` | ✅ | Kick API |
| `DeleteRoom` | ✅ | DELETE stream |
| `GetHost` | ✅ | 配置返回 |

**优点**:
- WHIP/WHEP 是 IETF 标准协议，兼容性好
- HTTP API 简单直接，对接成本低
- 自带 WebRTC 能力（内嵌，无需额外 TURN）
- 可做直播源中转，场景灵活
- 自部署 e2e 已验证(docker compose + WHIP/WHEP 双向音频通，见 `srs-selfhost-runbook.md`)

**缺点**:
- C++ 编译部署，定制门槛高
- 无参与者粒度控制（WHIP 是一对一流）
- 不支持 track 级别静音
- e2e 验证依赖 docker compose，暂无独立 runbook

---

### 2.3 MediaSoup（已集成）

| 项目 | |
|------|--|
| 仓库 | github.com/versatica/mediasoup |
| 语言 | C++（核心）+ Node.js（API） |
| License | ISC |
| 推荐度 | ★★★☆☆ |

**部署**: mediasoup-worker 子进程 + Node.js 服务桥接

**特点**: 底层媒体库，非开箱即用 SFU 服务，需自行搭建信令桥

**Provider 覆盖**:

| 方法 | 状态 | 说明 |
|------|------|------|
| `GenerateToken` | ✅ | 由中介 `mediasoup-worker` 自定义实现 |
| `ListRooms` | ✅ | 由 GOSpeak 服务自行维护 |
| `ListParticipants` | ❌ | bridge 无参与列表端点 |
| `MuteParticipant/MuteRoomParticipant` | ❌ | `notSupportedErr()` |
| `RemoveParticipant` | ❌ | `notSupportedErr()` |
| `DeleteRoom` | ✅ | 关闭 Router |

**优点**:
- 底层能力极强：Simulcast、SVC、PipeTransport
- 模块化，可深度定制
- 纯 C++ 核心，性能上限高

**缺点**:
- 非 SFU 服务，是媒体引擎——需自建完整信令、房间、控制层
- 必须有 mediasoup-worker 外部进程
- 项目已有 mediator bridge（`mediasoup-worker/` + `packages/mediasoup-worker/`），但维护成本高
- 无明显优势替代 LiveKit/SRS

---

### 2.4 Galene（新增候选）

| 项目 | |
|------|--|
| 仓库 | github.com/jech/galene |
| 语言 | **Go** (pion) |
| License | MIT |
| 推荐度 | ★★★☆☆ |

**仓库概况**:
- Description: "Galene is a selective forwarding unit (SFU)"
- 目标: easy-to-host video conferencing 系统
- 自部署: 极简，`CGO_ENABLED=0 go build` 单二进制

**核心架构**:

```
┌─────────────────┐
│   Galene Binary  │
│  ├── HTTP API    │  /galene-api/v0/ (REST management)
│  ├── WebSocket   │  /ws (client signaling + control)
│  ├── Web UI      │  Built-in client interface
│  └── SFU Core    │  Pion-based media relay
└─────────────────┘
```

**HTTP REST API** (`/galene-api/v0/`):

| 端点 | 方法 | 操作 |
|------|------|------|
| `.groups/` | GET | 列出所有房间(group) |
| `.groups/{name}` | GET/PUT/DELETE | 房间 CRUD（乐观锁 if-match） |
| `.groups/{g}/.users/{u}/.tokens/` | POST/GET | 有状态 Token 管理 |
| `.groups/{g}/.keys` | PUT | 无状态 JWK Set 上传 |
| `.stats` | GET | 服务器级别统计 |

**WebSocket 控制协议**（bot 操作员模式）:

| 消息 | 操作 | 条件 |
|------|------|------|
| `useraction: kick` | 踢出参与者 | 需 `op` 权限 |
| `useraction: mute` | 静音参与者 | 可静音不可取消静音 |
| `user` 事件 | 监听参与者上线/离线 | 需加入房间后接收 |

**Provider 接口覆盖度**:

| 方法 | 状态 | 实现方式 | 缺口严重度 |
|------|------|----------|-----------|
| `GenerateToken` | ✅ | POST `.tokens/` | 无 |
| `GenerateAdminToken` | ✅ | JWK 或管理员 API key | 无 |
| `ListRooms` | ✅ | GET `.groups/` | 无 |
| `ListParticipants` | ⚠️ 需 WS bot | 加入房间后监听 `user` 事件，本地维护状态缓存 | 中 |
| `MuteParticipant` | ❌ 半残 | WS 可静音但不可取消静音；无 trackSid | 高 |
| `MuteRoomParticipant` | ❌ 半残 | 同上 | 高 |
| `RemoveParticipant` | ✅ | WS `useraction: kick`（需 WS bot） | 中（需 bot） |
| `DeleteRoom` | ✅ | DELETE `.groups/{name}` | 无 |
| `GetHost` | ✅ | 配置返回 | 无 |

**集成成本**:

```
┌──────────────────────────────────────┐
│ GaleneProvider                       │
│  ├─ HTTP client → REST API          │  (房间+Token)
│  ├─ WS client  → bot 操作员连接      │  (参与者列表+Kick)
│  ├─ 本地参与者缓存 (sync.Map)         │  (ListParticipants)
│  └─ Mute → ErrNotSupported / no-op  │  (静音半残)
└──────────────────────────────────────┘
```

需要常驻一个 WebSocket "bot" 连接作为管理通道——增加故障处理逻辑（断线重连、状态同步）。

**与 LiveKit/SRS 对比**:

| 维度 | Galene | LiveKit | SRS |
|------|--------|---------|-----|
| 部署 | 单二进，极简 | Docker + Redis | Docker |
| REST API | 只有管理 API | 完整 REST + gRPC | HTTP API |
| Token 体系 | 共享密钥 token | JWT token | API Key |
| 参与者控制 | 需 WS bot | 原生 API | Kick only |
| Mute | 半残 | Full track-level | N/A |
| 水平扩展 | 无 | Redis 集群 | 单进程 |
| Go 集成 | HTTP + WS 对接 | gRPC 嵌入 | HTTP |

**结论**: 可用，但 mute 半残和 WS bot 模式增加复杂性。轻量场景可考虑。

---

### 2.5 Ion SFU（新增候选）

| 项目 | |
|------|--|
| 仓库 | github.com/ionorg/ion-sfu（原 pion/ion-sfu） |
| 语言 | **Go** (pion) |
| License | MIT |
| Stars | ~2.3k |
| **最后 commit** | **2021-11-30（停止维护）** |
| 推荐度 | ★★☆☆☆ |

**核心发现**: 项目已停止维护约 5 年，这是一个硬伤。

**架构**:

```
┌─────────────────┐
│    Ion SFU       │
│  ├─ JSON-RPC WS  │  客户端信令
│  ├─ gRPC         │  服务间通信
│  └─ Pion Core    │  WebRTC 媒体转发
└─────────────────┘
```

**关键限制**:
- **无 HTTP REST API**：房间/参与者管理必须通过 Go 内存对象操作
- **无 Token 体系**：需自行实现 JWT 验证
- **无管理 API**：无法从外部运维

**集成路径**:

唯一可行路径是**嵌入模式**——将 ion-sfu 作为 Go 包直接 import 到 GOSpeak 进程中：

```go
import "github.com/pion/ion-sfu/pkg/sfu"

// 在 GOSpeak 进程内启动 ion-sfu
s := sfu.NewSFU(config)
session := s.NewSession(sfu.SessionConfig{...})
```

这样可以直接操作内存对象访问房间、参与者、Track。

**Provider 接口覆盖**（嵌入模式）:

| 方法 | 状态 | 实现方式 |
|------|------|----------|
| `GenerateToken` | ❌ 需自建 | 无 token 体系，需自定义 JWT + 中间件 |
| `ListRooms` | ⚠️ | 访问 `SFU.sessions` map |
| `ListParticipants` | ⚠️ | 通过 `SessionLocal.Peers()` |
| `MuteParticipant` | ⚠️ | `DownTrack.Mute(bool)` |
| `RemoveParticipant` | ✅ | `SessionLocal.RemovePeer()` + `Peer.Close()` |
| `DeleteRoom` | ✅ | 关闭所有 peer，session 自动清理 |

**评价**:
- 停止维护是致命问题——5 年无更新，WebRTC 协议和浏览器兼容性持续演进，用停止维护的 SFU 有风险
- 嵌入模式虽然理论可行，但需要对 ion-sfu 内部 API 有深入理解
- 如果 fork 维护，成本高于直接集成 LiveKit 或 SRS
- **不推荐用于生产环境**

---

### 2.6 Janus（参考）

| 项目 | |
|------|--|
| 仓库 | github.com/meetecho/janus-gateway |
| 语言 | C |
| License | GPLv3 |
| 推荐度 | ★★☆☆☆ |

**说明**: 老牌 WebRTC 网关，插件架构功能极全，但：
- C 语言，部署复杂（需编译，依赖多）
- GPLv3 许可证与项目 License 兼容性待确认
- 架构重，配置管理复杂
- Provider 方式集成需通过 HTTP API + Admin API

---

### 2.7 Jitsi Videobridge（参考）

| 项目 | |
|------|--|
| 仓库 | github.com/jitsi/jitsi-videobridge |
| 语言 | Java |
| License | Apache 2.0 |
| 推荐度 | ★☆☆☆☆ |

**说明**: 成熟，功能全，但：
- Java 栈，部署沉重（JVM 调优）
- 需配套组件（Prosody XMPP, jicofo, jibri）
- 与 Go 技术栈差异极大

---

## 3. 对比矩阵

### 3.1 Provider 接口覆盖度

| Provider | Token | ListRooms | ListParts | Mute | MuteRoom | Kick | DeleteRoom | 自托管 | 维护 | 语言 |
|----------|-------|-----------|-----------|------|----------|------|------------|--------|------|------|
| **LiveKit** | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ 活跃 | Go |
| **SRS** | ✅ | ✅ | ❌ | ❌ | ❌ | ✅ | ✅ | ✅ | ✅ 活跃 | C++ |
| MediaSoup | ✅ | ✅ | ❌ | ❌ | ❌ | ❌ | ✅ | ✅ | ✅ 活跃 | C++/Node |
| Galene | ✅ | ✅ | ⚠️ WS | ❌ | ❌ | ✅ WS | ✅ | ✅ | ✅ 活跃 | Go |
| Ion SFU | ❌ | ⚠️ | ⚠️ | ⚠️ | ⚠️ | ✅ | ✅ | ✅ | ❌ 停维 | Go |
| Agora | ✅ | ✅ | ✅ | ❌ | ❌ | ❌ | ✅ | ❌ | ✅ 活跃 | REST |
| Daily | ⚠️ | ✅ | ✅ | ❌ | ❌ | ❌ | ✅ | ❌ | ✅ 活跃 | REST |

### 3.2 Go 技术栈契合度

| Provider | Provider 是否 Go 原生 | 部署组件 | 对接方式 | 技术契合度 |
|----------|----------------------|----------|----------|-----------|
| **LiveKit** | ✅ | Go Server | gRPC Go SDK | ★★★★★ |
| **SRS** | ❌ C++ | Docker | HTTP | ★★★☆☆ |
| MediaSoup | ❌ C++/Node | Docker + Node | HTTP Bridge | ★★☆☆☆ |
| **Galene** | ✅ Go (pion-based) | 单二进 | HTTP + WS | ★★★★☆ |
| Ion SFU | ✅ Go (pion-based) | 单二进 / 嵌入 | Go 包 import | ★★★☆☆(停维) |

### 3.3 部署复杂度

| Provider | 二进制 | 依赖 | 配置复杂度 | 水平扩展 |
|----------|--------|------|-----------|---------|
| Galene | 单二进 ~10MB | 无 | 低（JSON 文件） | ❌ |
| Ion SFU | 单二进 ~15MB | 无 | 低（TOML） | ❌ |
| SRS | Docker 200MB+ | 无 | 中 | ❌ |
| LiveKit | Docker 100MB+ | Redis | 中 | ✅ |
| MediaSoup | Docker + Node | Redis(可选) | 高 | ❌ |

---

## 4. 推荐方案

### 推荐优先级

```
                        生产首选
                     ┌──────────┐
                     │  LiveKit  │  ← 9/9 Provider 覆盖，Go 栈，集群支持
                     └─────┬────┘
                           │
            ┌──────────────┼──────────────┐
            │              │              │
        ┌───┴───┐    ┌────┴────┐    ┌────┴───┐
        │  SRS   │    │ Galene  │    │ Mediasoup│
        └───┬───┘    └────┬────┘    └────┬───┘
            │              │              │
        轻量生产/        极轻量/         深度定制/
        直播场景        教育场景         研究场景
            │              │              │
        ┌───┴───┐    ┌────┴────┐
        │ Ion   │    │  Janus  │
        └───────┘    └─────────┘
         (停维不推)    (GPLv3 重)
```

### 4.1 场景推荐表

| 场景 | 首要推荐 | 备选 | 理由 |
|------|---------|------|------|
| 生产上线 | **LiveKit** | SRS | 功能完整、集群支持、Go 栈 |
| 轻量自托管 | **SRS** | Galene | 单 docker、HTTP API 够用 |
| 极简单机 | **Galene** | — | 单二进 10MB，零依赖 |
| 与研究 | **MediaSoup** | — | 底层能力强、灵活定制 |
| ~~纯 Go 嵌入~~ | — | — | Ion 停维，无合格替代 |

### 4.2 详细建议

**生产环境 → LiveKit**
- Provider 接口 9/9 完整
- gRPC + Go SDK，性能最佳
- 集群水平扩展（redis-based）
- Egress/Ingress/Webhook 生态完善
- 需运行 Redis + LiveKit Server

**轻量/直播 → SRS**
- WHIP/WHEP 标准协议
- HTTP API 简单可靠
- 不支持 mute/participant 列表，场景受限
- 适合直播 + 简单通话

**极轻量/教育 → Galene**
- 单 Go 二进，部署最简单
- 可作为 LiveKit 降级选项
- Mute 半残（可静音不可取消），需评估业务是否需要远程取消静音
- 无水平扩展

### 4.3 不建议用于生产

| Provider | 原因 |
|----------|------|
| **Ion SFU** | 2021 年停维，5 年无更新 |
| **Janus** | GPLv3 License 不兼容，C 部署重 |
| **Jitsi** | Java 栈，太重，技术栈差异大 |

---

## 5. 未来新增 Provider 的建议

如果未来要新增自托管 Provider 到 `sfu/factory.go`，推荐检查清单：

1. **REST/gRPC API 完整度**：是否有房间 CRUD、参与者列表、Kick API
2. **Token 体系**：是否支持服务端生成加入 Token（或 API Key 鉴权）
3. **Go 语言/生态**：是否有 Go SDK，或至少 HTTP 接口
4. **维护活跃度**：仓库最近 1 年是否有 commit，issue 是否有人维护
5. **自部署复杂度**：是否需要额外依赖（数据库、消息队列等）
6. **Mute 能力**：是否支持服务端轨道级静音/取消静音（根据业务需求）

---

**参考文档**:
- `sfu-provider-maturity.md` - 现有 Provider 实现完整度矩阵
- `srs-selfhost-runbook.md` - SRS 自部署 e2e 验证记录
- `deployment-guide.md` - 环境变量配置
- `AGENTS.md` - Provider 接口定义和路由表
