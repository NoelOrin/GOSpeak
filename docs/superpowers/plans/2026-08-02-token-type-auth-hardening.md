# TokenType 鉴权收紧 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 收紧 HTTP 与 WebSocket 的 TokenType 鉴权，只允许 access/bot 用于 HTTP，只允许 subprotocol ws-ticket 用于 WS。

**Architecture:** `VerifyToken` 保持 HTTP 入口但内部增加 TokenType 白名单；新增 `VerifyWSTicket` 供 WebSocket 专用。WS 握手移除 header/cookie 凭据来源，只从 `Sec-WebSocket-Protocol` 提取 ticket。

**Tech Stack:** Go、Gin、nhooyr/websocket、Go 标准测试。

**Commit policy:** 根据仓库 `AGENTS.md`，本计划不自动提交；所有步骤只改文件、跑测试并汇报差异。

---

### Task 1: 为 middleware TokenType 策略写失败测试

**Files:**
- Modify: `app/server/internal/middleware/auth_test.go`

- [ ] **Step 1: 添加测试**

在 `app/server/internal/middleware/auth_test.go` 追加：

```go
func TestVerifyToken_AcceptsAccessAndBot(t *testing.T) {
	access, err := pkg.GenerateToken("alice", "Alice", "uuid-alice", "user", 1)
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}
	if _, code := VerifyToken(access); code != pkg.SUCCESS {
		t.Fatalf("expected access token accepted, got code=%d", code)
	}

	bot, err := pkg.GenerateBotToken("bot", "Bot", "uuid-bot", "bot", 1, []string{"signal:kick"}, false)
	if err != nil {
		t.Fatalf("GenerateBotToken: %v", err)
	}
	if _, code := VerifyToken(bot); code != pkg.SUCCESS {
		t.Fatalf("expected bot token accepted, got code=%d", code)
	}
}

func TestVerifyToken_RejectsRefreshAndWSTicket(t *testing.T) {
	refresh, err := pkg.GenerateRefreshToken("alice", "Alice", "uuid-alice", "user", 1)
	if err != nil {
		t.Fatalf("GenerateRefreshToken: %v", err)
	}
	ticket, err := pkg.GenerateWSTicket("alice", "Alice", "uuid-alice", "user", 1)
	if err != nil {
		t.Fatalf("GenerateWSTicket: %v", err)
	}

	for _, tc := range []struct {
		name  string
		token string
	}{
		{name: "refresh", token: refresh},
		{name: "ws-ticket", token: ticket},
	} {
		t.Run(tc.name, func(t *testing.T) {
			claims, code := VerifyToken(tc.token)
			if code != pkg.TOKEN_WRONG {
				t.Fatalf("expected TOKEN_WRONG, got code=%d", code)
			}
			if claims != nil {
				t.Fatalf("expected nil claims, got %#v", claims)
			}
		})
	}
}

func TestVerifyWSTicket_AcceptsWSTicket(t *testing.T) {
	ticket, err := pkg.GenerateWSTicket("alice", "Alice", "uuid-alice", "user", 1)
	if err != nil {
		t.Fatalf("GenerateWSTicket: %v", err)
	}
	claims, code := VerifyWSTicket(ticket)
	if code != pkg.SUCCESS {
		t.Fatalf("expected ws ticket accepted, got code=%d", code)
	}
	if claims == nil || claims.Username != "alice" {
		t.Fatalf("expected alice claims, got %#v", claims)
	}
}

func TestVerifyWSTicket_RejectsNonWSTicket(t *testing.T) {
	access, err := pkg.GenerateToken("alice", "Alice", "uuid-alice", "user", 1)
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}
	bot, err := pkg.GenerateBotToken("bot", "Bot", "uuid-bot", "bot", 1, []string{"signal:kick"}, false)
	if err != nil {
		t.Fatalf("GenerateBotToken: %v", err)
	}
	refresh, err := pkg.GenerateRefreshToken("alice", "Alice", "uuid-alice", "user", 1)
	if err != nil {
		t.Fatalf("GenerateRefreshToken: %v", err)
	}

	for _, tc := range []struct {
		name  string
		token string
	}{
		{name: "access", token: access},
		{name: "bot", token: bot},
		{name: "refresh", token: refresh},
	} {
		t.Run(tc.name, func(t *testing.T) {
			claims, code := VerifyWSTicket(tc.token)
			if code != pkg.TOKEN_WRONG {
				t.Fatalf("expected TOKEN_WRONG, got code=%d", code)
			}
			if claims != nil {
				t.Fatalf("expected nil claims, got %#v", claims)
			}
		})
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd app/server && go test ./internal/middleware -run 'TestVerify' -count=1`
Expected: FAIL；`VerifyWSTicket` 未定义，且 `VerifyToken` 会接受 refresh/ws-ticket。

### Task 2: 实现 middleware TokenType 校验

**Files:**
- Modify: `app/server/internal/middleware/auth.go:39-57`

- [ ] **Step 1: 替换 `VerifyToken` 并新增专用入口**

将现有 `VerifyToken` 函数体替换为：

```go
// VerifyToken 校验 HTTP 可用的 access/bot token。
func VerifyToken(tokenStr string) (*pkg.Claims, pkg.ErrCode) {
	return verifyToken(tokenStr, pkg.AccessTokenType, pkg.BotTokenType)
}

// VerifyWSTicket 校验 WebSocket 短时 ticket。
func VerifyWSTicket(tokenStr string) (*pkg.Claims, pkg.ErrCode) {
	claims, code := verifyToken(tokenStr, pkg.WSTicketType)
	if code != pkg.SUCCESS {
		return nil, code
	}
	if pkg.WSTicketExpired(claims) {
		return nil, pkg.TOKEN_EXPIRED
	}
	return claims, pkg.SUCCESS
}

func verifyToken(tokenStr string, allowedTypes ...string) (*pkg.Claims, pkg.ErrCode) {
	claims, err := pkg.ParseToken(tokenStr)
	if err != nil {
		return nil, pkg.TOKEN_WRONG
	}
	if !tokenTypeAllowed(claims, allowedTypes...) {
		return nil, pkg.TOKEN_WRONG
	}
	if pkg.IsTokenExpired(claims) {
		return nil, pkg.TOKEN_EXPIRED
	}
	if redis.IsBlacklisted(claims.ID) {
		return nil, pkg.TOKEN_REVOKED
	}
	if tokenVersionCheck != nil && claims.UserUUID != "" {
		currentVersion, err := tokenVersionCheck.GetTokenVersionByUUID(claims.UserUUID)
		if err != nil {
			return nil, pkg.INTERNAL_ERROR
		}
		if currentVersion != claims.TokenVersion {
			return nil, pkg.TOKEN_REVOKED
		}
	}
	return claims, pkg.SUCCESS
}

func tokenTypeAllowed(claims *pkg.Claims, allowedTypes ...string) bool {
	if claims == nil || claims.TokenType == "" {
		return false
	}
	for _, allowed := range allowedTypes {
		if claims.TokenType == allowed {
			return true
		}
	}
	return false
}
```

- [ ] **Step 2: 运行 middleware 测试确认通过**

Run: `cd app/server && go test ./internal/middleware -run 'TestVerify' -count=1`
Expected: PASS。

### Task 3: 为 WS 只接受 subprotocol ticket 写失败测试

**Files:**
- Modify: `app/server/internal/ws/upgrader_test.go`

- [ ] **Step 1: 更新提取测试并补充 ServeHTTP 拒绝用例**

将 `app/server/internal/ws/upgrader_test.go` 内容替换为：

```go
package ws

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"GOSpeak/internal/pkg"
)

func TestExtractToken_Subprotocol(t *testing.T) {
	r := httptest.NewRequest("GET", "/ws", nil)
	r.Header.Set("Sec-WebSocket-Protocol", "gospeak, ws-ticket-123")

	token, fromSubprotocol := extractToken(r)
	if token != "ws-ticket-123" || !fromSubprotocol {
		t.Fatalf("expected subprotocol ticket, got token=%q fromSubprotocol=%v", token, fromSubprotocol)
	}
}

func TestExtractToken_HeaderRejected(t *testing.T) {
	r := httptest.NewRequest("GET", "/ws", nil)
	r.Header.Set("Authorization", "Bearer test-token-123")

	token, fromSubprotocol := extractToken(r)
	if token != "" || fromSubprotocol {
		t.Fatalf("expected header token rejected, got token=%q fromSubprotocol=%v", token, fromSubprotocol)
	}
}

func TestExtractToken_CookieRejected(t *testing.T) {
	r := httptest.NewRequest("GET", "/ws", nil)
	r.AddCookie(&http.Cookie{Name: "gospeak_token", Value: "cookie-token-456"})

	token, fromSubprotocol := extractToken(r)
	if token != "" || fromSubprotocol {
		t.Fatalf("expected cookie token rejected, got token=%q fromSubprotocol=%v", token, fromSubprotocol)
	}
}

func TestExtractToken_QueryRejected(t *testing.T) {
	r := httptest.NewRequest("GET", "/ws?token=query-token-789", nil)

	token, _ := extractToken(r)
	if token != "" {
		t.Fatalf("query token must not be accepted, got %q", token)
	}
}

func TestExtractToken_Empty(t *testing.T) {
	r := httptest.NewRequest("GET", "/ws", nil)

	token, _ := extractToken(r)
	if token != "" {
		t.Fatalf("expected empty token, got %q", token)
	}
}

func TestExtractToken_PlainHeaderRejected(t *testing.T) {
	r := httptest.NewRequest("GET", "/ws", nil)
	r.Header.Set("Authorization", "raw-token-no-bearer")

	token, _ := extractToken(r)
	if token != "" {
		t.Fatalf("expected plain header token rejected, got %q", token)
	}
}

func TestExtractToken_BearerAndCookieRejected(t *testing.T) {
	r := httptest.NewRequest("GET", "/ws", nil)
	r.Header.Set("Authorization", "Bearer header-token")
	r.AddCookie(&http.Cookie{Name: "gospeak_token", Value: "cookie-token"})

	token, _ := extractToken(r)
	if token != "" {
		t.Fatalf("expected header/cookie tokens rejected, got %q", token)
	}
}

func TestUpgrader_ServeHTTP_RejectsHeaderAccessToken(t *testing.T) {
	access, err := pkg.GenerateToken("alice", "Alice", "uuid-alice", "user", 1)
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}

	upgrader := NewUpgrader(UpgraderConfig{})
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/ws", nil)
	r.Header.Set("Authorization", "Bearer "+access)

	upgrader.ServeHTTP(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for header access token, got %d", w.Code)
	}
}

func TestUpgrader_ServeHTTP_RejectsCookieAccessToken(t *testing.T) {
	access, err := pkg.GenerateToken("alice", "Alice", "uuid-alice", "user", 1)
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}

	upgrader := NewUpgrader(UpgraderConfig{})
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/ws", nil)
	r.AddCookie(&http.Cookie{Name: "gospeak_token", Value: access})

	upgrader.ServeHTTP(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for cookie access token, got %d", w.Code)
	}
}

func TestUpgrader_ServeHTTP_RejectsSubprotocolAccessToken(t *testing.T) {
	access, err := pkg.GenerateToken("alice", "Alice", "uuid-alice", "user", 1)
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}

	upgrader := NewUpgrader(UpgraderConfig{})
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/ws", nil)
	r.Header.Set("Sec-WebSocket-Protocol", "gospeak, "+access)

	upgrader.ServeHTTP(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for subprotocol access token, got %d", w.Code)
	}
}
```

- [ ] **Step 2: 运行 WS 测试确认失败**

Run: `cd app/server && go test ./internal/ws -run 'TestExtractToken|TestUpgrader_ServeHTTP' -count=1`
Expected: FAIL；`extractToken` 仍从 header/cookie 返回 token，且 ServeHTTP 对 header/cookie access token 不返回 401。

### Task 4: 实现 WS 只接受 subprotocol ticket

**Files:**
- Modify: `app/server/internal/ws/upgrader.go:40-56`
- Modify: `app/server/internal/ws/upgrader.go:97-110`

- [ ] **Step 1: 将 `extractToken` 改为仅读取 subprotocol**

替换 `extractToken` 为：

```go
// extractToken 从 Sec-WebSocket-Protocol 提取短时 WS ticket。
func extractToken(r *http.Request) (string, bool) {
	for _, protocol := range strings.Split(r.Header.Get("Sec-WebSocket-Protocol"), ",") {
		protocol = strings.TrimSpace(protocol)
		if protocol != "" && protocol != "gospeak" {
			return protocol, true
		}
	}
	return "", false
}
```

- [ ] **Step 2: 让 `ServeHTTP` 只接受 subprotocol 并调用 `VerifyWSTicket`**

替换 `ServeHTTP` 中鉴权片段为：

```go
	tokenStr, fromSubprotocol := extractToken(r)
	if tokenStr == "" || !fromSubprotocol {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	claims, code := middleware.VerifyWSTicket(tokenStr)
	if code != pkg.SUCCESS {
		log.Printf("[ws] upgrade rejected: code=%s client=%s", code.String(), r.RemoteAddr)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
```

- [ ] **Step 3: 运行 WS 测试确认通过**

Run: `cd app/server && go test ./internal/ws -run 'TestExtractToken|TestUpgrader_ServeHTTP|TestUpgrader_E2E' -count=1`
Expected: PASS。

### Task 5: 全量目标包验证

**Files:**
- 无代码改动

- [ ] **Step 1: 运行目标包测试**

Run: `cd app/server && go test ./internal/middleware ./internal/ws -count=1`
Expected: PASS。

- [ ] **Step 2: 运行关联包回归**

Run: `cd app/server && go test ./internal/pkg ./internal/service ./internal/signal -count=1`
Expected: PASS；本改动不改变签发/服务层行为。

### Task 6: 汇报差异

**Files:**
- `app/server/internal/middleware/auth.go`
- `app/server/internal/middleware/auth_test.go`
- `app/server/internal/ws/upgrader.go`
- `app/server/internal/ws/upgrader_test.go`

- [ ] **Step 1: 检查最终差异**

Run: `git diff -- app/server/internal/middleware/auth.go app/server/internal/middleware/auth_test.go app/server/internal/ws/upgrader.go app/server/internal/ws/upgrader_test.go docs/superpowers/specs/2026-08-02-token-type-auth-hardening-design.md docs/superpowers/plans/2026-08-02-token-type-auth-hardening.md`
Expected: 显示上述文件改动；不包含无关文件。

- [ ] **Step 2: 汇报结果**

不执行 `git commit`。汇报修改文件、验证命令和结果、已知范围外事项（`/system/stream` 后续处理）。
