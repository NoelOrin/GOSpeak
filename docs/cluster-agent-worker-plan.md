# GOSpeak 中心 Agent + Worker 架构实施计划

> 当前状态：Phase 0-5 控制面主体已实现（角色化启动、节点注册/心跳、Server 调度、NATS 控制命令、Worker 只读 DB、前端 workerUrl 路由、集群管理页、Nginx 集群路由、健康统计）；Phase 6 已提供主备锁、DB 写面 fence、自动扩缩/灰度扩展点。本文中未勾选的 checkbox 表示仍待验收或扩展项，不代表整体控制面未实现。
> 实现说明：详细实现与验收计划已由本文档承载；原 `2026-08-02-cluster-control-plane-completion.md` 已完成并清理。

## 1. 背景

GOSpeak 当前是单进程/对等实例架构：多个后端实例通过 NATS/Redis 共享运行时状态，没有中心控制节点。Guild（多 Server）功能已有数据模型、路由、信令隔离雏形，但权限种子、房间归属、前端 guild 上下文、删除清理等链路未接通，实际不可用。

同时，当前 Guild 入口只支持侧边栏输入邀请码，缺少公开发现页、邀请链接、二维码和统一的加入确认流程。本计划在集群化改造之外，一并补齐 Guild 发现与邀请链路。

## 2. 已确认架构决策

| 决策 | 内容 |
|---|---|
| 中心节点 | 只有一个 Center Agent 节点，承担集群调度 + Guild 业务控制面 |
| Worker | 完整 GOSpeak 后端实例，拥有信令、SFU、房间运行时能力 |
| 节点与 Server | 一个节点可托管多个 Server；一个 Server 可跨多个节点多副本扩容 |
| 无租户设计 | 用户、权限、全局数据为单租户统一模型；Server 只是可调度工作负载 |
| 一致性 | 所有最终用户数据只由 Agent 节点保证一致性，Worker 只读缓存 + 运行时快照 |
| 复用 | 复用现有 NATS/Redis Bus、signal.Hub、SFU Provider 抽象，不重写 |

术语对照：

| 术语 | 含义 |
|---|---|
| Guild | 业务侧语音服务器，即现有数据模型 |
| Server | 集群侧可调度工作负载，对应 Guild 的部署实例组 |
| Agent | 中心控制面，负责权威数据、调度和业务写路径 |
| Worker | 数据面节点，承载信令、SFU 与房间运行时 |

概念映射：

- Center Agent = kube-apiserver + scheduler + controller manager
- Worker 节点 = k8s Node / kubelet
- Server = Deployment（普通工作负载，不使用 Namespace/租户）
- Server 副本 = Pod
- Room = 可调度单元，在 Server 副本内执行

## 3. 总体目标

1. 同一二进制支持 `agent`、`worker`、`all`（单机开发）三种运行模式。
2. Agent 是唯一权威写路径，Worker 不直接写用户数据。
3. Server 创建后可由 Agent 调度到 1..N 个节点，节点可承载多个 Server。
4. 前端信令按“Server → 房间所在副本”路由到目标节点。
5. 节点注册、心跳、状态、扩缩容、驱逐、恢复全部由 Agent 控制面管理。
6. 补齐 Guild 发现、邀请链接与二维码入口：公开 Guild 可浏览、预览和加入，私有 Guild 仍走邀请码。

## 4. 阶段计划

### 阶段总览

| 阶段 | 主题 | 核心交付 |
|---|---|---|
| Phase 0 | 控制面正确性前置 | 修复 Guild 权限、房间归属、成员校验和删除清理 |
| Phase 0.5 | Guild 发现与邀请链接 | 发现页、公开列表、邀请预览、链接/二维码、剪贴板识别 |
| Phase 1 | 角色化启动 | `agent / worker / all` 模式与写面隔离 |
| Phase 2 | 节点模型与注册心跳 | 节点注册、心跳、状态机 |
| Phase 3 | Server 实例组与调度 | 多副本、房间调度、`workerUrl` |
| Phase 4 | 控制命令与一致性 | Agent 独占写、NATS 下发、对账恢复 |
| Phase 5 | 部署、前端与监控 | docker-compose、管理页、健康指标 |
| Phase 6 | 高可用与弹性 | Agent 主备、自动扩缩、灰度 |

### Phase 0：控制面正确性前置

目标：修复现有 Guild 断链，让 Server 管理先可用，为集群化提供正确的控制面数据。

涉及模块：`model`、`repository`、`service`、`handler`、`middleware`、`signal`。

任务：

- 权限与角色
  - [ ] 将 `guild:*` 权限码加入 `model.DefaultPermissions`、`DefaultRolePermissions`
  - [ ] 同步前端 `rolePermissions`
- 房间归属与隔离
  - [ ] `CreateRoomRequest` 增加 `guild_uuid`，`RoomService.CreateRoom` 持久化归属
  - [ ] `RoomInfo` 增加 `guild_uuid`，修复 `getMergedRooms` 按逻辑名合并导致的跨 Server 串名
- 中间件与权限校验
  - [ ] 在 `server/gin.go` 注入 `SetGuildChecker(guildSvc.IsMember)`
  - [ ] Guild 资源路由挂载 `RequireGuildMember`
- 删除与默认服务器
  - [ ] Guild 删除时清理 `guild_members`、关联房间，并接入 `signalHub.OnGuildDelete`
  - [ ] 修正 `migrateDefaultGuild` 的 owner/成员缺失问题
- 测试修复
  - [ ] 修复 `internal/signal` 测试编译失败（`message_bridge_test.go` 与 `OnDisconnect` 签名不匹配）

验收：

- 创建、查看、更新、删除、踢人、成员列表接口不再 403
- 房间按 Server 过滤与隔离，同名房间不串
- Guild 删除后成员、房间、信令会话全部清理

### Phase 0.5：Guild 发现与邀请链接

目标：补齐 Guild 邀请链路与发现页，使公开 Server 可以被浏览、预览和加入，为后续 Agent 控制面提供标准入口。

涉及模块：`handler/guild`、`router/guild`、`app/web`（sidebar、discover、invite）。

任务：

- 后端能力
  - [ ] `POST /api/v1/guild/list-public` 支持 `keyword` 搜索与分页，只返回 `is_public=true` 的 Guild
  - [ ] 新增 `POST /api/v1/guild/preview`，凭邀请码返回 Guild 基础信息，加入前展示确认
- 前端发现页
  - [ ] 新增 `/discover`，展示公开 Guild、分页搜索、新建服务器入口
  - [ ] 已加入 Guild 显示“已加入”并允许跳转
  - [ ] 原“新建服务器”按钮改为“发现服务器”并跳转发现页
  - [ ] 删除侧边栏“加入服务器”按钮及逻辑
- 邀请链接与剪贴板
  - [ ] Guild 详情页支持复制邀请码、复制 `/invite/g/{code}` 链接、二维码分享
  - [ ] 新增 `/invite/g/:code` 页面，先展示 Guild 信息再确认加入
  - [ ] 未登录用户跳转登录后回到原邀请页
  - [ ] 发现页加载时自动读取剪贴板，识别 Guild 邀请链接或邀请码并展示预览
  - [ ] 保留手动输入邀请码/邀请链接的入口
- 一致性边界
  - [ ] 邀请生成/预览属于 Agent 控制面读路径
  - [ ] Worker 模式不注册 Guild 写接口
  - [ ] 邀请链接随 Guild 删除失效
- 范围边界
  - [ ] 本次不做房间邀请链接，不引入房间 `invite_links`

验收：

- 发现页只展示公开 Guild，私有 Guild 不出现
- 加入前必须先展示 Guild 信息并确认
- 剪贴板中有 `/invite/g/{code}` 或邀请码时，页面自动识别
- 已加入 Guild 显示“已加入”，点击进入
- 邀请链接可打开、可二维码扫码，未登录用户在登录后回到邀请页
- Worker 模式无法通过业务写接口创建或修改 Guild

### Phase 1：角色化启动

目标：同一进程可切换为控制面或数据面。

涉及模块：`config`、`server/gin.go`、`router`。

任务：

- [ ] 配置新增 `GOSPEAK_ROLE=agent|worker|all`
- [ ] 拆分 `server/gin.go` 初始化：控制面（DB、业务 handler、管理 API）与数据面（signal Hub、SFU、WS）
- [ ] Worker 模式禁用业务写路由（room/guild/message 等写操作返回 403 或不注册）
- [ ] Worker 模式 DB 只读或仅使用缓存，禁止直接写
- [ ] `all` 模式保持现有单机行为，Agent 与 Worker 同进程

验收：

- 同一二进制三种模式均可启动
- Worker 模式无法写入用户数据

### Phase 2：节点模型与注册心跳

目标：Agent 能发现、跟踪、管理所有 Worker 节点。

涉及模块：`internal/cluster`、`agent`、`worker`、NATS。

任务：

- [ ] 新增 `internal/cluster` 包：Node 模型（uuid、host、labels、capabilities、serving_servers、状态）
- [ ] Agent API：`/api/v1/cluster/nodes` 注册、心跳、注销、列表
- [ ] 节点状态机：`pending / ready / busy / draining / offline / unhealthy`
- [ ] Worker 侧 AgentClient：启动注册、周期心跳、上报 rooms/connections/load/SFU 健康
- [ ] NATS 事件：`cluster.node.*` 状态变更
- [ ] DB 表与迁移：`cluster_nodes`
- [ ] 节点超时判定与自动标记 `offline`

验收：

- Agent 管理页/API 能看到节点状态
- 节点断开后按心跳超时标记 `offline`

### Phase 3：Server 实例组与调度

目标：Server 可多副本部署，房间按副本调度，前端信令路由正确。

涉及模块：`internal/cluster`、`signal`、前端 socket 路由。

任务：

- [ ] Server 实例关系表：`server_assignments`（server_uuid、node_id、status）
- [ ] Scheduler：按节点容量、标签、SFU Provider 匹配、负载选择节点；同 Server 副本尽量分散
- [ ] 扩缩容 API：`/api/v1/cluster/servers/:uuid/scale`，支持 `replicas`
- [ ] 房间调度：加入房间时 Agent 返回房间所在节点的 `workerUrl`
- [ ] 前端：guild 页面获取节点地址，socket 连接目标节点；`room:list` 带 `guild_uuid`
- [ ] 缩容/驱逐：节点 `draining` 后停止新调度，迁移房间副本

验收：

- 一个 Server 可运行在多个节点
- 前端加入房间能路由到持有该房间的节点
- 缩容后房间迁移到组内其他副本

### Phase 4：控制命令与一致性

目标：落实“最终用户数据只由 Agent 保证一致性”。

涉及模块：Agent、Worker、NATS、`signalHub`。

任务：

- [ ] Agent 独占写路径：所有用户数据写 API 仅注册在 Agent
- [ ] 变更广播：Agent 写库成功后发布 NATS internal 事件，Worker 刷新只读缓存
- [ ] 控制命令：kick/mute/房间删除/Guild 删除由 Agent 校验后经 NATS 下发目标节点执行
- [ ] `signalHub.OnGuildDelete` 接入 Agent 删除流程
- [ ] 状态对账：Agent 从 NATS KV/Redis 聚合房间与成员快照，与节点心跳对账
- [ ] Agent 重启恢复：启动时从权威 DB 重建节点与调度状态，等待 Worker 重新注册

验收：

- Worker 无法直接写用户数据
- Agent 重启后集群可恢复，用户数据不丢失

### Phase 5：部署、前端与监控

目标：可一键部署并可视化集群。

涉及模块：`deploy`、Nginx、前端管理页。

任务：

- [ ] docker-compose 增加 `agent` 与 `worker` 服务
- [ ] Nginx 按控制面/信令路由；生产环境 DB 使用 PostgreSQL
- [ ] 前端管理页：节点列表、Server 实例组、扩缩容操作
- [ ] 健康检查与指标：节点心跳、rooms、connections、SFU 健康
- [ ] 更新部署文档与 AGENTS.md

验收：

- `docker compose up` 可启动 1 Agent + N Worker + 多 Server 多副本
- 管理页可查看节点与扩缩容

### Phase 6（可选）：高可用与弹性

目标：在单 Agent 稳定运行后，按需提升可用性与扩缩容能力。

任务：

- [ ] Agent 主备切换，保持单一写者
- [x] NATS 主备锁 + DB `cluster_leader_fences` 写面 fence，旧 leader 写请求在接管后直接拒绝
- [x] Worker 只读副本配置（`DB_READ_DSN`/`DB_READ_*`）与会话级 `transaction_read_only`
- [x] replica lag 指标（SSE + Prometheus）与 `DB_REPLICA_LAG_THRESHOLD` 降级标记
- [x] 控制命令全节点广播 + `cluster.control.<nodeID>` 定向投递
- [x] 房间级 `(domain_uuid, room) → Worker` 归属路由
- [x] Agent 主锁抢占后才执行 seed/插件/bootstrap
- [x] draining 时迁移 Server 副本并定向通知旧节点客户端重连
- [x] Worker 注册绑定 `CLUSTER_NODE_SECRET`，禁止冒领已有节点 UUID
- [ ] 按负载自动扩缩容 Server 副本
- [ ] 滚动升级与节点灰度下线

验收：

- 主备切换后仍保持单一写者
- 扩缩容由负载指标驱动且不中断房间
- 灰度下线期间可完成副本迁移

## 5. 风险与依赖

| 风险 | 说明 | 缓解 |
|---|---|---|
| signal Hub 全量本地状态 | 现有 Hub 维护本地房间/成员/stream | 改为“本地运行时 + NATS KV 快照 + Agent 对账” |
| 前端信令路由 | 需要按 Server/房间连接目标节点 | Agent 下发 `workerUrl`，前端处理重连 |
| Worker 写面禁用 | 同一二进制需要路由层模式开关 | Phase 1 先行，测试覆盖 403 路径 |
| SQLite 限制 | SQLite 无法多节点共享权威库 | 生产必须 PostgreSQL，SQLite 仅 `all` 开发模式 |
| 现有测试断裂 | signal 测试当前编译失败 | Phase 0 修复，并补 Guild/集群测试 |
| 邀请码不可单独撤销 | 本期复用 `InviteCode`，无法独立过期/限次 | 后续可引入 `invite_links`；Guild 删除时保证链接失效 |
| 单 Agent 单点 | 用户明确单中心节点 | 权威数据落 DB，重启可恢复；HA 留 Phase 6 |

阶段依赖：Phase 0 → 0.5 → 1 → 2 → 3 → 4 → 5；Phase 6 独立可选。

说明：Phase 0.5 依赖 Phase 0 的 Guild 权限种子与成员校验；Phase 1 角色化后，邀请/发现相关写接口只注册在 Agent。

## 6. 建议执行顺序

1. 先完成 Phase 0，恢复 Guild 基础能力并补齐测试。
2. 完成 Phase 0.5，交付 Guild 发现与邀请入口，确保公开列表和邀请预览由 Agent/控制面读写。
3. Phase 1 角色化改造优先保证 `all` 模式行为不变。
4. Phase 2/3 实现节点注册与 Server 调度，前端配合改动。
5. Phase 4 收紧一致性边界，完成控制命令与对账。
6. Phase 5 交付部署与监控，Phase 6 按需演进。
