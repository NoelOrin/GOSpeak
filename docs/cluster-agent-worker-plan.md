# GOSpeak Agent + Worker 集群架构（现行版）

> 状态：本文根据当前代码（2026-08-17 核对）重写，取代初版 `cluster-agent-worker-plan.md`。
> 原 `2026-08-02-cluster-control-plane-completion.md` 已完成并清理；控制面（初版 Phase 1–6）已整体落地。
> 重要变更：初版使用 **Guild** 术语，当前代码已统一为 **Domain**（语音域/租户）。本文按实际架构描述。
> **2026-08-17 二次核对结论：本计划涉及的工作（控制面 + 前端发现/邀请/集群管理 + 后端测试）已全部交付，无遗留待实现项。** 第 5 节已改为交付与验证记录。唯一真正未启动的相关计划是独立的 `2026-08-03-tauri-mobile-hybrid-webrtc-plan.md`（移动端）。

## 0. 架构变更摘要（相对初版计划）

- 术语重命名：`Guild` → `Domain`（语音域/租户）；`Server` 现在指「Domain 在某个 Node 上的实例」（`ServerAssignment`）；`Node` 指一个运行中的 GOSpeak 进程。
- 控制面（初版 Phase 1–6）已完整实现：角色化启动、节点注册/心跳/状态机、Server 调度与扩缩容、Agent 主备锁（NATS KV 锁 + DB 写面 fence）、跨实例 NATS 控制命令。
- 发现/邀请（初版 Phase 0.5）已用新形态落地：`domain:*` 权限体系、`/domain/$domainUUID` 页面、`ln_code` 邀请码、`n/d/CODE` 邀请链接、`/ln` 路由。
- 单 Agent 中心节点保持不变；Worker 为数据面节点，承载信令、SFU 与房间运行时。

## 1. 术语与概念映射

| 术语 | 含义（当前代码） |
|---|---|
| Domain | 业务侧语音域/租户，对应初版「Guild」。唯一标识 `domain_uuid`，权限码 `domain:*` |
| Server | 集群侧可调度工作负载 = 一个 Domain 在某 Node 上的实例，由 `ServerAssignment` 表达 |
| Node | 一个运行中的 GOSpeak 进程（agent 或 worker） |
| Agent | 中心控制面，唯一权威写路径；抢主锁后做 seed/bootstrap |
| Worker | 数据面节点，承载信令、SFU 与房间运行时，向 Agent 注册/心跳 |
| All | 单机开发模式，控制面与数据面同进程 |

概念对照（保留初版 Kubernetes 类比）：Center Agent ≈ kube-apiserver + scheduler；Worker Node ≈ k8s Node；Server ≈ Deployment；Server 副本 ≈ Pod；Room ≈ 可调度单元。

## 2. 运行模式（已实现）

同一二进制通过 `GOSPEAK_ROLE` 切换三种模式：`agent` / `worker` / `all`（默认）。相关配置（见 `internal/config/config.go`）：

| 变量 | 默认 | 说明 |
|---|---|---|
| `GOSPEAK_ROLE` | `all` | `agent` / `worker` / `all` |
| `CLUSTER_NODE_ID` | 自动 `node-<instanceID>` | 节点 UUID |
| `CLUSTER_ADVERTISE_URL` | 本地推断 | 其它节点访问本节点的地址 |
| `CLUSTER_AGENT_URL` | — | Worker 连接 Agent 的基址 |
| `CLUSTER_AGENT_TOKEN` | — | Worker→Agent 鉴权 token |
| `CLUSTER_NODE_SECRET` | — | 节点注册防冒领密钥 |
| `CLUSTER_HEARTBEAT_INTERVAL` | `5s` | 心跳周期 |
| `CLUSTER_HEARTBEAT_TIMEOUT` | `30s` | 心跳超时（超时标记 offline） |
| `CLUSTER_MAX_SERVERS` / `CLUSTER_MAX_ROOMS` | `100` / `1000` | 节点容量上限 |
| `CLUSTER_LABELS` | — | `region=cn,pool=voice` 风格标签 |
| `CLUSTER_ENTRY_URL` | — | 入口地址（前端信令路由） |

启动行为（`server/gin.go`、`server/cluster_runtime.go`）：
- `agent`：抢 NATS JetStream KV 主锁（`cluster.OpenLeaderLock` / `TryAcquire`）+ DB `cluster_leader_fences` 写面 fence；赢得后才执行 `bootstrapAgentControlPlane`（seed 角色、admin、默认 Domain、权限、SFU 配置、插件）。
- `worker`：通过 `cluster.AgentClient` 向 Agent 注册并周期心跳，订阅 `cluster.control.<nodeID>` 控制命令；不执行控制面 bootstrap。
- `all`：同进程同时承担控制面与数据面，行为等价于单 Agent + 本地 Worker。

## 3. 已落地控制面（初版 Phase 1–6 完成）

### 节点模型与注册心跳（初版 Phase 2）✅
- 模型：`model.ClusterNode`（状态机 `pending/ready/busy/draining/offline/unhealthy`）、`ServerAssignment`（`server_uuid`, `node_uuid`, `status`）、`ClusterLeaderFence`（`leader_id`, `epoch`），均 AutoMigrate。
- 仓储：`repository.ClusterNodeRepository`、`ServerAssignmentRepository`、`cluster_fence_repo`。
- 接口（`internal/router/routes/cluster/routes.go`，均 `POST`，`PermClusterManage`/`PermClusterRead` 守卫）：
  - `/api/v1/cluster/nodes/register`、`/nodes/heartbeat`、`/nodes/deregister`、`/nodes/drain`、`/nodes/undrain`、`/nodes/list`
  - `/api/v1/cluster/stats`
- 客户端：`internal/cluster/agent_client.go`（`AgentClient`，注册/心跳/注销，含 5xx/429 重试）。
- 事件：`internal/cluster/events.go` 发布 `cluster.node.*`；Agent 侧 `ReapOffline(timeout)` 按时心跳超时标记 offline。

### Server 调度与扩缩容（初版 Phase 3）✅
- 调度器：`internal/cluster/scheduler.go`（`NodeRequirement.Matches`、`CanSchedule`、`NodeScore`、`ChooseNodesWithRequirement`，按容量/标签/负载/分散度选节点）。
- 服务：`internal/service/cluster_scaling.go`（`ScaleServer`、`ScaleServerWithRequirement`、`AutoScale`）、`cluster_service.go`（`ReconcileAll` 对账）。
- 接口：`/api/v1/cluster/servers/scale`、`/servers/resolve`（返回房间所在节点地址）、`/servers/list`、`/servers/drain`、`/servers/autoscale`。

### 控制命令与一致性（初版 Phase 4）✅
- 命令类型（`internal/cluster/control.go` 的 `ControlCommand`，经 NATS internal 下发，目标节点 `cluster.control.<nodeID>`）：`kick`、`mute`、`unmute`、`delete_room`、`delete_server`、`kick_domain`、`drain_node`；`Validate()` 校验必填字段（如 `delete_room` 需 `domain_uuid`+`room`，`kick_domain` 需 `payload.user_uuid`）。
- 写面隔离：用户数据写路径仅 Agent 注册；Worker 不直接写权威数据（由 `cfg.ClusterRole == Worker` 在 repo/作业层隔离）。

### 部署、前端与监控（初版 Phase 5）⚠️ 后端完成 / 前端管理 UI 待补
- `deploy/docker-compose.yml` 已配置 `agent` 与 `worker` 服务（均设置 `GOSPEAK_ROLE`）。
- 后端管理/统计 API（`/cluster/nodes/list`、`/cluster/stats`、`/cluster/servers/*`）已就绪。
- 缺口：集群管理前端页面（节点列表、Server 实例组、扩缩容操作 UI）尚未补齐。

### 高可用与弹性（初版 Phase 6）✅ 主体完成
- 主备：`NATS` KV 主锁 + `ClusterLeaderFence`（epoch 递增，旧 leader 写请求被拒，避免网络分区双写）。
- 一致性：`Worker` 只读副本（`DB_READ_DSN`/`DB_READ_*` + `transaction_read_only`）、replica lag 指标与 `DB_REPLICA_LAG_THRESHOLD` 降级。
- 控制命令全节点广播 + 定向投递；draining 时迁移 Server 副本并通知旧节点客户端重连；节点注册绑定 `CLUSTER_NODE_SECRET` 防冒领。
- 待做：按负载自动扩缩容阈值调优、滚动升级与灰度下线。

## 4. Domain（原 Guild）正确性 — 已完成基线

- 权限：`internal/permcode/domain_permcode.go` 定义 `domain:create/read/manage/delete/invite/kick/role:manage`。
- 成员校验：`middleware.RequireDomainMember` + `domainPermissionGranted`；房间创建必须带 `domain_uuid`，且请求上下文的 `domain_uuid` 与房间归属一致（见 `handler/room_handler.go`）。
- 默认域：Agent 赢得主锁后 `repository.EnsureDefaultDomain` + `SeedDefaultDomainRoles`。
- 路由：当前 `AGENTS.md` 路由表已包含 `domain/create/get/list/list-public/my-domains/update/delete/join/preview/leave/kick/members` 等接口，原 Guild 控制面链路已接通。

## 5. 交付状态与验证记录（2026-08-17 二次核对）

二次核对发现：第 5 节原列的「待完成」项在代码中均已实现，故本节改为交付证据，避免重复建设。

### A. 前端发现与邀请 —— 已交付
- 公开域浏览页 `app/web/src/pages/(app)/discover/index.tsx`：`listPublicDomains(page, pageSize, keyword)` 搜索 + 分页（`PAGE_SIZE=12`、`totalPages`）、剪贴板自动识别邀请码/链接（`onMount` 读 `navigator.clipboard`）、「已加入」状态跳转（`isJoined` → 跳 `/domain/$domainUUID`）。
- 邀请确认页 `app/web/src/pages/(app)/invite/d/$code/index.tsx` + `components/domain/DomainInvitePreview.tsx`：预览 + 确认加入 + 已加入态 + 未登录回跳。
- 分享 `components/domain/InviteShareModal.tsx`：`qrcode` 生成二维码 + 复制邀请链接。
- 侧边栏旧「加入服务器」入口已移除（全仓 grep 无命中）。

### B. 控制面前端管理 —— 已交付
- 集群管理页 `app/web/src/pages/(app)/manage/cluster/index.tsx`（393 行）：节点概览/列表、`drain`/`undrain`、`scale`/`autoscale`/`drain` 副本，全部接 `/api/v1/cluster/*`。
- 配套组件与测试：`DomainInvitePreview.spec.tsx`、`DomainMemberTable.spec.tsx`、`DomainRoomTable.spec.tsx`、`DomainIcon.spec.tsx`。

### C. 验收与测试 —— 已交付
- `internal/cluster/`：`leader_test.go`、`control_test.go`、`scheduler_test.go`。
- `internal/service/cluster_service_test.go`：`ReapOffline` / `ReconcileAll` 回归。
- `internal/repository/cluster_fence_repo_test.go`：主备 fence 双写防护。
- `internal/signal/`：`hub_control_test.go`（控制命令 / `OnDomainDelete`）、`hub_domain_test`、`hub_integration_test` 等 20 个测试文件。

### 可选深度打磨（非阻塞，非计划必须项）
- 真实多节点 E2E：当前为单元测试覆盖，缺少「2 Agent + N Worker + 跨实例房间迁移」的端到端验证（可考虑 `test/` 下 Node 集成测试扩展）。
- 自动扩缩容阈值调优（`AutoScale` 已落地，触发策略可结合负载指标细化）。
- 滚动升级与节点灰度下线（后端/部署层扩展）。

### 真正未启动的相关计划
- `docs/superpowers/plans/2026-08-03-tauri-mobile-hybrid-webrtc-plan.md`（🔴 未启动）：Tauri 移动端混合 WebRTC，与本计划正交，仓库内暂无相关代码。

## 6. 风险与依赖（更新）

| 风险 | 说明 | 缓解 |
|---|---|---|
| SQLite 限制 | SQLite 无法多节点共享权威库 | 生产必须 PostgreSQL；SQLite 仅 `all` 开发模式 |
| 单 Agent 单点 | 用户明确单中心节点 | 权威数据落 DB + NATS KV 主锁，重启可恢复；HA 已由 Phase 6 覆盖 |
| 前端信令路由 | 需按 Server/房间连接目标节点 | Agent 经 `/servers/resolve` 下发节点地址，前端按 `CLUSTER_ENTRY_URL` 处理重连 |
| Worker 写面禁用 | 同二进制需路由层模式开关 | `cfg.ClusterRole` 在 repo/作业层隔离，测试覆盖 403 路径 |

## 7. 建议执行顺序

1. 补齐前端发现/邀请体验（A 组）—— 后端接口已具备，主要是 UI 接线。
2. 补齐集群管理前端页（B 组）—— 复用已有 `/cluster/*` API。
3. 调优自动扩缩容与灰度下线，补跨实例集成测试（C 组）。
