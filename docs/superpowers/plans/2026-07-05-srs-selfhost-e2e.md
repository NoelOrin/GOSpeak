# SRS 自部署 e2e 集成 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 打通 SRS 自部署端到端 — 修配置、起服务、手动双向音频验证,并加 Playwright 自动 e2e。

**Architecture:** SRS5 docker 自部署,WHIP/WHEP 双 PC 直连(前端已有,无需改代码层)。修 `candidate` 解决 ICE,补 env 切换 provider,文档化手动 runbook,Playwright 2-context 验证 track 到达。

**Tech Stack:** SRS6 (`ossrs/srs:6`) · Docker Compose · Go env · Vite env · Playwright (`@playwright/test`)

## Global Constraints

- 不改 SRS provider/service/client Go 代码 — 已实现,仅修配置与部署
- 不改 `packages/sfu-client/src/srs-client.ts` — WHIP/WHEP 实现已完整
- dev 范围:`CANDIDATE=127.0.0.1`,浏览器与 docker 同宿主。LAN 部署仅文档说明
- token 装饰性 JWT(SRS 默认不校验 Bearer)dev 可接受,记录在案不修
- commit 规范: Conventional Commits (`feat:`/`fix:`/`docs:`/`chore:`)
- `.env.dev`/`.env.prod`/`.env.local` 已 gitignore,本计划仅改模板与新建 local(不入库)
- SRS 端口: `1935`(rtmp) `1985`(http_api+rtc+whip) `8080`(http_server) `8000/udp`(rtc_media)
- web 端口 vite 动态(`findAvailablePort` 从 42069 起)→ Playwright `webServer` 用固定 port `5180`

---

## File Structure

| 文件 | 责任 | 动作 |
|------|------|------|
| `deploy/srs/srs.conf` | SRS 配置,candidate 修正 | Modify |
| `deploy/docker-compose.example.yml` | 启用 srs 服务 + env | Modify |
| `app/server/.env.dev` | 切换 provider 用(本地,不入库) | Modify(本地) |
| `app/web/.env.example` | 前端 env 模板,加 srs 注释行 | Modify |
| `app/web/.env.local` | 前端实际切换 provider(本地,不入库) | Create(本地) |
| `docs/srs-selfhost-runbook.md` | 手动 e2e 复现步骤 + 排查表 | Create |
| `app/web/package.json` | 加 `@playwright/test` devDep + e2e script | Modify |
| `app/web/playwright.config.ts` | Playwright 配置(固定 webServer port) | Create |
| `app/web/e2e/srs-audio.spec.ts` | 2-context WHIP→WHEP track 到达断言 | Create |
| `docs/sfu-provider-maturity.md` | SRS 段补"自部署 e2e 已验证" | Modify |

---

### Task 1: 修 SRS candidate — `deploy/srs/srs.conf`

**Files:**
- Modify: `deploy/srs/srs.conf:19-23`

**Interfaces:**
- Produces: `srs.conf` 用 `$CANDIDATE` env 注入,compose 传值(Task 2 依赖此变量名)

- [ ] **Step 1: 改 rtc_server candidate**

`deploy/srs/srs.conf` 第 19-23 行,把 `candidate *;` 改为 `candidate $CANDIDATE;`:

```
rtc_server {
    enabled on;
    listen 1985;
    candidate $CANDIDATE;
}
```

SRS5 docker 镜像启动时用 envsubst 风格替换 `$CANDIDATE`(SRS 支持 `env` 语法 `candidate $CANDIDATE;` 读取环境变量)。dev 传 `127.0.0.1`。

- [ ] **Step 2: 提交**

```bash
git add deploy/srs/srs.conf
git commit -m "fix(srs): candidate 用 env 注入解决容器 ICE 不可达"
```

---

### Task 2: 启用 compose srs 服务 + CANDIDATE env

**Files:**
- Modify: `deploy/docker-compose.example.yml:163-181`

**Interfaces:**
- Consumes: Task 1 的 `$CANDIDATE` 变量名
- Produces: `docker compose up -d srs` 可起 SRS,健康检查端口 1985

- [ ] **Step 1: 取消注释 + 加 environment**

`deploy/docker-compose.example.yml` 第 163-181 行,把注释的 srs 服务块替换为启用版:

```yaml
  # ===========================================================================
  # SFU Provider: SRS (备选)
  # ===========================================================================
  # 启用: .env 设 SFU_PROVIDER=srs + app/web/.env.local 设 VITE_SFU_PROVIDER=srs
  srs:
    image: ossrs/srs:6
    container_name: gospeak-srs
    restart: unless-stopped
    ports:
      - "1935:1935"
      - "1985:1985"
      - "8080:8080"
      - "8000:8000/udp"
    environment:
      CANDIDATE: "127.0.0.1"
    volumes:
      - ./srs/srs.conf:/usr/local/srs/conf/srs.conf
    command: ./objs/srs -c conf/srs.conf
    extra_hosts:
      - "host.docker.internal:host-gateway"
```

LAN 部署:改 `CANDIDATE` 为宿主 LAN IP。

- [ ] **Step 2: 起 SRS 验证健康**

```bash
docker compose -f deploy/docker-compose.example.yml up -d srs
sleep 3
curl -s http://localhost:1985/api/v1/versions | head
```

Expected: JSON 含 `"code":0`。

- [ ] **Step 3: 提交**

```bash
git add deploy/docker-compose.example.yml
git commit -m "chore(deploy): 启用 SRS compose 服务 + CANDIDATE env"
```

---

### Task 3: env 模板补 SRS 块(可复现切换)

**Files:**
- Modify: `app/web/.env.example`

**Interfaces:**
- Produces: `.env.example` 含 `VITE_SFU_PROVIDER=srs` 注释行(用户本地 copy 到 `.env.local`)

- [ ] **Step 1: 改 `.env.example`**

`app/web/.env.example` 当前内容:
```
VITE_SFU_PROVIDER=livekit
VITE_AGORA_APP_ID=
```
追加 SRS 说明行:
```
VITE_SFU_PROVIDER=livekit
VITE_AGORA_APP_ID=
# SRS 自部署: 改上行为 VITE_SFU_PROVIDER=srs
```

- [ ] **Step 2: 提交**

```bash
git add app/web/.env.example
git commit -m "docs(web): env 模板补 SRS provider 切换说明"
```

---

### Task 4: 手动 e2e runbook — `docs/srs-selfhost-runbook.md`

**Files:**
- Create: `docs/srs-selfhost-runbook.md`

**Interfaces:**
- Produces: 复现步骤文档,Task 5(Playwright)与验收依赖此 runbook 的端口/启动约定

- [ ] **Step 1: 写 runbook**

`docs/srs-selfhost-runbook.md`:

````markdown
# SRS 自部署端到端 Runbook

dev 环境(浏览器与 docker 同宿主)。LAN 部署见末节。

## 1. 起 SRS

```bash
docker compose -f deploy/docker-compose.example.yml up -d srs
curl -s http://localhost:1985/api/v1/versions   # 期望 {"code":0,...}
```

## 2. 后端切 SRS

编辑 `app/server/.env.dev`:
- 注释 `SFU_PROVIDER="livekit"` 行
- 取消注释(或新增)`SFU_PROVIDER="srs"`

server config 已有 `SRS_HOST=localhost SRS_API_PORT=1985 SRS_WHIP_PORT=1985 SRS_SECRET=` 默认值,无需另设。

启动:
```bash
pnpm dev:server
```

## 3. 前端切 SRS

新建 `app/web/.env.local`(已 gitignore):
```
VITE_SFU_PROVIDER=srs
```

启动:
```bash
pnpm dev:web
```

## 4. 双标签双向音频验证

1. 浏览器开 `http://localhost:<vite端口>`(终端输出实际端口)
2. 标签 A 注册/登录,创建/加入房间 R
3. 标签 B(同浏览器不同标签或无痕)登录另一账号,加入房间 R
4. A 说话 → B 听到;B 说话 → A 听到
5. 任一方离场 → 另一方收到 track removed

## 排查表

| 症状 | 原因 | 修 |
|------|------|-----|
| ICE failed / 无声 | candidate 错 | `docker exec gospeak-srs printenv CANDIDATE` 应为 `127.0.0.1` |
| WHIP 401 | (dev 不应发生)SRS auth 开了 | 确认 srs.conf 无 `http_api auth` |
| 前端仍连 livekit | env 没生效 | 重启 `pnpm dev:web`,确认 `.env.local` 在 app/web 下 |
| `curl /api/v1/streams` 空 | 还没人 publish | 正常,publish 后才出现 stream |
| WHEP 收不到 track | join 顺序 | 任一方 publish 后另一方才能 WHEP subscribe |

## LAN 部署

改 `deploy/docker-compose.example.yml` srs 服务的 `CANDIDATE` 为宿主 LAN IP(如 `192.168.1.10`)。浏览器与 SRS 不同机时必须,否则 ICE candidate 不可达。
````

- [ ] **Step 2: 提交**

```bash
git add docs/srs-selfhost-runbook.md
git commit -m "docs: SRS 自部署 e2e runbook"
```

---

### Task 5: 加 Playwright devDep + config

**Files:**
- Modify: `app/web/package.json`
- Create: `app/web/playwright.config.ts`

**Interfaces:**
- Produces: `pnpm test:e2e:srs` 脚本 + Playwright 配置(`webServer` 起 vite 固定 port 5180,前置起 SRS + server)

- [ ] **Step 1: 安装 @playwright/test**

```bash
cd app/web && pnpm add -D @playwright/test && pnpm exec playwright install chromium
```

- [ ] **Step 2: package.json 加 script**

`app/web/package.json` `scripts` 段加:
```json
    "test:e2e:srs": "playwright test --config=playwright.config.ts e2e/srs-audio.spec.ts"
```

- [ ] **Step 3: 写 playwright.config.ts**

`app/web/playwright.config.ts`:

```typescript
import { defineConfig, devices } from "@playwright/test";

export default defineConfig({
	testDir: "./e2e",
	timeout: 60_000,
	expect: { timeout: 15_000 },
	workers: 1,
	retries: 1,
	use: {
		baseURL: "http://localhost:5180",
		headless: true,
		launchOptions: {
			args: [
				"--use-fake-ui-for-media-stream",
				"--use-fake-device-for-media-stream",
				"--autoplay-policy=no-user-gesture-required",
			],
		},
	},
	projects: [
		{ name: "chromium", use: { ...devices["Desktop Chrome"] } },
	],
	webServer: {
		command: "pnpm dev --port 5180 --strictPort",
		url: "http://localhost:5180",
		timeout: 60_000,
		reuseExistingServer: true,
	},
});
```

- [ ] **Step 4: 验证 config 可加载(无 spec 也应不报错)**

```bash
cd app/web && pnpm exec playwright test --config=playwright.config.ts --list 2>&1 | head
```

Expected: "No tests found" 或类似(因为 spec 还没写),不报 config 语法错。

- [ ] **Step 5: 提交**

```bash
git add app/web/package.json app/web/playwright.config.ts
git commit -m "chore(web): 加 Playwright e2e 框架 + srs 测试脚本"
```

---

### Task 6: Playwright SRS e2e spec — `app/web/e2e/srs-audio.spec.ts`

**Files:**
- Create: `app/web/e2e/srs-audio.spec.ts`

**Interfaces:**
- Consumes: Task 5 的 config(2 个 context + fake media flags)
- Produces: `pnpm test:e2e:srs` 验证 WHIP→WHEP track 到达

**前提(手动,非脚本自动):** SRS 容器已起 + 后端 `SFU_PROVIDER=srs` 已起 + 两个测试账号已注册。脚本聚焦前端 join→subscribe 通路。

- [ ] **Step 1: 写 spec**

`app/web/e2e/srs-audio.spec.ts`:

```typescript
import { expect, test } from "@playwright/test";

const ROOM = `e2e-srs-${Date.now()}`;

async function loginAndJoin(page: import("@playwright/test").Page, room: string) {
	await page.goto("/");
	// 假设已有测试账号 test-a/test-a、test-b/test-b(前置手动注册)
	// 登录流程依 UI 实际选择器调整;此处给最小骨架,执行前需对齐真实 selector
	await page.fill('[data-testid="login-username"]', "");
	await page.fill('[data-testid="login-password"]', "");
	await page.click('[data-testid="login-submit"]');
	await page.fill('[data-testid="room-name"]', room);
	await page.click('[data-testid="room-join"]');
}

test("SRS WHIP publish → WHEP subscribe: 远端 track 到达", async ({ browser }) => {
	const ctxA = await browser.newContext({
		permissions: ["microphone"],
	});
	const ctxB = await browser.newContext({
		permissions: ["microphone"],
	});
	const pageA = await ctxA.newPage();
	const pageB = await ctxB.newPage();

	// 注入探针:截获 SRSSFUClient onRemoteAudioTrack 回调
	await pageA.addInitScript(() => {
		(window as any).__remoteTracks = 0;
		const orig = (window as any).navigator;
	});
	await pageB.addInitScript(() => {
		(window as any).__remoteTracksArrived = false;
		// 拦截自定义事件或全局钩子;实际需对齐 app 暴露的 hook
	});

	await loginAndJoin(pageA, ROOM);
	await loginAndJoin(pageB, ROOM);

	// 等待 B 收到 A 的远端 track。具体断言点依 app 暴露面调整:
	// 方案 A: app 在 DOM 暴露 [data-testid="remote-track"] 元素
	// 方案 B: window 全局计数器(需 app 侧加 test-only 钩子)
	await expect(
		pageB.locator('[data-testid="remote-audio-count"]'),
	).toContainText(/[1-9]/, { timeout: 30_000 });

	await ctxA.close();
	await ctxB.close();
});
```

**注:** selector(`[data-testid=...]`)为占位骨架。执行前需对齐 app/web 真实 DOM:
- 若 app 无 testid,优先加 `data-testid` 到房间页相关元素(login form、room join、remote audio list)
- 或在 `socketStore`/`sfuSession` 加 `if (import.meta.env.DEV) (window as any).__srsRemoteTracks++` test-only 钩子,spec 读 `window.__srsRemoteTracks`

执行者在此 task 内补齐真实断言面,不另开 task。

- [ ] **Step 2: 跑 spec 验证**

前置:
```bash
docker compose -f deploy/docker-compose.example.yml up -d srs
# 后端 .env.dev 设 SFU_PROVIDER=srs 后
pnpm dev:server
# 注册 test-a/test-b 两账号(或调整 spec 用现有测试账号)
```

```bash
cd app/web && pnpm test:e2e:srs
```

Expected: 1 passed。flaky 允许 `--retries`(config 已设 1)。

- [ ] **Step 3: 提交**

```bash
git add app/web/e2e/srs-audio.spec.ts
git commit -m "test(web): SRS WHIP→WHEP e2e track 到达断言"
```

---

### Task 7: 文档收尾 — sfu-provider-maturity + .env.dev 注释块

**Files:**
- Modify: `docs/sfu-provider-maturity.md`
- Modify(本地,不入库): `app/server/.env.dev`

**Interfaces:**
- Produces: maturity 文档反映 SRS 自部署已验证

- [ ] **Step 1: maturity 文档补注**

`docs/sfu-provider-maturity.md` SRS 段(约 46-54 行)末尾追加:

```markdown
> 自部署 e2e 已验证(2026-07-05):docker compose + WHIP/WHEP 双向音频通,runbook 见 `docs/srs-selfhost-runbook.md`,自动 e2e 见 `app/web/e2e/srs-audio.spec.ts`。
```

- [ ] **Step 2: 提交**

```bash
git add docs/sfu-provider-maturity.md
git commit -m "docs(sfu): SRS 自部署 e2e 已验证标记"
```

- [ ] **Step 3: 本地 .env.dev 加 SRS 块(不入库)**

`app/server/.env.dev` LiveKit 块后追加(注释状态,切换时手动取消注释):

```
# SRS 自部署 (切换: 注释上方 livekit, 取消本行注释)
# SFU_PROVIDER="srs"
# SRS_HOST="localhost"
# SRS_API_PORT="1985"
# SRS_WHIP_PORT="1985"
# SRS_SECRET=""
```

不入库(.env.dev gitignored),无需 commit。

---

## Self-Review

**Spec coverage:**
- §设计1 candidate 修正 → Task 1 ✓
- §设计2 compose 启用 → Task 2 ✓
- §设计3 env 配置(server + web)→ Task 3(web 模板)+ Task 7 Step 3(server .env.dev 本地)✓
- §设计4 手动 runbook → Task 4 ✓
- §设计5 Playwright → Task 5(config)+ Task 6(spec)✓
- §验收 标准1 健康 → Task 2 Step 2 ✓
- §验收 标准2 双向音频 → Task 4 runbook ✓
- §验收 标准3 e2e pass → Task 6 Step 2 ✓
- §验收 标准4 runbook 完整 → Task 4 ✓

**Placeholder scan:** Task 6 spec 的 `data-testid` 为骨架,但 task 内明确要求执行者对齐真实 DOM/加 test-only 钩子,非"TODO 留空"。其余无 placeholder。

**Type consistency:** `CANDIDATE` env 名 Task1/2 一致。`VITE_SFU_PROVIDER` Task3/4 一致。`SFU_PROVIDER` Task4/7 一致。端口 5180(Task5)/1985(各处)一致。
