# GOSpeak 中心 Agent + Worker 架构实施计划

## 1. 背景

GOSpeak 当前是单进程/对等实例架构：多个后端实例通过 NATS/Redis 共享运行时状态，没有中心控制节点。Guild（多 Server）功能已有数据模型、路由、信令隔离雏形，但权限种子、房间归属、前端 guild 上下文、删除清理等链路未接通，实际不可用。

本计划将架构调整为 k8s 控制面模型：**单一 Center Agent 节点**统一管理集群调度与业务控制面，多个 **Worker 节点**承载 Server 运行时。

## 2. 已确认架构决策

| 决策 | 内容 |
|---|---|
| 中心节点 | 只有一个 Center Agent 节点，承担集群调度 + Guild 业务控制面 |
| Worker | 完整 GOSpeak 后端实例，拥有信令、SFU、房间运行时能力 |
| 节点与 Server | 一个节点可托管多个 Server；一个 Server 可跨多个节点多副本扩容 |
| 无租户设计 | 用户、权限、全局数据为单租户统一模型；Server 只是可调度工作负载 |
| 一致性 | 所有最终用户数据只由 Agent 节点保证一致性，Worker 只读缓存 + 运行时快照 |
| 复用 | 复用现有 NATS/Redis Bus、signal.Hub、SFU Provider 抽象，不重写 |

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

## 4. 阶段计划

### Phase 0：控制面正确性前置

目标：修复现有 Guild 断链，让 Server 管理先可用，为集群化提供正确的控制面数据。

任务：

- [ ] 权限种子：将 `guild:*` 权限码加入 `model.DefaultPermissions`、`DefaultRolePermissions`，并同步前端 `rolePermissions`
- [ ] 房间归属：`CreateRoomRequest` 增加 `guild_uuid`，`RoomService.CreateRoom` 持久化归属
- [ ] 房间列表：`RoomInfo` 增加 `guild_uuid`，修复 `getMergedRooms` 按逻辑名合并导致的跨 Server 串名
- [ ] 中间件接线：在 `server/gin.go` 注入 `SetGuildChecker(guildSvc.IsMember)`，Guild 资源路由挂载 `RequireGuildMember`
- [ ] 删除清理：Guild 删除时清理 `guild_members`、关联房间，并接入 `signalHub.OnGuildDelete`
- [ ] 默认服务器：修正 `migrateDefaultGuild` 的 owner/成员缺失问题
- [ ] 修复 `internal/signal` 测试编译失败（`message_bridge_test.go` 与 `OnDisconnect` 签名不匹配）

验收：

- 创建、查看、更新、删除、踢人、成员列表接口不再 403
- 房间按 Server 过滤与隔离，同名房间不串
- Guild 删除后成员、房间、信令会话全部清理

### Phase 1：角色化启动

目标：同一进程可切换为控制面或数据面。

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

- [ ] Agent 主备切换，保持单一写者
- [ ] 按负载自动扩缩容 Server 副本
- [ ] 滚动升级与节点灰度下线

## 5. 风险与依赖

| 风险 | 说明 | 缓解 |
|---|---|---|
| signal Hub 全量本地状态 | 现有 Hub 维护本地房间/成员/stream | 改为“本地运行时 + NATS KV 快照 + Agent 对账” |
| 前端信令路由 | 需要按 Server/房间连接目标节点 | Agent 下发 `workerUrl`，前端处理重连 |
| Worker 写面禁用 | 同一二进制需要路由层模式开关 | Phase 1 先行，测试覆盖 403 路径 |
| SQLite 限制 | SQLite 无法多节点共享权威库 | 生产必须 PostgreSQL，SQLite 仅 `all` 开发模式 |
| 现有测试断裂 | signal 测试当前编译失败 | Phase 0 修复，并补 Guild/集群测试 |
| 单 Agent 单点 | 用户明确单中心节点 | 权威数据落 DB，重启可恢复；HA 留 Phase 6 |

阶段依赖：Phase 0 → 1 → 2 → 3 → 4 → 5；Phase 6 独立可选。

## 6. 建议执行顺序

1. 先完成 Phase 0，恢复 Guild 基础能力并补齐测试。
2. Phase 1 角色化改造优先保证 `all` 模式行为不变。
3. Phase 2/3 实现节点注册与 Server 调度，前端配合改动。
4. Phase 4 收紧一致性边界，完成控制命令与对账。
5. Phase 5 交付部署与监控，Phase 6 按需演进。
