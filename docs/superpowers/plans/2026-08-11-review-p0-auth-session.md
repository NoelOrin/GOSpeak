# Review P0: Auth & Session Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 修复双线程 review 整合报告中认证/会话子系统的 P0 问题：refresh family 轮换、Logout 吊销 HttpOnly refresh cookie、登录 401 契约同步。

**Architecture:** 以 `AuthService.RefreshFromToken` 为修复核心：正常刷新轮换到新 family，旧 token（含历史无 family token）按 family/JTI 标记已用，杜绝“正常刷新一次后整族吊销”与“旧 token 无限重放”；Logout 从 HttpOnly cookie 读取 refresh token 并吊销；集成测试契约同步到 401。

**Tech Stack:** Go 1.26 + Gin + GORM + NATS（app/server），TypeScript + Vitest（test/）。

---

## 范围说明

本计划只覆盖认证/会话子系统：

- refresh family 轮换。
- Logout 吊销 HttpOnly refresh cookie。
- 登录 401 契约测试同步。

域权限/RBAC 已拆到独立计划 `2026-08-11-review-p0-domain-rbac.md`；NATS/WAL、SFU/媒体、Bot/前端、存储/迁移按 Scope Check 拆为后续子计划，不在本文件实现。

## 已核实的现状（避免重复实现）

- `pkg/response.go:49` 已把 `INVALID_PASSWORD`/`USER_NOT_FOUND` 映射为 HTTP 401；残留问题只是 `test/auth/auth.test.ts:28` 仍断言 400。
- `auth_handler.go` 的 `GetRefreshToken` 已支持从 `RefreshName` cookie 读取；`Logout` 尚未读取该 cookie。
- `RefreshFromToken` 当前把旧 family 标记 used 后又用同一 family 签发新 token，正常第二次刷新必然失败；历史 token 无 family 时可无限重放。

### Task 1: Refresh family 轮换修复

**Files:**
- Modify: `app/server/internal/service/auth_service.go:228-265`
- Test: `app/server/internal/service/auth_service_test.go`

背景：`RefreshFromToken` 目前把旧 family 标记 used 后又用**同一 family** 签发新 refresh token，导致用户正常刷新第二次必然被判重放并吊销整族；同时历史 token 缺 family 时可无限重放。修复后：正常刷新轮换到新 family；旧 token（含历史无 family token）以旧 family 或 JTI 作为已用标记，重放即拒。

- [ ] **Step 1: 写失败测试**

在 `app/server/internal/service/auth_service_test.go` 末尾追加：

```go
func TestRefreshFromToken_NewTokenCanRefreshAgain(t *testing.T) {
	svc := setupAuthServiceTest(t)
	refresh, err := pkg.GenerateRefreshToken("alice", "Alice", "uuid-alice", "user", 1)
	if err != nil {
		t.Fatalf("GenerateRefreshToken: %v", err)
	}
	first, err := svc.RefreshFromToken(refresh)
	if err != nil {
		t.Fatalf("first refresh: %v", err)
	}
	if first.RefreshToken == "" || first.RefreshToken == refresh {
		t.Fatal("expected a new rotated refresh token")
	}
	second, err := svc.RefreshFromToken(first.RefreshToken)
	if err != nil {
		t.Fatalf("second refresh with rotated token must succeed, got %v", err)
	}
	if second.RefreshToken == "" || second.RefreshToken == first.RefreshToken {
		t.Fatal("expected another rotated refresh token")
	}
}

`GenerateRefreshTokenWithFamily` 的 `family` 参数为空时会自动生成新 family，因此无法构造“无 family 的 token”。测试直接构造 Claims 并签名：

```go
func TestRefreshFromToken_LegacyTokenWithoutFamilyCannotReplay(t *testing.T) {
	svc := setupAuthServiceTest(t)
	claims := pkg.Claims{
		Username:      "alice",
		UserUUID:      "uuid-alice",
		Role:          "user",
		TokenVersion:  1,
		TokenType:     pkg.RefreshTokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(7 * 24 * time.Hour)),
			ID:        "legacy-jti-1",
		},
	}
	refresh, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(authstate.GetSigningKey())
	if err != nil {
		t.Fatalf("sign legacy refresh token: %v", err)
	}
	if _, err := svc.RefreshFromToken(refresh); err != nil {
		t.Fatalf("first refresh: %v", err)
	}
	if _, err := svc.RefreshFromToken(refresh); err == nil {
		t.Fatal("legacy refresh token replay must be rejected")
	}
}
```

测试文件需要新增 import：

```go
import (
	"time"

	"GOSpeak/internal/authstate"

	"github.com/golang-jwt/jwt/v5"
)
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd /Users/noelorin/GOSpeak/app/server && go test ./internal/service/ -run 'TestRefreshFromToken_NewTokenCanRefreshAgain|TestRefreshFromToken_LegacyTokenWithoutFamilyCannotReplay' -v`

Expected: 两个测试 FAIL；第一个在第二次刷新处失败（family 已被 used），第二个在重放处失败（无 family token 不受标记约束）。

- [ ] **Step 3: 修改 `auth_service.go`**

将 `RefreshFromToken` 中以下代码：

```go
	family := claims.RefreshFamily
	if family == "" {
		family, err = pkg.GenerateRefreshFamily()
		if err != nil {
			return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
		}
	}
```

替换为：

```go
	family := claims.RefreshFamily
	if family == "" {
		// 历史 token 没有 family：用 JTI 作为一次性标记，防止无限重放。
		family = claims.ID
	}
```

再将签发新 refresh token 的代码：

```go
	nextRefresh, err := pkg.GenerateRefreshTokenWithFamily(user.Name, user.DisplayName, user.UUID, user.Role, user.TokenVersion, family)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
```

替换为：

```go
	nextFamily, err := pkg.GenerateRefreshFamily()
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	nextRefresh, err := pkg.GenerateRefreshTokenWithFamily(user.Name, user.DisplayName, user.UUID, user.Role, user.TokenVersion, nextFamily)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
```

同时更新函数顶部注释，去掉“成功时返回同 family refresh_token”的旧语义。

- [ ] **Step 4: 运行测试确认通过**

Run: `cd /Users/noelorin/GOSpeak/app/server && go test ./internal/service/ -run 'TestRefreshFromToken' -v`

Expected: `TestRefreshFromToken_AcceptsRefreshToken`、`TestRefreshFromToken_RotatesAndDetectsReuse`、`TestRefreshFromToken_NewTokenCanRefreshAgain`、`TestRefreshFromToken_LegacyTokenWithoutFamilyCannotReplay`、`TestRefreshFromToken_RejectsNonRefreshTokens` 全部 PASS。

- [ ] **Step 5: 提交**

```bash
cd /Users/noelorin/GOSpeak/app/server
git add internal/service/auth_service.go internal/service/auth_service_test.go
git commit -m "fix(auth): rotate refresh family on every refresh"
```

---

### Task 2: Logout 吊销 HttpOnly refresh cookie

**Files:**
- Modify: `app/server/internal/handler/auth_handler.go:125-144`
- Test: `app/server/internal/handler/auth_handler_test.go`

背景：浏览器 refresh token 在 HttpOnly cookie 中，前端无法放入 body；当前 Logout 只清 cookie 并把空 token 传给 `AuthService.Logout`，被盗 refresh token 仍可续期。修复后 Logout 优先从 cookie 读取 refresh token 并吊销。

- [ ] **Step 1: 写失败测试**

在 `app/server/internal/handler/auth_handler_test.go` 末尾追加。先补 import：

```go
import (
	"net/http"
	"time"

	"GOSpeak/internal/authstate"
)
```

再追加测试：

```go
type fakeAuthBackend struct {
	blacklisted map[string]time.Time
}

func (f *fakeAuthBackend) BlacklistToken(jti string, remaining time.Duration) error {
	f.blacklisted[jti] = time.Now().Add(remaining)
	return nil
}
func (f *fakeAuthBackend) IsBlacklisted(jti string) bool { _, ok := f.blacklisted[jti]; return ok }
func (f *fakeAuthBackend) IsBlacklistedErr(jti string) (bool, error) { return f.IsBlacklisted(jti), nil }
func (f *fakeAuthBackend) GetSigningKey() (string, bool, error) { return "", false, nil }
func (f *fakeAuthBackend) SetSigningKey(string, int64) error { return nil }
func (f *fakeAuthBackend) UpdateSigningKey(string, int64) error { return nil }
func (f *fakeAuthBackend) GetCreatedAt() (int64, bool, error) { return 0, false, nil }
func (f *fakeAuthBackend) AddHistoryKey(string) error { return nil }
func (f *fakeAuthBackend) HistoryKeys() []string { return nil }
func (f *fakeAuthBackend) MarkRefreshFamilyUsed(string, time.Duration) (bool, error) { return true, nil }
func (f *fakeAuthBackend) IsRefreshFamilyUsed(string) (bool, error) { return false, nil }
func (f *fakeAuthBackend) RevokeRefreshFamily(string) error { return nil }
func (f *fakeAuthBackend) Backend() string { return "fake" }

func TestAuthHandler_Logout_RevokesRefreshTokenFromCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := newTestAuthHandler(t)
	backend := &fakeAuthBackend{blacklisted: map[string]time.Time{}}
	authstate.SetBackend(backend)
	t.Cleanup(func() { authstate.SetBackend(nil) })

	refresh, err := pkg.GenerateRefreshToken("alice", "Alice", "uuid-alice", "user", 1)
	if err != nil {
		t.Fatalf("GenerateRefreshToken: %v", err)
	}
	claims, err := pkg.ParseToken(refresh)
	if err != nil {
		t.Fatalf("ParseToken: %v", err)
	}

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("claims", claims)
		c.Next()
	})
	router.POST("/logout", h.Logout)

	req := httptest.NewRequest(http.MethodPost, "/logout", nil)
	req.AddCookie(&http.Cookie{Name: h.cookie.RefreshName, Value: refresh})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if code := intCode(resp["code"]); code != 0 {
		t.Fatalf("expected code 0, got %d: %s", code, resp["msg"])
	}
	if revoked, _ := authstate.IsBlacklistedErr(claims.ID); !revoked {
		t.Fatal("refresh token from cookie must be blacklisted")
	}
}
```

`fakeAuthBackend` 实现了 `authstate.Backend` 的全部方法；`pkg` 与 `httptest` 已在文件顶部 import。

- [ ] **Step 2: 运行测试确认失败**

Run: `cd /Users/noelorin/GOSpeak/app/server && go test ./internal/handler/ -run TestAuthHandler_Logout_RevokesRefreshTokenFromCookie -v`

Expected: FAIL，响应虽为 code 0，但 `authstate.IsBlacklistedErr` 返回 false（Logout 收到的 refresh token 为空）。

- [ ] **Step 3: 修改 `auth_handler.go` 的 Logout**

将 `Logout` 函数整体替换为：

```go
func (h *AuthHandler) Logout(c *gin.Context) {
	var accessClaims *pkg.Claims
	if v, ok := c.Get("claims"); ok {
		accessClaims, _ = v.(*pkg.Claims)
	}

	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	_ = c.ShouldBindJSON(&req)
	if req.RefreshToken == "" {
		if cookie, err := c.Request.Cookie(h.cookie.RefreshName); err == nil && cookie.Value != "" {
			req.RefreshToken = cookie.Value
		}
	}

	// 无论服务端黑名单是否成功，浏览器侧 Cookie 都立即失效。
	h.cookie.Clear(c)

	if err := h.authService.Logout(accessClaims, req.RefreshToken); err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, nil)
}
```

调整后 Logout 顺序：读 cookie/body → `Clear` → `authService.Logout`。

- [ ] **Step 4: 运行测试确认通过**

Run: `cd /Users/noelorin/GOSpeak/app/server && go test ./internal/handler/ -run 'TestAuthHandler_Logout|TestAuthHandler_Login_SetsHttpOnlyCookies|TestAuthHandler_GetRefreshToken_ReadsRefreshCookie' -v`

Expected: 全部 PASS。

- [ ] **Step 5: 提交**

```bash
cd /Users/noelorin/GOSpeak/app/server
git add internal/handler/auth_handler.go internal/handler/auth_handler_test.go
git commit -m "fix(auth): revoke HttpOnly refresh cookie on logout"
```

---

### Task 3: 同步登录 401 契约测试

**Files:**
- Modify: `test/auth/auth.test.ts:28`

背景：后端已把 `INVALID_PASSWORD`/`USER_NOT_FOUND` 映射为 HTTP 401（`pkg/response.go:49`），集成测试仍断言 400，导致 CI 必挂。

- [ ] **Step 1: 更新断言**

将 `test/auth/auth.test.ts` 中：

```ts
  expect(result.status).toBe(400);
  expect(result.code).toBe(1010);
```

替换为：

```ts
  expect(result.status).toBe(401);
  expect(result.code).toBe(1010);
```

- [ ] **Step 2: 运行测试确认通过**

Run: `cd /Users/noelorin/GOSpeak && pnpm vitest run test/auth/auth.test.ts`

Expected: 该文件全部 PASS（需要本地已启动后端；若后端未启动，先运行 `cd app/server && go run . server` 再执行）。

- [ ] **Step 3: 提交**

```bash
cd /Users/noelorin/GOSpeak
git add test/auth/auth.test.ts
git commit -m "test(auth): sync login failure contract to 401"
```

---

### Task 4: 认证子计划全量验证

**Files:**
- 无新增代码

- [ ] **Step 1: 运行 Go 全量测试**

Run: `cd /Users/noelorin/GOSpeak/app/server && go test ./...`

Expected: 全部 PASS（Task 1-2 的服务与 handler 改动不影响其他包）。

- [ ] **Step 2: 运行集成契约测试**

Run: `cd /Users/noelorin/GOSpeak && pnpm vitest run test/auth/auth.test.ts`

Expected: PASS（需本地后端运行；先 `cd app/server && go run . server`）。

- [ ] **Step 3: 静态检查**

Run: `cd /Users/noelorin/GOSpeak/app/server && gofmt -l . && go vet ./...`

Expected: 无未格式化文件，vet 无输出。

- [ ] **Step 4: 收尾提交**

```bash
cd /Users/noelorin/GOSpeak
git status --short
git add app/server/internal/service/auth_service.go app/server/internal/service/auth_service_test.go app/server/internal/handler/auth_handler.go app/server/internal/handler/auth_handler_test.go test/auth/auth.test.ts
git commit -m "chore: finalize auth session fixes"
```

`git status --short` 确认仅本计划文件后执行；若包含用户未提交改动，只 add 本计划修改过的文件，绝不动用户改动。

---

## 后续子计划（不在本文件实现）

- Domain RBAC：`2026-08-11-review-p0-domain-rbac.md`。
- 认证残留：cookie Secure 的 `X-Forwarded-Proto` 判定、token_type 旧 token 兼容窗口、blacklist fail-open。
- NATS/WAL、SFU/媒体、Bot/前端、存储/迁移、契约/CI 见整合报告。

## Self-Review

1. **Spec coverage**：认证/会话 P0 中 refresh family、Logout cookie、401 契约三项全部覆盖；cookie Secure 与 token_type 窗口按范围拆分到后续认证子计划。
2. **Placeholder scan**：所有代码步骤均给出完整可编译代码；未出现 TBD/TODO/“适当处理”式占位。
3. **Type consistency**：Task 1 使用现有 `pkg.Claims`、`jwt.RegisteredClaims`、`authstate.GetSigningKey`；Task 2 的 `fakeAuthBackend` 完整实现 `authstate.Backend` 接口，方法签名与现有 backend 一致。
