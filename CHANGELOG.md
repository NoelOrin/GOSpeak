# Changelog / 更新日志

## [0.3.0](https://github.com/NoelOrin/GOSpeak/compare/v0.2.3...v0.3.0) (2026-08-28)

> 自 v0.2.3 以来的发布，共 28 个变更点（feat 12 / fix 4 / refactor 3 / style 2 / perf 1 / docs 4 / ci 1 / chore 1），基于 1 个合并提交（PR #17，含 28 个源提交）。

### Features

* feat(guest): 游客访问权限体系（guest 登录、guard 中间件、配置/封禁/清理/续期 API、Domain 级 listen/speak/message 开关）
* feat(guest): GuestService join/ban 与 DomainGuestBan 模型 + 仓库
* feat(guest): 前端游客入口页、登录游客按钮、guest store 与 api client
* feat(guest): 按 Domain 能力门控的游客 UI 与管理员审核界面
* feat(guest): 在 signal/sfu/message 路径强制 listen/speak/message 开关
* feat(web): 登录页重设计（鼠标视差）
* feat(auth): 重构登录权限服务 + 邮箱验证码模板按主应用风格重写
* feat(permission): 新增基于 casbin 的 Domain 权限适配器

### Bug Fixes

* fix(guest): 加固 cleanup、join guard 与 renewal 校验
* fix(guest): 加固 ban check、guard 白名单与 speak-off 强制
* fix(guest): guest join 响应不泄露邀请码
* fix(cluster): 加固多节点 agent-worker 运行时

### Refactor

* refactor(storage): 将 MinIO 替换为 RustFS 对象存储
* refactor(auth): 抽取 issueTokens 令牌对辅助函数
* refactor(ws): 移除 Fanout.marshalCount 生产字段，测试改用收消息数断言

### Style

* style(web): 登录页重设计 + 鼠标视差
* style(server): gofmt guest handler 与 router imports

### Performance

* perf(sfu): DynamicProvider 配置缓存 + Fanout 反向索引

### Documentation

* docs(guest): 游客访问权限设计文档与实现计划
* docs(guest): 游客访问路由说明

### CI

* ci: 重写 release 工作流，支持 push 到分支时自动发布

## [0.2.3](https://github.com/NoelOrin/GOSpeak/compare/v0.2.2...v0.2.3) (2026-08-22)

> 自 v0.2.2 以来的发布，共 5 个变更点（refactor 1 / fix 2 / style 1 / ci 1），基于 1 个提交。

### Refactor

* refactor(auth): integrate casbin, authboss, go-oauth2 and go-mail

### Bug Fixes

* fix(server): harden signal, SFU, auth and infra edge cases
* fix(server): permanent bot NULL expiry, JWT compat, SFU cleaner lifecycle and hub heartbeat

### Style

* style(web): replace favicon/logo with new A2 mark

### CI

* ci: remove release-please workflow

## [0.2.2](https://github.com/NoelOrin/GOSpeak/compare/v0.2.1...v0.2.2) (2026-08-21)

> 自 v0.2.1 以来的发布，共 8 个变更点（refactor 1 / fix 2 / style 1)，基于 1 个提交。

### Refactor

* refactor(auth): integrate casbin, authboss, go-oauth2 and go-mail

### Bug Fixes

* fix(server): harden signal, SFU, auth and infra edge cases
* fix(server): permanent bot NULL expiry, JWT compat, SFU cleaner lifecycle and hub heartbeat

### Features

* feat(auth): use Authboss BCryptHasher for password hash/verify (compatible with existing bcrypt hashes)
* feat(oauth): migrate token exchange for GitHub/Google/QQ/Generic to golang.org/x/oauth2
* feat(email): replace manual SMTP with go-mail client (SSL 465 / STARTTLS 587)
* feat(web): add role permission management page with create/delete role and permission sync
* feat(permission): replace in-memory cache with Casbin SyncedEnforcer backed by role_permissions table

### Style

* style(web): replace favicon/logo with new A2 mark (acid green tile, black bubble, white G)

## [0.2.1](https://github.com/NoelOrin/GOSpeak/compare/v0.2.0...v0.2.1) (2026-08-19)

> 自 v0.2.0 以来的发布，共 12 个变更点（fix 10 / chore 1 / ci 1)，基于 1 个提交。

### Bug Fixes

* fix(audit): validation, AuditIP via RemoteIP, nil-DB guard, dropped counter, logger
* fix(handler): validate audit params, use AuditIP in domain/mute/room/user
* fix(domain): atomic ResetInviteCode with RETURNING fallback, reset_invite audit
* fix(domain/web): invite error handling, QR race and clipboard timer fixes
* fix(sfu): CachedMuteRuleStore L1 cache, none-backend warnings, Agora/SRS guards
* fix(sfu/srs): publish block error handling and SRS store wiring warning
* fix(bus): document NATS mandatory for membership/mute stores
* fix(server): fix duplicate plugin StopAll in graceful shutdown
* fix(web): link page rAF dialog and timer handling
* fix(bot): speakingRooms ordering, SFU token inside try, identity guard

### Chore

* chore: extend lefthook pre-push (biome ci, go vet, typecheck, go test)

### CI

* ci: build docker image and binaries on release published

## 0.2.0 (2026-08-18)

> 自 v0.1.0-alpha1 以来的发布，共 287 个提交（feat 129 / fix 65 / perf 1 / docs 20 / ci 3）。

### Features

* feat(bot): improve socket client capabilities and add tests
* feat(web): domain invite share modal and link page
* feat(sfu): consolidate hard mute rule store across sfu/bus and agora/srs providers
* feat(domain): harden domain RBAC, mute, room and user management with db migration
* feat(audit): add audit log module and API routes
* feat(sfu-client): event-driven speaking detection
* feat(sfu): hard mute fixes, provider hardening and UI contract alignment
* feat(sfu,srs): Discord-style hard mute and SRS API room management
* feat(handler): add domain-first resource permission helper
* feat: enhance message, auth, WebSocket and cluster capabilities
* feat: 域权限优先并下线全局权限页
* feat: cluster agent worker, room resolver and voice session updates
* feat: enforce domain permissions on message deletion
* feat: add domain role management APIs and enforce domain permissions
* feat: add domain role management UI
* feat: add per-domain role and permission service
* feat: migrate and seed per-domain roles
* feat: add domain role repository and seed
* feat: add frontend domain role API and permission cache
* feat: add per-domain role models
* feat(db): support Turso libSQL with auto migration and tests
* feat(sfu-client): implement restartIce in mediasoup client for transport reconnection
* feat: cluster agent runtime + observability stack + comprehensive test suite
* feat(web): pass explicit domain when creating room
* feat(web): room list type icons per room type
* feat(mediasoup-worker): 显式资源清理与单元测试
* feat(web): Socket 状态机、乐观消息幂等与断线清理
* feat(server): 注入 bot token 校验、禁言过期与域踢出接线
* feat(ws): 连接状态机与错误 ACK 脱敏
* feat(signal): 域内踢人与踢出冷却，join 注册移出全局锁
* feat(sfu): 能力矩阵覆盖、provider 资源释放与错误测试
* feat(mute): 临时禁言到期后完整解禁
* feat(message): 稳定作者 UUID、消息幂等与软删除状态
* feat(domain): 事务化归属转移与跨实例踢出命令
* feat(auth): 加固 JWT 鉴权与用户状态管理
* feat(bus): Redis 成员 CAS 与 JetStream 消息去重
* feat(signal): 多实例房间元数据与成员原子注册
* feat(bus): 强化共享状态存储与断线恢复
* feat(cluster): add leader lock and auto-scaling hooks
* feat(deploy): add cluster nginx routing
* feat(web): add cluster management page
* feat(cluster): expose cluster health stats
* feat(cluster): reconcile cluster state on agent startup
* feat(cluster): worker executes NATS control commands
* feat(cluster): publish control commands from agent
* feat(cluster): define NATS control command envelope
* feat(cluster): explicitly reject worker business writes
* feat(cluster): worker mode uses read-only DB and skips seeding
* feat(web): route voice signal join to assigned worker
* feat(web): support worker URL socket connection
* feat(domain): enrich member list with user names
* feat(frontend): update domain detail page with room improvements
* feat(room): add domain-required validation and update room module
* feat(skills): update room-voice-e2e skill documentation
* feat(frontend): update socket store and API
* feat(backend): update message service and signal hub
* feat(frontend): update UI components, pages and assets
* feat(frontend): update UI components, pages and stores
* feat(auth): add auth middleware improvements and user service enhancements
* feat(room): add room duplicate detection and list utilities
* feat(message): enhance message and conversation services with tests
* feat(oauth): add OAuth account encryption and provider models
* feat(domain): implement complete Domain module with CRUD and tests
* feat(cluster): add cluster events, scheduler tests and handler APIs
* feat: add cluster module, improve storage service, and polish frontend UI
* feat(domain): add room management to domain admin page
* feat(storage): allow PDF and plain text uploads
* feat(room): improve room list and detail components
* feat(user-group): add user group module (backend + frontend)
* feat(web): add domain room table
* feat(web): add edit room modal
* feat(web): support target domain in create room modal
* feat(web): add room list update delete api
* feat(server): enforce room manage permission on update and delete
* feat(web): polish room and domain UI
* feat(web): add guild invite preview and polish invite flow
* feat(web): add GuildMemberTable component and guild management page
* feat(web): update room list UI, guild index page, and room state management
* feat(web): update guild store and API with multi-guild support
* feat(signal): update hub with guild namespace isolation and state sync
* feat(server): update room handler and service with guild isolation
* feat(server): add guild middleware for request context and DB init improvements
* feat(bot): add media routing, TTS, ASR and WS ticket auth
* feat(web): add guild discovery, text chat and UX improvements
* feat(server): complete guild permissions, discovery and room isolation
* feat(server): harden auth, uploads and WebSocket handshake
* feat(ws): complete socket.io to WebSocket migration
* feat: socket
* feat(server): add guildUUID to JoinPolicy interface
* feat(server): reset-password CLI command and planning docs
* feat(web): private message API types and socket events
* feat(socket): add native WebSocket client adapter
* feat(guild): integrate GuildList into layout, add create/join buttons, enhance guild page
* feat(chat): add private chat UI with conversation list, chat window, member sidebar, and /chat route
* feat(socket): bind PRIVATE_NEW event globally in socketStore
* feat(signal): wire private:send WS event and /conversation/send endpoint
* feat(service): add SendDirect for private chat messages
* feat(model): add conversation fields to Message and private chat event constants
* feat: 添加文字聊天功能并完善相关文档与接口
* feat: compact
* feat(chat): add frontend conversation API, chat store, and IDB cache
* feat(chat): add DM signal events and personal room routing
* feat(chat): add DM service, handler, router, and DI wiring
* feat(chat): add direct message model and repository layer
* feat(signal): add OnGuildDelete cleanup and DRY test setup
* feat(web): extend PIP keepalive for AndroidTWA and improve error handling
* feat(android): add Android TWA scaffold
* feat(web): add PIP keepalive utility for iOS picture-in-picture
* feat(ws): add ws package (Client, Fanout, HandlerRegistry, Upgrader) + WSDeliverer
* feat(guild): add default Guild migration for existing rooms
* feat(guild): register Guild permissions in seed data
* feat(guild): add frontend Guild API, store, components, and route page
* feat(guild): namespace Signal Hub rooms by guild UUID
* feat(guild): add GuildUUID filtering to Room CRUD and signal roomStore interface
* feat(guild): add Guild handler, routes, middleware, and DI wiring
* feat(guild): add GuildService with create/join/leave/kick/transfer
* feat(guild): add GuildRepository with CRUD and member operations
* feat(guild): add GuildUUID to Room/Message models and guild permcodes
* feat(guild): add Guild and GuildMember data models
* feat(service): add async write-steal pattern to message service
* feat(signal): add KV membership fallback to message bridge
* feat(signal): add ByIdentity index for O(1) member lookup in Hub
* feat(bus): add ConcurrentDeliverer with concurrent fanout to all SIO clients
* feat(im): message list API and DI wiring
* feat(im): socket message:send bridge via MessageService
* feat(im): message service with EventBus publish
* feat(im): add message model and repository
* feat(sfu): 管理页按 provider 隔离保存配置
* feat(web): APIKey 吊销改用确认弹窗

### Bug Fixes

* fix(signal): deny ws message access without claims
* fix(signal): fail closed when ws domain permission checker missing
* fix(signal): enforce domain role permissions on ws message events
* fix(domain): restrict admin assignment to domain owner
* fix(handler): apply claims-aware checks to room manage and delete others
* fix(handler): honor explicit claims permissions on platform fallback
* fix(message): enforce domain role permissions on message endpoints
* fix(room): enforce resource permissions in handler for all room routes
* fix(room): let domain roles manage rooms without global permission gate
* fix(handler): guard domain permission helper against typed nil checkers
* fix(room): enforce domain role permissions on room endpoints
* fix(auth): revoke HttpOnly refresh cookie on logout
* fix(auth): rotate refresh family on every refresh
* fix: set cookie Secure flag based on TLS to avoid cookie issues in prod
* fix: heartbeat goroutine exit wait + S3 5s timeout context
* fix: mediasoup restart-ice, form type safety, domain_uuid validation, permissions from profile
* fix(sfu-client): match domain-scoped room events
* fix(bus): sanitize colons in NATS KV keys
* fix(server): require domain on room create and broadcast room list
* fix(web): room list scrolling with hidden scrollbar
* fix: review gap fixes
* fix(handlers): 控制面失败不阻塞业务响应并拒绝 SRS 非法回调
* fix(plugin): 拒绝非法生命周期状态迁移
* fix(ws): 连接级唯一 ID、心跳保活与幂等关闭
* fix: 修复跨域房间切换与 chatStore 循环依赖
* fix: harden cluster/signal/web/deploy review findings
* fix(server): repair upload ownership check and plugin secret encryption
* fix(server): harden oauth bot plugin auth and secret storage
* fix(server): harden ws lifecycle and speaking mute checks
* fix(server): harden cluster lifecycle, key rotation and state sync
* fix(server): bind cloudflare sessions to their owner
* fix(server): verify webhook signatures and honor bot claim permissions
* fix(server): keep permanent mutes enforced beyond 24 hours
* fix(server): reload storage config and enforce upload ownership
* fix(web): pass domain_uuid and sfuRoom through voice join
* fix(server): read domain_uuid from middleware context
* fix(web): sync frontend permissions and update AGENTS docs
* fix(signal): use composite room keys for media cleanup and slots
* fix(server): enforce domain membership and use composite sfu room
* fix(router): normalize import ordering and fix test coverage
* fix(text-room): improve message input and rendering
* fix(room): remove unused voiceChat component
* fix(web): surface edit room errors and unify validation
* fix(web): correct edit room modal import casing
* fix: remove duplicate ConversationList import + add @tanstack/solid-virtual
* fix(packages): bot permissions, mediasoup dedup, srs cleanup
* fix(web): form validator, UserInfo type, and vite config guard
* fix(server): concurrency safety for role cache and job queue
* fix(server): path traversal protection and nil pointer fixes
* fix(guild): add error handling, loading text, and data-ready guard to guild page
* fix(chat): set author_id on optimistic messages and remove duplicate PM listeners
* fix(handler): monitor 与 mute 处理器边界条件修复
* fix(plugin): TOCTOU 竞争修复与注册逻辑完善
* fix(service): 错误处理统一与边界检查完善
* fix: compilation blockers, panic risks, crypto, SFU provider bugs
* fix(im): 修未读数bug + fanout nil保护 + 消息竞态 + ack/事务/nit
* fix(android): robust startForeground with try/catch and logging
* fix(server): race-safe guild checker, stack-allocated invite code, testutil options
* fix(message): add write-worker fallback and graceful shutdown
* fix(bot): add livekit-client type declaration for optional dependency
* fix(im): address review findings
* fix(ops,signal,web): cherry-pick portable fixes from mobile-responsive branch
* fix(web): 重连成功后关闭 reconnecting toast
* fix(auth): 开发态固定 JWT 静态密钥
* fix: 小问题

### Performance

* perf(signal): batch localRoomSnapshots to eliminate N+1 in getMergedRoomsScoped

### Documentation

* docs: mark cluster-agent-worker plan as delivered with verification evidence
* docs: add bilingual changelog baseline
* docs(review): split p0 auth and domain rbac plans
* docs(review): mark repo hygiene items as handled
* docs: sync review status and gap-fix plan completion
* docs: 同步 Domain 术语与多实例状态同步说明
* docs(cluster): mark control plane completion status
* docs: add tauri mobile hybrid webrtc design and plan
* docs: update AGENTS.md and documentation
* docs: update deployment docs and superpowers specs/plans
* docs(config): update AGENTS.md, vite config and frontend documentation
* docs: update AGENTS and swagger for domain rename
* docs: design domain room management feature
* docs(web): define room and domain UI
* docs: update AGENTS.md and regenerate swagger docs
* docs: expand cluster agent worker plan with discovery phase
* docs: update AGENTS.md across monorepo for new modules
* docs(guild): update AGENTS.md with Guild architecture and route table
* docs: add websocket fanout migration plan
* docs(plans): room IM over EventBus implementation plan

### CI/CD

* ci: start releases from v0.2.0
* ci: merge release-please automated tagging
* ci: add release-please automated tagging

## v0.1.0-alpha1 (2026-07-15)

- 历史基线：首个 alpha 发布 / Historical baseline: initial alpha release.