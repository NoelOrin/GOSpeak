# SRS 自部署端到端集成设计

日期: 2026-07-05
分支: feature/agora-sfu (后续切 feature/srs-selfhost)

## 背景

SRS 作为 GOSpeak 多 SFU Provider 之一,代码层面已实现但从未端到端打通:

- 后端 provider `app/server/internal/srs/` — REST + JWT,9 方法中 4 个 `ErrNotSupported`(`ListParticipants`/`Mute`/`MuteRoom`)
- 前端 `packages/sfu-client/src/srs-client.ts` — WHIP/WHEP 双 RTCPeerConnection 真实现,leaveRoom 正确 DELETE resource
- deploy `deploy/srs/srs.conf` + `deploy/docker-compose.example.yml:167-180` — compose 服务被注释
- server config 对 `SRS_*` 有 localhost 默认值;`.env.dev` 无 SRS 变量
- web `DEFAULT_SFU_PROVIDER="livekit"`,需 `VITE_SFU_PROVIDER=srs` 切换

token 为装饰性 JWT:SRS 默认不校验 WHIP Bearer,`SRSSecret` 空时退化为 `room:identity` 明文。自部署 dev 可接受,安全靠网络边界。

## 目标

1. **B — 基础设施打通 + 手动浏览器 e2e**:取消 compose 注释、修 SRS candidate、配 env、跑通双人通话
2. **C — Playwright 自动 e2e**:2 context + fake media,断言 WHIP→WHEP 订阅通路

## 非目标

- SRS WHIP auth 强制校验(后续 hook,另议)
- `ListParticipants` / `Mute` 补齐(SRS5 REST 可做,范围另议)
- 多设备 LAN 部署(仅文档说明,不实现)
- prod `.env.prod` SRS 配置(聚焦 dev)

## 现状诊断(必修项)

| 问题 | 位置 | 影响 |
|------|------|------|
| `candidate *` | `deploy/srs/srs.conf:23` | 容器内解析为容器 IP,宿主浏览器 ICE 不可达 → 通话必失败 |
| compose srs 服务注释 | `deploy/docker-compose.example.yml:167-180` | SRS 容器不起 |
| `.env.dev` 无 SRS 块 | `app/server/.env.dev` | 切换 provider 需手敲 env,易错 |
| web 无 `.env` | `app/web/`(仅 `.env.example`) | `VITE_SFU_PROVIDER` 缺省回退 livekit |
| token 装饰性 | `internal/srs/token.go` | SRS 不校验,dev 可接受,记录在案 |

## 设计

### 1. SRS 配置修正 — `deploy/srs/srs.conf`

`candidate *` → `candidate $CANDIDATE;`。SRS5 docker 镜像支持 env 注入 `$CANDIDATE`。compose 传 `CANDIDATE=127.0.0.1`(单机 dev,浏览器与 docker 同宿主)。LAN 部署文档注明改宿主 LAN IP。

### 2. compose 启用 — `deploy/docker-compose.example.yml`

取消 srs 服务注释,`environment` 加 `CANDIDATE=127.0.0.1`。保留端口映射 `1935/1985/8080/8000-udp`(8000/udp 为 RTC 媒体)。

### 3. env 配置

- `app/server/.env.dev`:追加注释块 `# SFU_PROVIDER="srs"` + `# SRS_*` 变量(全有默认,注释状态)。切换 = 取消 `SFU_PROVIDER=srs` 注释、注释掉 livekit 行。
- `app/web/.env.local`(新建,已 gitignore):`VITE_SFU_PROVIDER=srs`。同步在 `.env.example` 加注释行。

### 4. 手动 e2e runbook — `docs/srs-selfhost-runbook.md`

步骤:
1. `docker compose -f deploy/docker-compose.example.yml up -d srs`
2. `SFU_PROVIDER=srs pnpm dev:server`(或编辑 `.env.dev`)
3. `pnpm dev:web`(读 `.env.local`)
4. 浏览器开两个标签,加入同一房间
5. 标签1说话 → 标签2听到;反向验证

验收:双向音频通。失败排查表(candidate / 端口 / provider 切换 / SRS REST `/api/v1/streams` 自检)。

### 5. Playwright e2e — `app/web/e2e/srs-audio.spec.ts`

- 新增 devDep `@playwright/test` + `app/web/playwright.config.ts`
- 测试:2 个 browser context,chromium flags `--use-fake-ui-for-media-stream --use-fake-device-for-media-stream`
- context1 `joinRoom` → context2 `joinRoom` 同房 → 断言 context2 `onRemoteAudioTrack` 回调触发 + `track.attach()` 返回 `HTMLAudioElement`
- **不验证可听音量**(headless 不可行),仅验证 track 到达 = WHIP publish → WHEP subscribe 通路通
- 脚本 `pnpm test:e2e:srs`,前置依赖运行中的 SRS(`docker compose up -d srs`),不进默认 `pnpm test`,文档标 flaky 可能

## 风险

- **SRS candidate 错配** → ICE fail。dev 用 `127.0.0.1` 缓解;LAN 需手动改
- **Playwright WebRTC e2e flaky** → gated 脚本,非 CI 默认,允许重试
- **macOS chromium fake media** → 需特定 flag 组合,runbook 记录
- **token 装饰性** → dev 可接受,prod 自部署需加 SRS http_api auth 或 WHIP hook(非本计划)

## 测试

- B: 手动 runbook 验收(双向音频)
- C: `pnpm test:e2e:srs` pass(track 到达断言)

## 验收标准

1. `docker compose up -d srs` 后 SRS 健康(`http://localhost:1985/api/v1/versions` 返回 code=0)
2. 两标签加入同房,双向听到对方说话
3. `pnpm test:e2e:srs` 通过
4. runbook 文档完整可复现
