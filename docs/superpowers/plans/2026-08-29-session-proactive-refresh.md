# 会话过期下发与主动续期（expires_in + 懒续期）实现计划

> **面向 AI 代理的工作者：** 必需子技能：使用 superpowers:subagent-driven-development（推荐）或 superpowers:executing-plans 逐任务实现此计划。步骤使用复选框（`- [ ]`）语法来跟踪进度。

**目标：** 服务端在 login/register/refresh/OAuth 响应下发 `expires_in`，前端 userStore 据此懒续期——未过期同步放行（切路由零 RTT），临近过期先无感刷新；TTL 提为 env 可配。

**架构：** `pkg/jwt.go` 的两个 TTL 常量改包级变量由 `gin.go` 注入 config 值；auth/oauth 响应结构体加 `expires_in` 字段；前端 `ensureSession` 从「每导航探测 profile」改为「过期时间驱动的三分支懒续期」，401 拦截器兜底撤销不变。

**技术栈：** Go (gin + caarlos0/env + golang-jwt) / SolidJS + TypeScript + vitest。

**约束（重要）：**
- `release` 工作区有一批**未提交**的前置改动（登录页重写、userStore profile-first、OAuth postMessage 等，见规格「关联」）。本计划的前端任务修改其中部分同名文件（userStore.ts / login/index.tsx / api/auth.ts），**无法从这批改动中剥离**——因此**前端任务不做 commit**，与既有批次同置工作区等待用户统一拍板；**后端任务文件均无未提交改动，各自 commit**。
- config TTL 非法值处理遵循同文件 `JWTKeyTTLDuration`（config.go:249）的**回落默认值**模式，不启动报错（规格相应行同步修正）。

**规格：** `docs/superpowers/specs/2026-08-29-session-proactive-refresh-design.md`

---

## 文件结构

| 文件 | 动作 | 职责 |
|------|------|------|
| `app/server/internal/config/config.go` | 修改 | 新增 `JWT_ACCESS_TTL` / `JWT_REFRESH_TTL` 字段 + Duration 方法 |
| `app/server/internal/pkg/jwt.go` | 修改 | TTL const→var + `AccessTokenExpiresIn()` helper |
| `app/server/server/gin.go` | 修改 | 装配时注入 TTL |
| `app/server/internal/service/auth_service.go` | 修改 | `AuthResponse`/`RefreshResponse` 加 `ExpiresIn`，4 构造点填充 |
| `app/server/internal/handler/oauth_handler.go` | 修改 | postMessage payload 加 `expires_in` |
| `app/server/internal/config/config_test.go` | 创建（若无） | TTL 解析测试 |
| `app/server/internal/handler/auth_handler_test.go` | 修改 | login/refresh 响应断言 `expires_in` |
| `app/web/src/api/authTransport.ts` | 修改 | `refreshSession()` 返回 `expires_in` |
| `app/web/src/stores/userStore.ts` | 修改 | 懒续期重构（过期时间记录 + 三分支） |
| `app/web/src/api/auth.ts` | 修改 | `LoginData.expires_in` + 透传 |
| `app/web/src/pages/login/index.tsx` | 修改 | 登录调用点透传 + OAuth payload 接收 |
| `app/web/src/stores/userStore.test.ts` | 修改 | 懒续期用例 |
| `app/web/src/api/authTransport.test.ts` | 创建 | refreshSession 返回值测试 |
| `AGENTS.md` / 规格文档 | 修改 | 配置表补行 / spec 两处修正 |

---

### 任务 1：后端 TTL 可配（config → jwt → gin 装配）

**文件：**
- 修改：`app/server/internal/config/config.go`（JWTKeyTTL 字段后 + JWTKeyTTLDuration 方法后）
- 修改：`app/server/internal/pkg/jwt.go:33-37`
- 修改：`app/server/server/gin.go`（约 :134 `service.SetBcryptCost` 旁）
- 测试：`app/server/internal/config/config_test.go`（已存在则追加，不存在则创建；先 `ls` 确认）

- [ ] **步骤 1：写失败的测试**

```go
func TestJWTTokenTTLDurations(t *testing.T) {
	cfg := &Config{}
	if got := cfg.JWTAccessTTLDuration(); got != 15*time.Minute {
		t.Errorf("default access ttl = %v, want 15m", got)
	}
	if got := cfg.JWTRefreshTTLDuration(); got != 7*24*time.Hour {
		t.Errorf("default refresh ttl = %v, want 168h", got)
	}
	cfg.JWTAccessTTL = "5m"
	cfg.JWTRefreshTTL = "72h"
	if got := cfg.JWTAccessTTLDuration(); got != 5*time.Minute {
		t.Errorf("custom access ttl = %v, want 5m", got)
	}
	if got := cfg.JWTRefreshTTLDuration(); got != 72*time.Hour {
		t.Errorf("custom refresh ttl = %v, want 72h", got)
	}
	cfg.JWTAccessTTL = "bogus"
	cfg.JWTRefreshTTL = "-1m"
	if got := cfg.JWTAccessTTLDuration(); got != 15*time.Minute {
		t.Errorf("invalid access ttl should fall back to 15m, got %v", got)
	}
	if got := cfg.JWTRefreshTTLDuration(); got != 7*24*time.Hour {
		t.Errorf("invalid refresh ttl should fall back to 168h, got %v", got)
	}
}
```

- [ ] **步骤 2：运行确认失败**

运行：`cd app/server && go test ./internal/config/ -run TestJWTTokenTTLDurations -v`
预期：FAIL（方法未定义）

- [ ] **步骤 3：实现**

config.go 字段区（`JWTKeyTTL` 之后）：

```go
	JWTAccessTTL  string `env:"JWT_ACCESS_TTL" envDefault:"15m"`
	JWTRefreshTTL string `env:"JWT_REFRESH_TTL" envDefault:"168h"`
```

config.go 方法区（`JWTKeyTTLDuration` 之后，同款回落模式）：

```go
// JWTAccessTTLDuration 解析 access token 有效期。
func (c *Config) JWTAccessTTLDuration() time.Duration {
	d, err := time.ParseDuration(strings.TrimSpace(c.JWTAccessTTL))
	if err != nil || d <= 0 {
		return 15 * time.Minute
	}
	return d
}

// JWTRefreshTTLDuration 解析 refresh token 有效期。
func (c *Config) JWTRefreshTTLDuration() time.Duration {
	d, err := time.ParseDuration(strings.TrimSpace(c.JWTRefreshTTL))
	if err != nil || d <= 0 {
		return 7 * 24 * time.Hour
	}
	return d
}
```

jwt.go（const→var，注释原样保留）：

```go
var AccessTokenTTL = 15 * time.Minute
var RefreshTokenTTL = 7 * 24 * time.Hour
```

同文件新增 helper：

```go
// AccessTokenExpiresIn access token 剩余有效期（秒），供响应体 expires_in 使用。
func AccessTokenExpiresIn() int64 {
	return int64(AccessTokenTTL / time.Second)
}
```

gin.go（`service.SetBcryptCost(cfg.BcryptCost)` 附近；确认 `GOSpeak/internal/pkg` 已在 import，无则补）：

```go
	pkg.AccessTokenTTL = cfg.JWTAccessTTLDuration()
	pkg.RefreshTokenTTL = cfg.JWTRefreshTTLDuration()
```

- [ ] **步骤 4：运行确认通过 + 全量编译**

运行：`cd app/server && go test ./internal/config/ -run TestJWTTokenTTLDurations -v && go build ./... && go test ./... -count=1`
预期：全部 PASS（const→var 不影响既有调用方）

- [ ] **步骤 5：Commit**

```bash
git add app/server/internal/config/config.go app/server/internal/config/config_test.go app/server/internal/pkg/jwt.go app/server/server/gin.go
git commit -m "feat(auth): access/refresh TTL 支持 JWT_ACCESS_TTL/JWT_REFRESH_TTL 配置"
```

---

### 任务 2：响应体下发 expires_in（auth 4 构造点 + refresh + handler 断言）

**文件：**
- 修改：`app/server/internal/service/auth_service.go`（:84 AuthResponse、:92 RefreshResponse、:123/:191/:404 构造点、:303 RefreshFromToken 返回）
- 修改：`app/server/internal/handler/auth_handler_test.go`
- 参考：`app/server/internal/service/oauth_service.go:217`（任务 3 处理其 handler 侧，本任务只改共享结构体）

- [ ] **步骤 1：写失败的测试**

在 `auth_handler_test.go` 的 login 成功用例与 refresh 成功用例中追加断言（按该文件现有取值方式，典型形态）：

```go
	// login 成功响应
	data := respBody["data"].(map[string]any)
	if data["expires_in"].(float64) != float64(900) {
		t.Errorf("expires_in = %v, want 900 (default 15m)", data["expires_in"])
	}
```

refresh 用例同样断言 `expires_in == 900`。若该表无 refresh 成功用例，按文件内 login 用例的既有搭法（直接调 handler + gin test context 或 httptest）补一个最小用例。

- [ ] **步骤 2：运行确认失败**

运行：`cd app/server && go test ./internal/handler/ -run TestLogin -v && go test ./internal/handler/ -run TestRefresh -v`
预期：FAIL（expires_in 字段缺失，类型断言失败）

- [ ] **步骤 3：实现**

auth_service.go 两个结构体加字段：

```go
type AuthResponse struct {
	Token              string     `json:"access_token"`
	RefreshToken       string     `json:"refresh_token"`
	User               model.User `json:"user"`
	NeedChangePassword bool       `json:"need_change_password"`
	ExpiresIn          int64      `json:"expires_in"`
}

type RefreshResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
}
```

4 个 `AuthResponse{` 构造点（:123 Login、:191 Register、:404 ResetPassword、oauth_service.go:217 buildAuthResponse）与 :303 `RefreshResponse{` 各追加：

```go
		ExpiresIn: pkg.AccessTokenExpiresIn(),
```

- [ ] **步骤 4：运行确认通过**

运行：`cd app/server && go test ./internal/handler/ ./internal/service/ -count=1 && go build ./...`
预期：PASS

- [ ] **步骤 5：Commit**

```bash
git add app/server/internal/service/auth_service.go app/server/internal/service/oauth_service.go app/server/internal/handler/auth_handler_test.go
git commit -m "feat(auth): login/register/reset/refresh/oauth 响应下发 expires_in"
```

---

### 任务 3：OAuth 回调 postMessage payload 携带 expires_in

**文件：**
- 修改：`app/server/internal/handler/oauth_handler.go:110`（`payload, _ := json.Marshal(map[string]bool{"ok": true})`）
- 测试：`app/server/internal/handler/oauth_handler_test.go`

- [ ] **步骤 1：写失败的测试**

在 `oauth_handler_test.go` 的 Callback 成功用例中追加（沿用该文件现有的 resp body 读取方式）：

```go
	if !strings.Contains(body.String(), `"expires_in":900`) {
		t.Errorf("oauth bridge payload should carry expires_in, got: %s", body.String())
	}
```

- [ ] **步骤 2：运行确认失败**

运行：`cd app/server && go test ./internal/handler/ -run TestOAuthCallback -v`
预期：FAIL

- [ ] **步骤 3：实现**

```go
	payload, _ := json.Marshal(map[string]any{"ok": true, "expires_in": pkg.AccessTokenExpiresIn()})
```

- [ ] **步骤 4：运行确认通过**

运行：`cd app/server && go test ./internal/handler/ -count=1 && go build ./...`
预期：PASS

- [ ] **步骤 5：Commit**

```bash
git add app/server/internal/handler/oauth_handler.go app/server/internal/handler/oauth_handler_test.go
git commit -m "feat(oauth): 回调 postMessage payload 携带 expires_in"
```

---

### 任务 4：前端 refreshSession 返回 expires_in

**文件：**
- 修改：`app/web/src/api/authTransport.ts:11-20`
- 创建：`app/web/src/api/authTransport.test.ts`
- **不 commit**（authTransport.ts 本身干净，但保持与前端其余任务同批入库——见计划头部约束）

- [ ] **步骤 1：写失败的测试**

```ts
import { describe, expect, it, vi } from "vitest";

const postMock = vi.fn();
vi.mock("axios", () => ({
	default: { create: () => ({ post: (...args: unknown[]) => postMock(...args) }) },
}));

describe("refreshSession", () => {
	it("透传响应体 expires_in", async () => {
		postMock.mockResolvedValue({ data: { code: 0, data: { expires_in: 900 } } });
		const { refreshSession } = await import("@/api/authTransport");
		await expect(refreshSession()).resolves.toBe(900);
	});

	it("响应体缺 expires_in：返回 null", async () => {
		postMock.mockResolvedValue({ data: { code: 0, data: {} } });
		const { refreshSession } = await import("@/api/authTransport");
		await expect(refreshSession()).resolves.toBeNull();
	});
});
```

（vi.mock 提升到顶层，`await import` 拿到 mock 注入后的模块；两条用例若共用模块实例需各自 `vi.resetModules()` 后再 import，实现时按 vitest 实际行为调整。）

- [ ] **步骤 2：运行确认失败**

运行：`cd app/web && pnpm vitest run src/api/authTransport.test.ts`
预期：FAIL（返回 undefined，非 900/null）

- [ ] **步骤 3：实现**

```ts
let pendingRefresh: Promise<number | null> | null = null;

/** 由 HttpOnly refresh cookie 静默续期 access token，返回服务端下发的 expires_in（秒）。 */
export async function refreshSession(): Promise<number | null> {
	if (!pendingRefresh) {
		pendingRefresh = rawAxios
			.post("/api/v1/auth/refresh_token")
			.then((resp) => {
				const data = resp.data?.data as { expires_in?: number } | undefined;
				return typeof data?.expires_in === "number" ? data.expires_in : null;
			})
			.finally(() => {
				pendingRefresh = null;
			});
	}
	return pendingRefresh;
}
```

- [ ] **步骤 4：运行确认通过**

运行：`cd app/web && pnpm vitest run src/api/authTransport.test.ts`
预期：PASS。既有调用点（apiClient.ts:89、userStore）`await refreshSession()` 忽略返回值，无需改动即可编译。

---

### 任务 5：userStore 懒续期重构

**文件：**
- 修改：`app/web/src/stores/userStore.ts`
- 修改：`app/web/src/stores/userStore.test.ts`
- **不 commit**（文件带前置批次的未提交改动）

- [ ] **步骤 1：写失败的测试**

在 `userStore.test.ts` 追加（沿用文件内 `vi.mock("@/api/auth")` / `vi.mock("@/api/authTransport")` 与 beforeEach 重置模式；过期时间通过公开 API `store.login(user, expiresIn)` 播种）：

```ts
	it("有会话且距过期 >60s：true 且零网络请求", async () => {
		await store.login(baseUser, 900);
		await expect(store.ensureSession()).resolves.toBe(true);
		expect(getProfileMock).not.toHaveBeenCalled();
		expect(refreshMock).not.toHaveBeenCalled();
	});

	it("临近过期(<60s)：先 refresh 再 profile，并记录新过期时间", async () => {
		await store.login(baseUser, 30);
		refreshMock.mockResolvedValue(900);
		getProfileMock.mockResolvedValue(baseUser);
		await expect(store.ensureSession()).resolves.toBe(true);
		expect(refreshMock).toHaveBeenCalledTimes(1);
		expect(getProfileMock).toHaveBeenCalledTimes(1);
		// 新过期时间已持久化：紧接的第二次调用不再发请求
		await expect(store.ensureSession()).resolves.toBe(true);
		expect(refreshMock).toHaveBeenCalledTimes(1);
	});

	it("无过期时间的存量会话：宽限窗口(10min)内验证过则放行", async () => {
		getProfileMock.mockResolvedValue(baseUser);
		await store.fetchProfile(); // 建立 lastVerifiedAt
		await store.clearAuth(); // 清掉 user？——注意：clearAuth 同时清 lastVerifiedAt
	});
	// ↑ 上条按实现语义修正：宽限分支要求 user() 存在。改为：
	it("无过期时间的存量会话：宽限窗口内验证过则放行", async () => {
		getProfileMock.mockResolvedValue(baseUser);
		await store.ensureSession(); // 探测路径：profile 成功 → lastVerifiedAt 建立
		getProfileMock.mockClear();
		await expect(store.ensureSession()).resolves.toBe(true);
		expect(getProfileMock).not.toHaveBeenCalled();
	});

	it("refreshSession 返回 expires_in：持久化到 localStorage", async () => {
		await store.login(baseUser, 30);
		refreshMock.mockResolvedValue(900);
		getProfileMock.mockResolvedValue(baseUser);
		await store.ensureSession();
		expect(Number(localStorage.getItem("gospeak_session_expires_at"))).toBeGreaterThan(Date.now());
	});

	it("logout 清除过期时间", async () => {
		await store.login(baseUser, 900);
		logoutApiMock.mockResolvedValue(undefined);
		await store.logout();
		expect(localStorage.getItem("gospeak_session_expires_at")).toBeNull();
	});
```

（`getProfileMock` / `refreshMock` / `logoutApiMock` / `baseUser` 为该测试文件既有 mock 句柄命名，实现时对齐文件内实际名称。）

- [ ] **步骤 2：运行确认失败**

运行：`cd app/web && pnpm vitest run src/stores/userStore.test.ts`
预期：新增用例 FAIL（现有实现总是探测 / 无过期时间逻辑）

- [ ] **步骤 3：实现 userStore.ts**

模块顶部（`ensureSessionPromise` 旁）：

```ts
const STORAGE_EXPIRES_AT = "gospeak_session_expires_at";
const EXPIRE_MARGIN_MS = 60_000;
const UNVERIFIED_GRACE_MS = 10 * 60_000;

let sessionExpiresAt: number | null = readExpiresAt();
let lastVerifiedAt = 0;

function readExpiresAt(): number | null {
	const raw = localStorage.getItem(STORAGE_EXPIRES_AT);
	if (!raw) return null;
	const n = Number(raw);
	return Number.isFinite(n) && n > 0 ? n : null;
}

function recordSessionExpiryAction(expiresIn: number | null | undefined) {
	if (typeof expiresIn === "number" && expiresIn > 0) {
		sessionExpiresAt = Date.now() + expiresIn * 1000;
		localStorage.setItem(STORAGE_EXPIRES_AT, String(sessionExpiresAt));
		return;
	}
	sessionExpiresAt = null;
	localStorage.removeItem(STORAGE_EXPIRES_AT);
}
```

`loginAction` 加参：`async function loginAction(u: UserInfo, expiresIn?: number)`，函数体末尾 `recordSessionExpiryAction(expiresIn)`（undefined 时清 key，无害）。
`clearAuthAction` 追加：`localStorage.removeItem(STORAGE_EXPIRES_AT); sessionExpiresAt = null; lastVerifiedAt = 0;`

`ensureSessionAction` 整体替换：

```ts
/**
 * 确保会话可用（过期时间驱动的懒续期）：
 * - 有过期时间且距过期 >60s：同步放行，零网络请求
 * - 临近过期/已过期：先 refresh（记录新 expires_in）再重验 profile
 * - 无过期时间（存量/被清）：宽限窗口内验证过则放行，否则探测 profile 补齐
 * - 无用户元数据：冷启动探测（cookie 可能仍有效）
 * - refresh 被限流(1017)不视为鉴权失败：有缓存会话则沿用
 * - refresh 返回真实 token 错误 → 清缓存返回 false
 */
async function ensureSessionAction(): Promise<boolean> {
	if (ensureSessionPromise) return ensureSessionPromise;
	ensureSessionPromise = (async () => {
		try {
			if (user()) {
				if (sessionExpiresAt) {
					if (Date.now() < sessionExpiresAt - EXPIRE_MARGIN_MS) return true;
					recordSessionExpiryAction(await refreshSession());
					const ok = await fetchProfileAction();
					if (ok) lastVerifiedAt = Date.now();
					return ok;
				}
				if (lastVerifiedAt && Date.now() - lastVerifiedAt < UNVERIFIED_GRACE_MS) {
					return true;
				}
			}
			if (await fetchProfileAction()) {
				lastVerifiedAt = Date.now();
				return true;
			}
			recordSessionExpiryAction(await refreshSession());
			const ok = await fetchProfileAction();
			if (ok) lastVerifiedAt = Date.now();
			return ok;
		} catch (e) {
			if (isRateLimitedError(e) && user()) return true;
			await clearAuthAction();
			return false;
		} finally {
			ensureSessionPromise = null;
		}
	})();
	return ensureSessionPromise;
}
```

store 对象加导出：`recordSessionExpiry: recordSessionExpiryAction,`

- [ ] **步骤 4：运行确认通过**

运行：`cd app/web && pnpm vitest run src/stores/userStore.test.ts`
预期：新旧用例全 PASS（旧用例「缓存用户 + profile 成功：true」在新逻辑下走探测路径仍成立）

---

### 任务 6：调用点透传（api/auth.ts + login 页 + OAuth 接收）

**文件：**
- 修改：`app/web/src/api/auth.ts`（LoginData 接口，:20 附近）
- 修改：`app/web/src/pages/login/index.tsx`（:58 completeOAuthLogin、:85-90 message handler、:206/:210/:570 三处 `userStore.login(data.user)`）
- **不 commit**

- [ ] **步骤 1：实现**

auth.ts `LoginData` 加字段：

```ts
export interface LoginData {
	user: BackendUser;
	need_change_password: boolean;
	expires_in?: number;
}
```

login/index.tsx：
- 三处 `await userStore.login(data.user)` → `await userStore.login(data.user, data.expires_in)`；
- message handler payload 类型 `{ type?: string; ok?: boolean; expires_in?: number }`，`if (data.ok) completeOAuthLogin(data.expires_in);`
- `completeOAuthLogin(expiresIn?: number)`，async 体首行 `userStore.recordSessionExpiry(expiresIn);`（OAuth 登录后 user() 为空，本次仍走探测加载 user，但过期时间已记录，后续导航进入零 RTT 节奏）。

- [ ] **步骤 2：类型检查 + 全量前端验证**

运行：`cd app/web && pnpm exec tsc --noEmit && pnpm test && pnpm exec biome check src/api/authTransport.ts src/stores/userStore.ts src/api/auth.ts src/pages/login/index.tsx`
预期：tsc 无错、测试全绿（现有 253+ 新增）、biome 无新增问题

---

### 任务 7：文档修正 + 全量回归 + 测试日志

**文件：**
- 修改：`AGENTS.md`（JWT 配置表）
- 修改：`docs/superpowers/specs/2026-08-29-session-proactive-refresh-design.md`（两处修正，见下）
- 创建：`agent_test_logs/session-proactive-refresh-test-2026-08-29.md`

- [ ] **步骤 1：AGENTS.md JWT 配置表补两行**

```markdown
| `JWT_ACCESS_TTL` | `15m` | access token 有效期（login/refresh 响应 `expires_in` 同源） |
| `JWT_REFRESH_TTL` | `168h` | refresh token 有效期 |
```

- [ ] **步骤 2：规格修正（与实现对齐）**

1. 后端改动 §1「非法值启动时报错」→「非法值/非正值回落默认值（同 `JWTKeyTTLDuration` 模式，不阻塞启动）」。
2. 前端改动 §2 补充存量桥接机制：「无过期时间的存量会话以 `lastVerifiedAt` 宽限窗口（10 分钟）过渡：窗口内同步放行，过期后触发一次 refresh 换取真实 `expires_in`」；测试计划同步补宽限窗口用例。

- [ ] **步骤 3：全量回归**

运行：
```bash
cd app/server && go build ./... && go test ./... -count=1
cd app/web && pnpm test && pnpm build
```
预期：Go 全绿；web 测试全绿；vite build 成功且分包不变。

- [ ] **步骤 4：写测试日志**

按 `agent_test_logs/AGENTS.md` 规范输出 `session-proactive-refresh-test-2026-08-29.md`（✅/❌ 标识各验证项）。

- [ ] **步骤 5：Commit（仅文档）**

```bash
git add AGENTS.md docs/superpowers/specs/2026-08-29-session-proactive-refresh-design.md agent_test_logs/session-proactive-refresh-test-2026-08-29.md
git commit -m "docs: 补充 JWT TTL 配置项与 expires_in 懒续期测试记录"
```

---

## 自检记录

- **规格覆盖度**：TTL 可配（任务 1）、expires_in 下发 login/register/reset/refresh（任务 2）、OAuth postMessage（任务 3）、refreshSession 返回值（任务 4）、懒续期三分支 + 存量宽限桥（任务 5）、调用点透传 + OAuth 接收（任务 6）、配置文档 + 回归（任务 7）。规格「非目标」项（定时器/多标签/guest/bot）无对应任务，正确。
- **占位符扫描**：任务 2 步骤 1 的断言依赖 auth_handler_test.go 现有取值形态（执行时对齐）；任务 5 的 mock 句柄名对齐现有文件——均为「对齐既有代码」而非缺失设计，已注明对齐点。
- **类型一致性**：`refreshSession(): Promise<number | null>`（任务 4）↔ `recordSessionExpiryAction(number | null | undefined)`（任务 5）↔ `completeOAuthLogin(expiresIn?: number)`（任务 6）；`pkg.AccessTokenExpiresIn() int64` 贯穿任务 1/2/3。
