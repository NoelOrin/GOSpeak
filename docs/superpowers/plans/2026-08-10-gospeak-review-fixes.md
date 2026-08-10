# GOSpeak Review Fixes Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 修复 GOSpeak 6 轮 caveman-review 聚合出的高优先级问题（除用户明确排除的「强制校验 JWT 密钥」外），并为已修复项补齐回归测试。

**Architecture:** 分 9 个独立任务推进：认证错误语义统一、OAuth state 加固、Domain 中间件 body 限流、平台级私密房间历史访问控制、NATS pending 事件磁盘 WAL、前端 JWT base64url 解码修复、ban 用户前端权限禁用、Cloudflare 会话 Domain 成员校验、消息持久化失败回归确认。全部采用 TDD：先写失败测试，再实现，最后提交。

**Tech Stack:** Go 1.26 + Gin + GORM + NATS（app/server），SolidJS + TypeScript + Vitest（app/web）。

---

## 已核实的现状（避免重复修复）

以下 review findings 经复核已由现有代码/测试覆盖，不在本计划中重复实现：

- 消息持久化失败返回成功：`message_service.go` 的 `Send` 在 `syncWriteAllowed=false` 时已返回错误，`message_service_test.go:593` 已有 `TestMessageService_Send_RejectsSyncWriteOnDataPlane`。
- 会话归属校验：`conversation_service.go` 的 `GetMessages`/`MarkRead` 已校验参与者，`conversation_service_test.go` 已有 `TestConversationService_MarkRead_RejectsNonParticipant`。
- OAuth state 基础防护：`oauth_handler.go` 已有随机 state + HttpOnly cookie + 一次性删除 + SameSite=Lax。
- Cloudflare session IDOR：`cloudflare_media_service.go` 的 `authorizeSession` 已校验 session owner。

所有任务完成后运行：

```bash
cd /Users/noelorin/GOSpeak/app/server && go test ./...
cd /Users/noelorin/GOSpeak/app/web && pnpm test
```

---

### Task 1: 登录错误统一 401 + 通用文案

**Files:**
- Modify: `app/server/internal/pkg/response.go`
- Modify: `app/server/internal/service/auth_service.go`
- Test: `app/server/internal/pkg/response_test.go`
- Test: `app/server/internal/service/auth_service_test.go`

背景：`errToHTTPStatus` 将 `USER_NOT_FOUND`/`INVALID_PASSWORD` 映射为 400，且 `AuthService.Login` 返回不同文案，攻击者可枚举用户名。修复后登录失败统一 401 + `"invalid credentials"`。注册冲突（`USERNAME_EXISTS`/`EMAIL_ALREADY_EXISTS`）保持 400，不破坏注册 UX。

- [ ] **Step 1: 写失败测试（response 映射）**

在 `app/server/internal/pkg/response_test.go` 末尾追加：

```go
func TestHandleError_LoginErrorsUse401(t *testing.T) {
	for _, code := range []ErrCode{USER_NOT_FOUND, INVALID_PASSWORD} {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/login", nil)
		HandleError(c, NewAppError(code, "invalid credentials"))
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("code %d status = %d, want 401", code, w.Code)
		}
	}
}

func TestHandleError_RegisterErrorsUse400(t *testing.T) {
	for _, code := range []ErrCode{USERNAME_EXISTS, EMAIL_ALREADY_EXISTS} {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/register", nil)
		HandleError(c, NewAppError(code, "duplicate"))
		if w.Code != http.StatusBadRequest {
			t.Fatalf("code %d status = %d, want 400", code, w.Code)
		}
	}
}
```

- [ ] **Step 2: 写失败测试（Login 通用文案）**

在 `app/server/internal/service/auth_service_test.go` 末尾追加（文件顶部已有 `pkg` import，若缺 `errors` 则补）：

```go
func TestAuthService_Login_UnknownUserGenericMessage(t *testing.T) {
	svc := newTestAuthService(t)
	_, err := svc.Login(&LoginRequest{Username: "nobody", Password: "x"})
	var appErr *pkg.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("want AppError, got %v", err)
	}
	if appErr.Code != pkg.USER_NOT_FOUND || appErr.Msg != "invalid credentials" {
		t.Fatalf("got code=%d msg=%q, want USER_NOT_FOUND + generic message", appErr.Code, appErr.Msg)
	}
}
```

- [ ] **Step 3: 运行测试确认失败**

Run: `cd /Users/noelorin/GOSpeak/app/server && go test ./internal/pkg/ ./internal/service/ -run 'TestHandleError_LoginErrorsUse401|TestHandleError_RegisterErrorsUse400|TestAuthService_Login_UnknownUserGenericMessage' -v`
Expected: 3 个测试 FAIL（401/400 断言与文案断言不满足）。

- [ ] **Step 4: 修改 `pkg/response.go`**

将：

```go
	case INVALID_PASSWORD, USER_NOT_FOUND, USERNAME_EXISTS, EMAIL_ALREADY_EXISTS:
		// 登录/注册业务错误：避免与 token 鉴权 401 混淆
		return http.StatusBadRequest
```

替换为：

```go
	case INVALID_PASSWORD, USER_NOT_FOUND:
		// 登录失败统一 401，避免通过状态码枚举用户名
		return http.StatusUnauthorized
	case USERNAME_EXISTS, EMAIL_ALREADY_EXISTS:
		return http.StatusBadRequest
```

- [ ] **Step 5: 修改 `service/auth_service.go` 的 Login**

将：

```go
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, pkg.NewAppError(pkg.USER_NOT_FOUND, "user not found")
		}
```

替换为：

```go
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, pkg.NewAppError(pkg.USER_NOT_FOUND, "invalid credentials")
		}
```

并将：

```go
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		return nil, pkg.NewAppError(pkg.INVALID_PASSWORD)
	}
```

替换为：

```go
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		return nil, pkg.NewAppError(pkg.INVALID_PASSWORD, "invalid credentials")
	}
```

- [ ] **Step 6: 运行测试确认通过**

Run: `cd /Users/noelorin/GOSpeak/app/server && go test ./internal/pkg/ ./internal/service/ -run 'TestHandleError_LoginErrorsUse401|TestHandleError_RegisterErrorsUse400|TestAuthService_Login_UnknownUserGenericMessage' -v`
Expected: PASS。

- [ ] **Step 7: 全量回归 + 提交**

Run: `cd /Users/noelorin/GOSpeak/app/server && go test ./...`
Expected: 全部 PASS。

```bash
git add app/server/internal/pkg/response.go app/server/internal/service/auth_service.go app/server/internal/pkg/response_test.go app/server/internal/service/auth_service_test.go
git commit -m "fix(auth): unify login errors to 401 with generic message"
```

---

### Task 2: OAuth state 强制服务端生成

**Files:**
- Modify: `app/server/internal/handler/oauth_handler.go`
- Test: `app/server/internal/handler/oauth_handler_test.go`

背景：`Login` 当前信任客户端传入的 `?state=` 并原样写入 cookie。虽然已有回调校验，仍应统一由服务端生成，避免攻击者预置固定 state。

- [ ] **Step 1: 写失败测试**

在 `app/server/internal/handler/oauth_handler_test.go` 末尾追加：

```go
func TestOAuthHandler_Login_IgnoresClientState(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := newTestOAuthHandler(t)

	router := gin.New()
	router.GET("/login/:provider", h.Login)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/login/github?state=attacker-state", nil)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want 302", rec.Code)
	}
	var stateCookie string
	for _, ck := range rec.Result().Cookies() {
		if ck.Name == oauthStateCookie {
			stateCookie = ck.Value
		}
	}
	if stateCookie == "" {
		t.Fatal("expected oauth state cookie")
	}
	if stateCookie == "attacker-state" {
		t.Fatal("handler must not trust client-provided state")
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd /Users/noelorin/GOSpeak/app/server && go test ./internal/handler/ -run TestOAuthHandler_Login_IgnoresClientState -v`
Expected: FAIL（当前 cookie 值等于 `attacker-state`）。

- [ ] **Step 3: 修改 `oauth_handler.go` 的 Login**

将：

```go
	// 服务端生成高熵 state，写入 HttpOnly cookie，callback 强制校验，防 OAuth Login CSRF。
	state := c.Query("state")
	if state == "" {
		generated, err := randomState(16)
		if err != nil {
			redirectOAuthError(c, "failed to generate oauth state")
			return
		}
		state = generated
	}
```

替换为：

```go
	// 服务端生成高熵 state，写入 HttpOnly cookie，callback 强制校验，防 OAuth Login CSRF。
	// 不信任客户端传入的 state：统一服务端生成，避免预置固定 state。
	generated, err := randomState(16)
	if err != nil {
		redirectOAuthError(c, "failed to generate oauth state")
		return
	}
	state := generated
```

- [ ] **Step 4: 运行测试确认通过**

Run: `cd /Users/noelorin/GOSpeak/app/server && go test ./internal/handler/ -run TestOAuthHandler_Login_IgnoresClientState -v`
Expected: PASS。

- [ ] **Step 5: 全量回归 + 提交**

Run: `cd /Users/noelorin/GOSpeak/app/server && go test ./...`
Expected: 全部 PASS。

```bash
git add app/server/internal/handler/oauth_handler.go app/server/internal/handler/oauth_handler_test.go
git commit -m "fix(oauth): force server-side state generation"
```

---

### Task 3: Domain 中间件 body 读取限流

**Files:**
- Modify: `app/server/internal/middleware/domain.go`
- Test: `app/server/internal/middleware/domain_test.go`

背景：`RequireDomainMember`/`RequireDomainMemberIfProvided` 用 `io.ReadAll` 读取整个请求体，大请求（上传、大 JSON）会全量占内存。用 `http.MaxBytesReader` 限制到 1 MiB，超限返回 400。

- [ ] **Step 1: 写失败测试**

在 `app/server/internal/middleware/domain_test.go` 末尾追加：

```go
func TestRequireDomainMember_OversizedBodyRejected(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("user_uuid", "uuid-user")
		c.Next()
	})
	r.POST("/test", RequireDomainMember(), func(c *gin.Context) {
		c.Status(200)
	})
	body := strings.Repeat("a", maxDomainBodyBytes+1)
	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}
```

确保 `domain_test.go` 顶部已有 `net/http`、`net/http/httptest`、`strings`、`github.com/gin-gonic/gin` import；缺哪个补哪个。

- [ ] **Step 2: 运行测试确认失败**

Run: `cd /Users/noelorin/GOSpeak/app/server && go test ./internal/middleware/ -run TestRequireDomainMember_OversizedBodyRejected -v`
Expected: FAIL（当前会读到全部 body，状态 200）。

- [ ] **Step 3: 修改 `middleware/domain.go`**

在 import 中加入 `"net/http"`，并在文件顶部常量区添加：

```go
// maxDomainBodyBytes 限制 body 读取大小：中间件只需 domain_uuid 字段，超限直接拒绝。
const maxDomainBodyBytes = 1 << 20 // 1 MiB
```

将 `RequireDomainMemberIfProvided` 中的：

```go
		if domainUUID == "" {
			var body struct {
				DomainUUID string `json:"domain_uuid"`
			}
			raw, readErr := io.ReadAll(c.Request.Body)
			c.Request.Body = io.NopCloser(bytes.NewReader(raw))
			if readErr == nil && len(raw) > 0 {
				if err := json.Unmarshal(raw, &body); err == nil {
					domainUUID = body.DomainUUID
				}
			}
		}
```

替换为：

```go
		if domainUUID == "" {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxDomainBodyBytes)
			raw, readErr := io.ReadAll(c.Request.Body)
			c.Request.Body = io.NopCloser(bytes.NewReader(raw))
			if readErr != nil {
				pkg.Fail(c, pkg.INVALID_PARAMS, "request body too large")
				c.Abort()
				return
			}
			if len(raw) > 0 {
				var body struct {
					DomainUUID string `json:"domain_uuid"`
				}
				if err := json.Unmarshal(raw, &body); err == nil {
					domainUUID = body.DomainUUID
				}
			}
		}
```

对 `RequireDomainMember` 中的相同代码块做同样的替换。

- [ ] **Step 4: 运行测试确认通过**

Run: `cd /Users/noelorin/GOSpeak/app/server && go test ./internal/middleware/ -run 'TestRequireDomainMember_OversizedBodyRejected|TestRequireDomainMember_PreservesBody|TestRequireDomainMemberIfProvided' -v`
Expected: 全部 PASS。

- [ ] **Step 5: 全量回归 + 提交**

Run: `cd /Users/noelorin/GOSpeak/app/server && go test ./...`
Expected: 全部 PASS。

```bash
git add app/server/internal/middleware/domain.go app/server/internal/middleware/domain_test.go
git commit -m "fix(middleware): cap domain body read at 1MiB"
```

---

### Task 4: 平台级私密文本房间历史/搜索访问控制

**Files:**
- Modify: `app/server/internal/service/message_history.go`
- Modify: `app/server/internal/handler/message_handler.go`
- Test: `app/server/internal/service/message_service_test.go`

背景：`ListHistory`/`Search` 对 Domain 房间有成员校验，但平台级（`DomainUUID == ""`）带密码的文本房间，任何登录用户只要知道 `room_uuid` 就能读取历史/搜索。为这两个方法增加 `password` 参数，平台级私密房间要求密码（创建者免密）。

- [ ] **Step 1: 写失败测试**

在 `app/server/internal/service/message_service_test.go` 末尾追加，复用现有 `setupMessageServiceTest`（返回 `svc, bus, queue, roomRepo, textUUID`）与 `memRoomRepo`/`pkg.HashPassword`：

```go
func TestMessageService_ListHistory_PlatformPasswordRoomRejectsWrongPassword(t *testing.T) {
	svc, _, _, roomRepo, _ := setupMessageServiceTest(t)
	roomUUID := uuid.New().String()
	hashed, _ := pkg.HashPassword("secret")
	roomRepo.mu.Lock()
	roomRepo.rooms[roomUUID] = &model.Room{
		UUID:       roomUUID,
		Name:       "private-text",
		Type:       model.RoomTypeText,
		DomainUUID: "",
		Password:   hashed,
	}
	roomRepo.mu.Unlock()

	actor := MessageActor{Identity: "bob", UserUUID: "uuid-bob"}
	_, _, _, err := svc.ListHistory(roomUUID, actor, "", 100, "wrong")
	var appErr *pkg.AppError
	if !errors.As(err, &appErr) || appErr.Code != pkg.FORBIDDEN {
		t.Fatalf("want FORBIDDEN, got %v", err)
	}
}

func TestMessageService_ListHistory_PlatformPasswordRoomAllowsCorrectPassword(t *testing.T) {
	svc, _, _, roomRepo, _ := setupMessageServiceTest(t)
	roomUUID := uuid.New().String()
	hashed, _ := pkg.HashPassword("secret")
	roomRepo.mu.Lock()
	roomRepo.rooms[roomUUID] = &model.Room{
		UUID:       roomUUID,
		Name:       "private-text",
		Type:       model.RoomTypeText,
		DomainUUID: "",
		Password:   hashed,
	}
	roomRepo.mu.Unlock()

	actor := MessageActor{Identity: "bob", UserUUID: "uuid-bob"}
	if _, _, _, err := svc.ListHistory(roomUUID, actor, "", 100, "secret"); err != nil {
		t.Fatalf("want nil, got %v", err)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd /Users/noelorin/GOSpeak/app/server && go test ./internal/service/ -run 'TestMessageService_ListHistory_PlatformPasswordRoom' -v`
Expected: FAIL（当前不校验密码，错误密码也能读到历史）。

- [ ] **Step 3: 修改 `message_history.go`**

新增方法：

```go
// requireRoomAccess 校验房间访问权：Domain 房间走成员校验；
// 平台级私密房间要求调用者提供正确密码（创建者免密）。
func (s *MessageService) requireRoomAccess(room *model.Room, actor MessageActor, password string) error {
	if room.DomainUUID != "" {
		return s.requireDomainMembership(room, actor)
	}
	if room.Password == "" {
		return nil
	}
	if actor.UserUUID != "" && room.CreatedBy == actor.Identity {
		return nil
	}
	if pkg.VerifyPassword(room.Password, password) {
		return nil
	}
	return pkg.NewAppError(pkg.FORBIDDEN, "room password required")
}
```

将 `ListHistory` 签名改为：

```go
func (s *MessageService) ListHistory(roomUUID string, actor MessageActor, before string, limit int, password string) (items []MessageDTO, hasMore bool, nextBefore string, err error) {
```

将其中的 `if err := s.requireDomainMembership(room, actor); err != nil {` 替换为 `if err := s.requireRoomAccess(room, actor, password); err != nil {`。

将 `Search` 签名改为：

```go
func (s *MessageService) Search(roomUUID string, actor MessageActor, query, password string) ([]MessageDTO, error) {
```

将其中的 `if err := s.requireDomainMembership(room, actor); err != nil {` 替换为 `if err := s.requireRoomAccess(room, actor, password); err != nil {`。

- [ ] **Step 4: 修改 `message_handler.go`**

`List` 请求结构体增加 `Password string` 字段：

```go
	var req struct {
		RoomUUID string `json:"room_uuid" binding:"required"`
		Before   string `json:"before"`
		Limit    int    `json:"limit"`
		Password string `json:"password"`
	}
```

调用改为：

```go
	items, hasMore, nextBefore, err := h.msgSvc.ListHistory(req.RoomUUID, actor, req.Before, req.Limit, req.Password)
```

`Search` 请求结构体增加 `Password string` 字段：

```go
	var req struct {
		RoomUUID string `json:"room_uuid" binding:"required"`
		Query    string `json:"query" binding:"required"`
		Password string `json:"password"`
	}
```

调用改为：

```go
	items, err := h.msgSvc.Search(req.RoomUUID, actor, req.Query, req.Password)
```

- [ ] **Step 5: 运行测试确认通过**

Run: `cd /Users/noelorin/GOSpeak/app/server && go test ./internal/service/ -run 'TestMessageService_ListHistory_PlatformPasswordRoom' -v`
Expected: PASS。

- [ ] **Step 6: 全量回归 + 提交**

Run: `cd /Users/noelorin/GOSpeak/app/server && go test ./...`
Expected: 全部 PASS。

```bash
git add app/server/internal/service/message_history.go app/server/internal/handler/message_handler.go app/server/internal/service/message_service_test.go
git commit -m "fix(message): require password for platform private room history"
```

---

### Task 5: NATS pending 事件磁盘 WAL 持久化

**Files:**
- Create: `app/server/internal/bus/wal.go`
- Modify: `app/server/internal/bus/nats_bus.go`
- Modify: `app/server/internal/bus/nats_publish.go`
- Test: `app/server/internal/bus/wal_test.go`

背景：NATS 断线期间的 pending 事件只存内存，进程崩溃即永久丢失（禁言/踢人等关键事件）。新增磁盘 WAL：`enqueuePending` 时同步 append + fsync，`flushPending` 成功后 truncate，启动时恢复。

- [ ] **Step 1: 写失败测试（WAL 基础行为）**

创建 `app/server/internal/bus/wal_test.go`：

```go
package bus

import (
	"path/filepath"
	"testing"
)

func TestPendingWAL_AppendReadTruncate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pending.wal")
	w, err := newPendingWAL(path)
	if err != nil {
		t.Fatalf("open wal: %v", err)
	}
	env := Envelope{V: 1, InstanceID: "i1", Scope: "room", Room: "r1", Event: "kick", TS: 1}
	if err := w.Append("gospeak.signal.room.r1", env); err != nil {
		t.Fatalf("append: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	reopened, err := newPendingWAL(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	items, err := reopened.ReadAll()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(items) != 1 || items[0].subject != "gospeak.signal.room.r1" || items[0].env.Event != "kick" {
		t.Fatalf("recovered %+v, want 1 kick envelope", items)
	}
	if err := reopened.Truncate(); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	items, err = reopened.ReadAll()
	if err != nil || len(items) != 0 {
		t.Fatalf("after truncate: items=%v err=%v, want empty", items, err)
	}
	if err := reopened.Close(); err != nil {
		t.Fatalf("close reopened: %v", err)
	}
}
```

- [ ] **Step 2: 实现 `wal.go`**

创建 `app/server/internal/bus/wal.go`：

```go
package bus

import (
	"bufio"
	"encoding/json"
	"os"
	"sync"
)

// pendingWAL 将断线期间 pending 的事件追加到磁盘 JSONL 文件，
// 每次 Append 后 fsync，进程崩溃后可从 ReadAll 恢复。
type pendingWAL struct {
	mu   sync.Mutex
	path string
	f    *os.File
}

func newPendingWAL(path string) (*pendingWAL, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, err
	}
	return &pendingWAL{path: path, f: f}, nil
}

func (w *pendingWAL) Append(subject string, env Envelope) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	data, err := json.Marshal(pendingEnvelope{subject: subject, env: env})
	if err != nil {
		return err
	}
	if _, err := w.f.Write(append(data, '\n')); err != nil {
		return err
	}
	return w.f.Sync()
}

func (w *pendingWAL) ReadAll() ([]pendingEnvelope, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	f, err := os.Open(w.path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []pendingEnvelope
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1024*1024), 4*1024*1024)
	for sc.Scan() {
		var p pendingEnvelope
		if err := json.Unmarshal(sc.Bytes(), &p); err == nil {
			out = append(out, p)
		}
	}
	return out, sc.Err()
}

func (w *pendingWAL) Truncate() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := w.f.Truncate(0); err != nil {
		return err
	}
	_, err := w.f.Seek(0, 0)
	return err
}

func (w *pendingWAL) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.f.Close()
}
```

- [ ] **Step 3: 运行 WAL 测试确认通过**

Run: `cd /Users/noelorin/GOSpeak/app/server && go test ./internal/bus/ -run TestPendingWAL_AppendReadTruncate -v`
Expected: PASS。

- [ ] **Step 4: 接线 NATSBus**

在 `nats_bus.go` 的 `NATSBusConfig` 增加字段：

```go
	// WALPath 非空时启用断线事件磁盘持久化；为空保持纯内存行为（默认）。
	WALPath string
```

在 `NATSBus` 结构体增加字段：

```go
	wal *pendingWAL
```

在 `NewNATSBus` 中 `b := &NATSBus{...}` 之后、`nats.Connect` 之前加入：

```go
	if cfg.WALPath != "" {
		wal, err := newPendingWAL(cfg.WALPath)
		if err != nil {
			return nil, fmt.Errorf("open pending wal: %w", err)
		}
		b.wal = wal
		if recovered, err := wal.ReadAll(); err == nil && len(recovered) > 0 {
			b.pending = recovered
			log.Printf("[EventBus] recovered %d pending events from WAL", len(recovered))
		}
	}
```

在 `Close()` 中 `b.nc.Close()` 之后加入：

```go
	if b.wal != nil {
		_ = b.wal.Close()
	}
```

在 `nats_publish.go` 的 `enqueuePending` 末尾（`b.pending = append(...)` 之后）加入：

```go
	if b.wal != nil {
		if err := b.wal.Append(subject, env); err != nil {
			log.Printf("[EventBus] wal append failed, keeping in memory only: %v", err)
		}
	}
```

在 `flushPending` 的重放循环之后加入：

```go
	if b.wal != nil {
		if err := b.wal.Truncate(); err != nil {
			log.Printf("[EventBus] wal truncate failed: %v", err)
		}
	}
```

- [ ] **Step 5: 写集成测试**

在 `app/server/internal/bus/nats_bus_test.go` 末尾追加：

```go
func TestNATSBus_PendingWALRecoveredOnRestart(t *testing.T) {
	url := natsTestURL(t)
	walPath := filepath.Join(t.TempDir(), "pending.wal")
	b, err := NewNATSBus(NATSBusConfig{
		InstanceID:    "instance-1",
		SubjectPrefix: "gospeak",
		URL:           url,
		Name:          "instance-1",
		Mode:          "external",
		WALPath:       walPath,
	})
	if err != nil {
		t.Fatalf("new bus: %v", err)
	}
	b.enqueuePending("gospeak.signal.room.r1", Envelope{
		V: 1, InstanceID: "instance-1", Scope: "room", Room: "r1", Event: "kick", TS: 1,
	})
	if err := b.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	b2, err := NewNATSBus(NATSBusConfig{
		InstanceID:    "instance-2",
		SubjectPrefix: "gospeak",
		URL:           url,
		Name:          "instance-2",
		Mode:          "external",
		WALPath:       walPath,
	})
	if err != nil {
		t.Fatalf("reopen bus: %v", err)
	}
	defer b2.Close()
	b2.pendingMu.Lock()
	recovered := len(b2.pending)
	b2.pendingMu.Unlock()
	if recovered != 1 {
		t.Fatalf("recovered %d pending, want 1", recovered)
	}
}
```

确保 `nats_bus_test.go` 顶部已 import `path/filepath`；缺则补。

- [ ] **Step 6: 运行测试确认通过**

Run: `cd /Users/noelorin/GOSpeak/app/server && go test ./internal/bus/ -run 'TestPendingWAL_AppendReadTruncate|TestNATSBus_PendingWALRecoveredOnRestart' -v`
Expected: PASS。

- [ ] **Step 7: 全量回归 + 提交**

Run: `cd /Users/noelorin/GOSpeak/app/server && go test ./...`
Expected: 全部 PASS。

```bash
git add app/server/internal/bus/wal.go app/server/internal/bus/wal_test.go app/server/internal/bus/nats_bus.go app/server/internal/bus/nats_publish.go app/server/internal/bus/nats_bus_test.go
git commit -m "feat(bus): persist pending NATS events to disk WAL"
```

---

### Task 6: 前端 JWT base64url 解码修复

**Files:**
- Modify: `app/web/src/utils/permissions.ts`
- Test: `app/web/src/utils/permissions.test.ts`

背景：`decodeJWTPayload` 直接用 `atob()` 解码 JWT payload。JWT 使用 base64url（`-`/`_`、无 padding），`atob` 对含 `-`/`_` 或需要 padding 的 payload 会解码失败或被静默吞掉。新增导出 `decodeBase64Url`，先转标准 base64 再补 padding，并用 `TextDecoder` 正确处理 UTF-8。

- [ ] **Step 1: 写失败测试**

在 `app/web/src/utils/permissions.test.ts` 顶部 import 中追加 `decodeBase64Url`：

```ts
import { decodeBase64Url, rolePermissions } from "./permissions";
```

在文件末尾追加：

```ts
describe("decodeBase64Url", () => {
	it("decodes base64url payload containing URL-safe characters", () => {
		const payload = JSON.stringify({ sub: "user", role: "admin", permissions: ["room:create"] });
		const b64 = btoa(payload).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/, "");
		expect(decodeBase64Url(b64)).toBe(payload);
	});

	it("decodes unicode payload", () => {
		const payload = JSON.stringify({ name: "管理员" });
		const b64 = btoa(unescape(encodeURIComponent(payload)))
			.replace(/\+/g, "-")
			.replace(/\//g, "_")
			.replace(/=+$/, "");
		expect(decodeBase64Url(b64)).toBe(payload);
	});
});
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd /Users/noelorin/GOSpeak/app/web && pnpm vitest run src/utils/permissions.test.ts`
Expected: FAIL（`decodeBase64Url` 未导出）。

- [ ] **Step 3: 修改 `permissions.ts`**

在文件顶部（`userStore` import 之后）新增：

```ts
/** 解码 JWT base64url payload；兼容 UTF-8 与 URL-safe 字符集。 */
export function decodeBase64Url(input: string): string {
	const base64 = input.replace(/-/g, "+").replace(/_/g, "/");
	const padded = base64.padEnd(Math.ceil(base64.length / 4) * 4, "=");
	const raw = atob(padded);
	const bytes = Uint8Array.from(raw, (ch) => ch.charCodeAt(0));
	return new TextDecoder().decode(bytes);
}
```

将 `decodeJWTPayload` 中的：

```ts
		return JSON.parse(atob(payload)) as T;
```

替换为：

```ts
		return JSON.parse(decodeBase64Url(payload)) as T;
```

- [ ] **Step 4: 运行测试确认通过**

Run: `cd /Users/noelorin/GOSpeak/app/web && pnpm vitest run src/utils/permissions.test.ts`
Expected: PASS。

- [ ] **Step 5: 全量前端测试 + 提交**

Run: `cd /Users/noelorin/GOSpeak/app/web && pnpm test`
Expected: 全部 PASS。

```bash
git add app/web/src/utils/permissions.ts app/web/src/utils/permissions.test.ts
git commit -m "fix(web): decode JWT payload as base64url"
```

---

### Task 7: ban 用户前端权限禁用

**Files:**
- Modify: `app/web/src/utils/permissions.ts`
- Test: `app/web/src/utils/permissions.test.ts`

背景：前端 `claimPermissions()` 优先读 JWT claims 中的 permissions，ban 用户持有旧 token 时仍可能通过前端守卫进入管理页。修复：ban 用户一律返回无权限。

- [ ] **Step 1: 写失败测试**

在 `app/web/src/utils/permissions.test.ts` 顶部追加 mock（放在现有 `vi.mock("idb-keyval", ...)` 之后、import 之前）：

```ts
vi.mock("@/stores/userStore", () => ({
	default: {
		user: () => ({ role: "ban", permissions: ["room:create", "user:read"] }),
		accessToken: () => "eyJhbGciOiJIUzI1NiJ9.eyJwZXJtaXNzaW9ucyI6WyJyb29tOmNyZWF0ZSJdfQ.x",
	},
}));
```

在文件末尾追加：

```ts
describe("banned user", () => {
	it("has no permissions even when role permissions or claims exist", () => {
		expect(hasPermission("room:create")).toBe(false);
		expect(hasPermission("user:read")).toBe(false);
		expect(hasManageAccess()).toBe(false);
	});
});
```

将 import 改为：

```ts
import { hasManageAccess, hasPermission, rolePermissions } from "./permissions";
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd /Users/noelorin/GOSpeak/app/web && pnpm vitest run src/utils/permissions.test.ts`
Expected: FAIL（当前 ban 用户经 `rolePermissions` 或 claims 仍可能拿到权限）。

- [ ] **Step 3: 修改 `permissions.ts`**

新增辅助函数：

```ts
function isBannedUser(): boolean {
	return userStore.user()?.role === "ban";
}
```

将 `hasPermission` 改为：

```ts
export function hasPermission(code: string): boolean {
	// 被封禁用户一律无权限，即使旧 token 仍携带 permissions claims。
	if (isBannedUser()) return false;

	// 服务端 profile 下发的权限是权威来源；未加载时退回 JWT claims 与本地兜底表。
	const profilePerms = userStore.user()?.permissions;
	if (profilePerms && profilePerms.length > 0) {
		return profilePerms.includes(code);
	}

	const claims = claimPermissions();
	if (claims) return claims.includes(code);

	const role = userStore.user()?.role;
	if (!role) return false;
	return rolePermissions[role]?.includes(code) ?? false;
}
```

将 `claimPermissions` 改为：

```ts
function claimPermissions(): string[] | null {
	if (isBannedUser()) return null;
	const token = userStore.accessToken();
	if (!token) return null;
	const payload = decodeJWTPayload<{ permissions?: unknown }>(token);
	const perms = payload?.permissions;
	if (!Array.isArray(perms) || perms.length === 0) return null;
	return perms.filter(
		(p): p is string => typeof p === "string" && p.length > 0,
	);
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `cd /Users/noelorin/GOSpeak/app/web && pnpm vitest run src/utils/permissions.test.ts`
Expected: PASS。

- [ ] **Step 5: 全量前端测试 + 提交**

Run: `cd /Users/noelorin/GOSpeak/app/web && pnpm test`
Expected: 全部 PASS。

```bash
git add app/web/src/utils/permissions.ts app/web/src/utils/permissions.test.ts
git commit -m "fix(web): disable permissions for banned users"
```

---

### Task 8: Cloudflare 会话 Domain 成员校验

**Files:**
- Modify: `app/server/internal/sfu/providers/cloudflare/provider.go`
- Modify: `app/server/internal/service/cloudflare_media_service.go`
- Modify: `app/server/internal/sfu/factory/dynamic_provider.go`
- Test: `app/server/internal/sfu/providers/cloudflare/provider_test.go`
- Test: `app/server/internal/service/cloudflare_media_service_test.go`

背景：Cloudflare 媒体接口已有 session owner IDOR 防护，但未校验「session 所属 Domain 中当前用户是否为成员」。provider 的 room 是 `domainUUID:roomName` 复合键，可用 `pkg.SplitRoomKey` 反查 domain，再校验成员关系。

- [ ] **Step 1: 写失败测试（provider SessionDomain）**

在 `app/server/internal/sfu/providers/cloudflare/provider_test.go` 末尾追加：

```go
func TestService_SessionDomain(t *testing.T) {
	svc := NewService(&config.Config{CFAppID: "app", CFAppSecret: "secret"})
	svc.putSession("dom-1:room-a", "alice", "session-1", 1, "uuid-alice")

	domain, ok := svc.SessionDomain("session-1")
	if !ok || domain != "dom-1" {
		t.Fatalf("SessionDomain = %q, %v; want dom-1, true", domain, ok)
	}
	if _, ok := svc.SessionDomain("unknown-session"); ok {
		t.Fatal("expected unknown session to have no domain")
	}
}
```

确保文件已 import `GOSpeak/internal/config`；缺则补。

- [ ] **Step 2: 运行测试确认失败**

Run: `cd /Users/noelorin/GOSpeak/app/server && go test ./internal/sfu/providers/cloudflare/ -run TestService_SessionDomain -v`
Expected: FAIL（方法不存在）。

- [ ] **Step 3: 实现 `provider.go` 的 SessionDomain**

在 `SessionOwner` 方法之后追加：

```go
// SessionDomain 返回 session 所属 Domain UUID；room 是 domainUUID:roomName 复合键。
func (s *Service) SessionDomain(sessionID string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for room, members := range s.sessions {
		for _, meta := range members {
			if meta.sessionID == sessionID {
				domainUUID, _ := pkg.SplitRoomKey(room)
				return domainUUID, true
			}
		}
	}
	return "", false
}
```

- [ ] **Step 4: 写失败测试（service 拒绝非成员）**

在 `app/server/internal/service/cloudflare_media_service_test.go` 末尾追加。先阅读该文件现有构造方式（`NewCloudflareMediaService` + `clientFactory` + `sessionOwner`），复用相同模式：

```go
func TestCloudflareMediaService_RejectsNonDomainMember(t *testing.T) {
	svc := NewCloudflareMediaService(func() (*config.Config, error) {
		return &config.Config{CFAppID: "app", CFAppSecret: "secret"}, nil
	})
	svc.clientFactory = func() (cloudflareMediaClient, error) {
		return &fakeCloudflareMediaClient{}, nil
	}
	svc.sessionOwner = func(sessionID string) (string, bool) {
		return "uuid-alice", true
	}
	svc.sessionDomain = func(sessionID string) (string, bool) {
		return "dom-1", true
	}
	svc.domainMember = func(domainUUID, userUUID string) bool {
		return userUUID == "uuid-member"
	}
	if err := svc.AddTracks("session-1", "uuid-alice", &cloudflare.TrackRequest{}); err == nil {
		t.Fatal("expected FORBIDDEN for non-domain-member owner")
	}
}
```

若测试文件无 `fakeCloudflareMediaClient`，按现有测试中的 stub 模式定义最小实现（实现 `cloudflareMediaClient` 接口，各方法返回 nil/nil 或对应类型）。

- [ ] **Step 5: 运行测试确认失败**

Run: `cd /Users/noelorin/GOSpeak/app/server && go test ./internal/service/ -run TestCloudflareMediaService_RejectsNonDomainMember -v`
Expected: FAIL（当前不校验 domain 成员）。

- [ ] **Step 6: 实现 service 层校验**

在 `app/server/internal/service/cloudflare_media_service.go` 的 `CloudflareMediaService` 结构体增加字段：

```go
	sessionDomain func(sessionID string) (string, bool)
	domainMember  func(domainUUID, userUUID string) bool
```

新增注入方法：

```go
// SetSessionDomainLookup 注入 sessionID → domainUUID 查询。
func (s *CloudflareMediaService) SetSessionDomainLookup(lookup func(sessionID string) (string, bool)) {
	s.sessionDomain = lookup
}

// SetDomainMemberChecker 注入 Domain 成员校验函数。
func (s *CloudflareMediaService) SetDomainMemberChecker(checker func(domainUUID, userUUID string) bool) {
	s.domainMember = checker
}
```

在 `authorizeSession` 的 owner 校验之后追加：

```go
	if s.sessionDomain != nil && s.domainMember != nil {
		if domainUUID, ok := s.sessionDomain(sessionID); ok && domainUUID != "" {
			if !s.domainMember(domainUUID, userUUID) {
				return pkg.NewAppError(pkg.FORBIDDEN, "not a member of the session domain")
			}
		}
	}
```

- [ ] **Step 7: 接线 DynamicProvider**

在 `app/server/internal/sfu/factory/dynamic_provider.go` 中新增接口与方法：

```go
type sessionDomainLookup interface {
	SessionDomain(sessionID string) (string, bool)
}

// SessionDomain 返回 provider session 所属 Domain（受支持时）。
func (p *DynamicProvider) SessionDomain(sessionID string) (string, bool) {
	provider, err := p.current()
	if err != nil {
		return "", false
	}
	if lp, ok := provider.(sessionDomainLookup); ok {
		return lp.SessionDomain(sessionID)
	}
	return "", false
}
```

- [ ] **Step 8: 接线组合根**

找到 `server/gin.go` 中 `NewCloudflareMediaService` 的调用位置（搜索 `SetSessionOwnerLookup`），在其后追加：

```go
	mediaSvc.SetSessionDomainLookup(dynamicProvider.SessionDomain)
	mediaSvc.SetDomainMemberChecker(func(domainUUID, userUUID string) bool {
		return domainSvc.IsMember(domainUUID, userUUID)
	})
```

变量名以 `server/gin.go` 实际命名为准（`mediaSvc`、`dynamicProvider`、`domainSvc` 均为现有组合根变量）。

- [ ] **Step 9: 运行全部相关测试**

Run: `cd /Users/noelorin/GOSpeak/app/server && go test ./internal/sfu/... ./internal/service/ ./internal/handler/ -run 'TestService_SessionDomain|TestCloudflareMediaService' -v`
Expected: 全部 PASS。

- [ ] **Step 10: 全量回归 + 提交**

Run: `cd /Users/noelorin/GOSpeak/app/server && go test ./...`
Expected: 全部 PASS。

```bash
git add app/server/internal/sfu/providers/cloudflare/provider.go app/server/internal/sfu/providers/cloudflare/provider_test.go app/server/internal/service/cloudflare_media_service.go app/server/internal/service/cloudflare_media_service_test.go app/server/internal/sfu/factory/dynamic_provider.go app/server/server/gin.go
git commit -m "fix(cloudflare): enforce domain membership on media routes"
```

---

### Task 9: 消息持久化失败回归确认

**Files:**
- Verify: `app/server/internal/service/message_service_test.go:593`

背景：review 指出「消息持久化失败仍返回成功 DTO」，复核确认 `Send` 已正确处理且已有回归测试。本任务只做验证，不改代码；若运行中发现测试缺口，再按 TDD 补一条。

- [ ] **Step 1: 运行现有回归测试**

Run: `cd /Users/noelorin/GOSpeak/app/server && go test ./internal/service/ -run 'TestMessageService_Send_RejectsSyncWriteOnDataPlane|TestMessageService_Send_QueueFallback|TestMessageService_Send_NoBusNoQueue' -v`
Expected: 全部 PASS。

- [ ] **Step 2: 检查测试断言覆盖**

阅读 `message_service_test.go:593` 的 `TestMessageService_Send_RejectsSyncWriteOnDataPlane`。若它只断言错误非 nil，追加一条断言：错误消息包含 `"message persistence unavailable"`，且 DB 中无该消息记录。

```go
	var appErr *pkg.AppError
	if !errors.As(err, &appErr) || !strings.Contains(appErr.Msg, "message persistence unavailable") {
		t.Fatalf("want persistence-unavailable error, got %v", err)
	}
```

若测试已覆盖上述断言，直接进入 Step 3。

- [ ] **Step 3: 运行全量 Go 测试**

Run: `cd /Users/noelorin/GOSpeak/app/server && go test ./...`
Expected: 全部 PASS。

```bash
git add app/server/internal/service/message_service_test.go
git commit -m "test(message): assert persistence-unavailable error on data plane"
```

---

## 附录 A: Top 10 修复追踪表

| # | 问题 | 状态 | 任务 |
|---|------|------|------|
| 1 | 强制校验 JWT 密钥 | 用户明确排除，不实施 | - |
| 2 | 登录/邮箱错误统一 401/通用文案 | 待实施 | Task 1 |
| 3 | OAuth state 落库校验 | 基础防护已有，加固服务端生成 | Task 2 |
| 4 | 修复 Domain 中间件 body 预读 | 待实施（限流 1 MiB） | Task 3 |
| 5 | 会话/消息接口补归属校验 | 会话与 Domain 房间已校验；平台级私密房间补密码 | Task 4 |
| 6 | NATS pending 事件持久化或 WAL | 待实施（磁盘 JSONL WAL） | Task 5 |
| 7 | permissions.ts 换标准 base64url 解码 | 待实施 | Task 6 |
| 8 | ban 用户前端权限与 JWT claims 对齐 | 待实施 | Task 7 |
| 9 | Cloudflare 接口补 Domain 校验 | session owner 已有；补 domain 成员 | Task 8 |
| 10 | 消息持久化失败不得返回成功 | 已修复，补回归确认 | Task 9 |

## 附录 B: 6 轮 Review 完整 Findings（220 条）

### 轮次统计

| 轮次 | 🔴 Bug | 🟡 Risk | 🔵 Nit | 小计 |
|------|--------|---------|--------|------|
| R1 业务/接口层 | 11 | 26 | 10 | 47 |
| R2 深度扫描 | 7 | 26 | 9 | 42 |
| R3 遗漏补扫 | 7 | 25 | 2 | 34 |
| R4 SFU/总线 | 11 | 22 | 4 | 37 |
| R5 仓储/权限 | 7 | 23 | 3 | 33 |
| R6 集成/测试 | 9 | 14 | 4 | 27 |
| **合计** | **52** | **136** | **32** | **220** |

### R1 明细（47）

**Agent1 — Service：** `auth_service.go` Login 每次 bcrypt 比对默认密码（🔴）；`auth_service.go` 改密无复杂度校验（🟡）；`auth_service.go` Logout nil claims 静默成功（🔴）；`message_service.go` actor.Identity 无校验（🟡）；`room_service.go` service 级互斥锁跨 Domain 阻塞（🟡）；`domain_service.go` 内存成员缓存跨实例失效竞态（🟡）；`domain_service.go` invalidation payload 解析失败无日志（🔵）；`mute_service.go` byIdentity/byUserID 双缓存口径不一致（🟡）；`cluster_service.go` 空 ClusterNodeID 静默 no-op（🔵）。

**Agent2 — Handler：** `auth_handler.go` Login 400/401 不一致可枚举用户名（🔴）；`auth_handler.go` bot_ 前缀大小写绕过（🟡）；`room_handler.go` canManageRoom 无 permSvc 时回退 CreatedBy（🟡）；`room_handler.go` List 忽略 ShouldBindJSON 错误（🟡）；`signal_handler.go` WS ticket 走 subprotocol 明文（🔴）；`signal_handler.go` LiveKit 签名严格时间比对（🟡）；`user_handler.go` PresetAvatars 硬编码路径（🔵）。

**Agent3 — Infra：** `ws/client.go` 超限消息只 continue 不断连（🟡）；`ws/fanout.go` 广播串行等待慢客户端（🟡）；`ws/fanout.go` 每广播 N goroutine（🟡）；`bus/bus.go` NATS 失败仍返回成功（🟡）；`bus/bus.go` IsConnected 未被调用（🟡）；`hub.go` muteCache key 无房间维度（🟡）；`hub_room_join.go` 踢人冷却先于密码校验（🔴）；`sfu/provider.go` 能力与实现口径不一（🔵）；`hub.go` domainMemberAllowed nil checker 返回 false（🟡）。

**Agent4 — Frontend：** `voiceChatStore.ts` 持久化异步覆盖初始 store（🔴）；`apiClient.ts` ban 用户仍走刷新路径（🔴）；`apiClient.ts` refreshSubscribers 失败后竞态（🟡）；`wsClient.ts` pendingAcks 无超时清理（🟡）；`wsClient.ts` reconnectTimer 与 disconnect 竞态（🔵）；`botRunner.ts` 状态字段需进一步审查（🟡）。

### R2 明细（42）

**Agent1：** `oauth_service.go` HandleCallback 缺 state 校验（🔴）；`oauth_service.go` ClientSecret 明文落库（🟡）；`oauth_service.go` OAuth 自动建号绕过邮箱验证（🟡）；`storage_service.go` GetProvider 双初始化竞态（🔴）；`storage_service.go` fallback 配置泄露密钥（🟡）；`conversation_service.go` syncWriteAllowed 默认 true（🟡）；`email_service.go` 每次解析模板（🟡）；`sfu_service.go` 先成员后密码泄露房间存在性（🟡）；`sfu_service.go` ClientInfo 暴露凭据（🔵）；`plugin_service.go` OnConfigUpdated 顺序（🟡）。

**Agent2：** `domain_handler.go` ListPublic 忽略 keyword（🔴）；`domain_handler.go` 无限流（🟡）；`cloudflare_handler.go` sessionId 无格式校验（🟡）；`cloudflare_handler.go` 无 domain 校验（🔴）；`cloudflare_handler.go` CloseTracks 身份校验（🔴）；`oauth_handler.go` 错误进 URL（🟡）。

**Agent3：** `hub_disconnect.go` 解锁后发布竞态（🟡）；`redis_store.go` TTL 固定 24h（🟡）；`redis_store.go` CAS version 溢出（🟡）；`hub_queries.go` CheckRoomLimit 忽略 audience（🟡）；`hub_mute.go` N 次 SFU 调用（🟡）；`hub_mute.go` identityForUserID 无缓存（🔵）。

**Agent4：** `chatStore.ts` pendingNonces 不清理（🔴）；`domainStore.ts` loadMyDomains 吞错误（🟡）；`botRunner.ts` TTS 队列卡死泄漏（🟡）；`mediasoup-worker` routerPromises 不清理（🟡）；`mediasoup-worker` worker close 时状态不一致（🔴）。

### R3 明细（34）

**Agent1：** `oauth_service.go` state 无服务端存储（🟡）；`conversation_service.go` GetMessages 越权（🔴，复核已修复）；`cloudflare_media_service.go` authorizeSession 依赖注入（🔴，复核已防护）；`cluster/leader.go` TTL 与续期间隙（🟡）；`speech` buffer 不清理（🟡）；`speech` VAD 阈值固定（🟡）。

**Agent2：** `message_handler.go` ListHistory 无归属校验（🔴，复核 Domain 房间已校验、平台私密房间缺口由 Task 4 修复）；`message_handler.go` Search 无结果上限（🟡）；`cloudflare_handler.go` AddTracks 无 domain 校验（🟡，Task 8 修复）；`oauth_handler.go` 错误泄漏（🔵）。

**Agent3：** `ws/upgrader.go` extractToken 多协议取任意值（🟡）；`bus/membership_store.go` PutStream 无版本（🟡）；`cluster/leader.go` Create 非 Update 锁（🟡）；`nats_bus.go` 无重试（🟡）。

**Agent4：** `sfuSession.ts` 无响应校验（🟡）；`apiClientAuth.ts` 刷新竞态（🟡）；`mediasoupSignal.ts` listener 泄漏（复核后由 R4 覆盖）；`permissions.ts` 解码与 ban 权限（复核后由 Task 6/7 覆盖）。

### R4 明细（37）

**Agent1：** `oauth_service.go` state 确认（🔴，Task 2 加固）；`livekit/client.go` token 固定 1h（🔴）；`livekit/client.go` 错误透传（🟡）；`srs/provider.go` registry nil 泛化错误（🟡）；`nats_bus.go` pending 内存丢失（🔴，Task 5 修复）；`nats_bus.go` deliverCh 溢出静默（🟡）；`queue.go` 双消费者重复落库（🟡）；`queue.go` 无死信队列（🟡）。

**Agent2：** `oauth_handler.go` error URL（🔵）；`oauth_handler.go` SameSite 弱化（🟡）；`message_handler.go` Search 无分页（🟡）；`cloudflare_handler.go` 越权（🔴，Task 8 修复）；`conversation_handler.go` MarkRead 单边未读（🟡）。

**Agent3：** `hub_disconnect.go` 清理竞态（🟡）；`nats_bus.go` dropped 计数无导出（🟡）；`cluster/leader.go` 选举空窗（🟡）；`oauth.go` 允许 http（🟡）；`oauth.go` 无重试（🔵）。

**Agent4：** `mediasoupSignal.ts` listener 泄漏（🟡）；`roomState.ts` 无操作仍返回新数组（🟡）；`roomState.ts` member 事件先于 room 事件丢失（🟡）；`roomState.ts` count 可负（🔵）；`useVoiceSession.ts` WeakSet 累积（🟡）；`useVoiceSession.ts` 重连集合不清理（🟡）。

### R5 明细（33）

**Agent1/Repo：** `message_repo.go` UpdateContent 无软删检查（🟡）；`message_repo.go` SoftDelete 用 Unscoped 硬删（🔴，复核为显式双写设计，保持现状）；`user_repo.go` 5s 超时登录失败（🟡）；`state_sync.go` lease 2min 幽灵成员（🟡）；`state_sync.go` heartbeat 竞态（🟡）；`mute_service.go` EnsureMentions N 次往返（🟡）。

**Agent2：** `permissions.ts` 硬编码兜底表（🟡）；`permissions.ts` atob 解码（🔴，Task 6 修复）；`permissions.ts` token 刷新期间权限不一致（🟡）；`mute_handler.go` 先广播后落库（🔴）；`mute_handler.go` 无重复禁言守卫（🟡）；`mute_handler.go` reason 无长度限制（🟡）。

**Agent3：** `state_sync.go` lease 不可配置（🟡）；`redis_store.go` CAS/TTL 复确认（🟡）；`hub_mute.go` identity 无缓存（🟡）。

**Agent4：** `permissions.ts` base64url 与 ban role（🔴，Task 6/7 修复）；`permissions.ts` profile 竞态（🟡）；`mediasoupSignal.ts` listener 泄漏复确认（🟡）；`roomState.ts` count 负值复确认（🟡）。

### R6 明细（27）

**Agent1/Config：** `config.go` JWT_KEY 默认值（🔴，用户排除）；`config.go` 启动无密钥校验（🟡，随排除项不实施）；`errors.go` 码位稀疏（🔵）；`response.go` 错误详情泄漏（🔴，Task 1 部分覆盖）；`response.go` 无 correlation id（🟡）。

**Agent2：** `middleware/domain.go` body 预读破坏绑定（🔴，Task 3 修复）；`response.go` 状态码枚举复确认（🔴，Task 1 修复）；`response.go` RATE_LIMITED 未实际使用（🟡）。

**Agent3：** `nats_bus.go` 指标不导出（🟡）；`scheduler.go` 节点不足返回 nil（🟡）；`scheduler.go` LoadPercent 缺失偏置（🔵）；`cluster_service.go` 空节点静默（🟡）。

**Agent4：** `permissions.ts` ban 与 claims 冲突（🔴，Task 7 修复）；`permissions.ts` 自定义角色零权限（🟡）。

**测试评估：** service 21/27、signal 17、ws 6、handler 15、bus 10、sfu 15、web 20、bot 13；无 `t.Parallel()`、无 `-race` CI 配置（🔵）。

> 注：R1-R6 明细为聚合后压缩记录，部分条目在复核中已确认为已修复或已有防护，最终以「已核实现状」与各任务为准。

## 附录 C: 🟡/🔵 低优先级处理进度

### 已核实无需改动

- `wsClient.ts` pendingAcks 已有 10s 超时 timer（`emitAck` 内 `setTimeout`），无需新增。
- `message_repo.go` SoftDelete 的 `Unscoped()` 是显式状态+软删双写设计，保持现状。
- `signal_handler.go` LiveKit 签名校验已有 5 分钟时间容差。
- `metrics` 已完整接入 `EventBusDroppedPublish`（monitor → snapshot → Prometheus gauge），无需新增。
- `hub.go` muteCache 按 identity 缓存是用户维度禁言设计（禁言与房间无关），非缺陷。

### 已修复（第一批，2026-08-10）

| 项 | 修复 | 文件 |
|----|------|------|
| 改密无复杂度校验 | 新密码最小 8 字符（ChangePassword/FirstChangePassword/ResetPassword） | `app/server/internal/service/auth_service.go` |
| `bot_` 前缀大小写绕过 | `strings.ToLower` 后再判前缀 | `app/server/internal/handler/auth_handler.go` |
| 房间列表忽略绑定错误 | 非法 JSON 返回 400，空 body 放行 | `app/server/internal/handler/room_handler.go` |
| WS 超限消息静默 | 超限记日志并关闭连接 | `app/server/internal/ws/client.go` |
| 邮箱模板重复解析 | 包级 `template.Must` 预解析 | `app/server/internal/service/email_service.go` |
| 禁言 reason 无上限 | 超过 500 runes 拒绝 | `app/server/internal/service/mute_service.go` |
| Cloudflare sessionId 无格式校验 | 非空 + 128 字符上限 | `app/server/internal/service/cloudflare_media_service.go` |

### 已修复（第二批，2026-08-10）

| 项 | 修复 | 文件 |
|----|------|------|
| 已删除消息仍可编辑 | Edit 检查 `DeletedAt.Valid` 后拒绝 | `app/server/internal/service/message_mutation.go` |
| chatStore 消息错误无提示 | 失败时 showToast | `app/web/src/stores/chatStore.ts` |
| domainStore 加载错误被吞 | 记录错误并保持 reject 语义 | `app/web/src/stores/domainStore.ts` |
| voiceChatStore 持久化加载竞态 | 用户已操作则不用旧值覆盖，读取失败有日志 | `app/web/src/stores/voiceChatStore.ts` |

### 已修复（第三批，2026-08-10）

| 项 | 修复 | 文件 |
|----|------|------|
| room_service 锁跨 Domain 阻塞 | 改为按 Domain 分片锁 | `app/server/internal/service/room_service.go` |
| SRS registry nil 错误语义 | 返回 `SFU_NOT_CONFIGURED` | `app/server/internal/sfu/providers/srs/provider.go` |
| NATS 重放失败即丢弃 | 有限重试 3 次 + 200ms backoff | `app/server/internal/bus/nats_publish.go` |
| 测试无 race 检测 | CI `test-server` 启用 `-race` | `.github/workflows/ci.yml` |

### 已修复（第四批，2026-08-10）

| 项 | 修复 | 文件 |
|----|------|------|
| fanout 每广播 N goroutine + 队头阻塞 | 并发池化（最多 8 worker，仍等待全部送达） | `app/server/internal/ws/fanout.go` |
| 语音转写 speaker 缓冲不清理 | 超过 30s 无新帧即丢弃缓冲 | `packages/bot/src/speech/openaiCompatiblePipeline.ts` |

### 已修复（第五批，2026-08-10）

| 项 | 修复 | 文件 |
|----|------|------|
| 消息发送 actor 身份未校验 | Send 拒绝空 Identity/UserUUID | `app/server/internal/service/message_service.go` |
| 注册密码无强度约束 | Register 复用最小 8 字符校验 | `app/server/internal/service/auth_service.go` |
| Domain 失效 payload 解析失败无日志 | 未知类型记录日志 | `app/server/internal/service/domain_service.go` |

### 已修复（第六批，2026-08-10）

| 项 | 修复 | 文件 |
|----|------|------|
| 集群调度节点不足静默 | ScaleServer 不足时返回 INTERNAL_ERROR | `app/server/internal/service/cluster_scaling.go` |
| Domain 创建无限流 | `/domain/create` 加 10/min 限流 | `app/server/internal/router/routes/domain/routes.go` |

### 已修复（第七批，2026-08-10）

| 项 | 修复 | 文件 |
|----|------|------|
| 提及关系逐条写入 | 联合唯一索引 + 批量 `OnConflict DoNothing` | `app/server/internal/model/message_mention.go`、`app/server/internal/repository/message_repo.go` |
| 独立测试串行执行 | WAL 测试启用 `t.Parallel` | `app/server/internal/bus/wal_test.go` |

### 已修复（第八批，2026-08-10）

| 项 | 修复 | 文件 |
|----|------|------|
| OAuth 自动建号不可控 | 新增 `OAUTH_AUTO_CREATE_USER`（默认 true，可关闭） | `app/server/internal/config/config.go`、`app/server/internal/service/oauth_service.go`、`app/server/server/gin.go` |
| Bot TTS 队列残留 | speak 完成后从 `_speakQueues` 清理（保留串行语义） | `packages/bot/src/runtime/botRunner.ts` |

### 已修复（第九批，2026-08-10）

| 项 | 修复 | 文件 |
|----|------|------|
| Redis 成员快照 TTL 固定 | `RedisStateStoreConfig.TTL` 可配置，默认保持 24h | `app/server/internal/bus/redis_store.go` |

### 已修复（第十批，2026-08-10）

| 项 | 修复 | 文件 |
|----|------|------|
| 独立测试串行 | `pkg/response_test` 启用 `t.Parallel` | `app/server/internal/pkg/response_test.go` |

### 已核实无需改动（第三批补充）

- `storage_service` 密钥已通过 `ToPublicStorageConfig` 脱敏（AccessKey/SecretKey 不回显），无需改动。
- `plugin_service` OnConfigUpdated 与 SaveConfig 顺序存在不同失败窗口，属架构权衡，暂不修改。

### 已核实无需改动（第四批补充）

- `roomState.applyMemberLeft` 已用 `Math.max(0, count-1)` 防负，且有测试覆盖。
- OAuth provider `ClientSecret` 已通过 `encryptOAuthProviderSecrets`/`decryptOAuthProviderSecrets` 加密存储。
- Redis 成员快照 version 为 Lua number（2^53 精度上限），理论溢出实际不可达，不修改。
- `EnsureMentions` 批量化为架构项：需联合唯一索引 + 数据迁移，暂缓。

### 已核实无需改动（第五批补充）

- 消息 `Search` 已有 limit 100/200 上限。
- 限流中间件已使用 `pkg.RATE_LIMITED` + 429。
- `IsMutedByIdentity` 未命中时回源 `IsMuted`，双缓存口径一致，注释说明禁言回源补详情。
- 切换 Domain 时 chatStore effect 已调用 `leaveTextRoom` 清理旧房间状态。
- `routerPromises` 已有 `.finally(() => delete(roomId))` 清理。
- `SetSyncWriteAllowed(cfg.IsAgent())` 已在 `server_jobs.go` 组合根接线。

### 已核实无需改动（第六批补充）

- WS 握手已被重构为仅 HttpOnly cookie 鉴权（不再支持 subprotocol token），多协议风险随重构消失。
- `CheckRoomLimit` 当前按成员合并视图计数；房间无独立观众计数字段，audience 语义属产品设计权衡，暂不修改。

### 已核实无需改动（第七批补充）

- 重复禁言已由 `MuteService.MuteUser` 走 `muteRepo.Upsert`，不会累积重复记录。
- OAuth 错误回显经 `oauthErrorMessage` 使用业务文案，不泄露内部 `err.Error()` 细节。

### 已核实无需改动（第八批补充）

- `wsClient.disconnect()` 已清除 `reconnectTimer`、置 `shouldReconnect=false` 并移除 `onclose`，重连竞态已有防护。

### 已核实无需改动（第九批补充）

- 前端 token 刷新已由 `authTransport.refreshSession` 的 `pendingRefresh` 单例 promise 去重，并发 401 共享一次刷新。
- `SFUService.GetJoinToken` 先做 Domain 成员检查再进密码/限流，非成员对不存在房间同样返回 `not a member`，无法通过错误差异枚举房间。

### 已核实无需改动（第十批补充 / 收尾）

- Job 队列已通过 `ConsumeChat`（Agent）与 `ConsumeRuntime`（Worker）做角色分流，`TargetNodeID` 按专属 subject 投递，无双消费者重复处理问题。
- `PutStream` 是 stream → (room, identity) 点映射，覆盖语义合理，无需成员快照式的 CAS 版本。
- `domain_service` 成员缓存窗口（30s TTL + NATS 失效兜底）与 `hub_disconnect`「解锁后 I/O」均为有意设计，代码有注释说明。
- 前端自定义角色零权限：后端 profile 下发的权限是权威来源，前端兜底表仅服务端未加载时使用，属设计。
- `cluster/leader` 5s TTL 锁与 `hub_mute` 逐房间 SFU 调用为架构决策，暂不调整。
- 其余 `t.Parallel` 扩展受全局 checker/共享内存 DB 约束，不适合盲目并行。

### 待处理候选（下一批）

- `room_service.go` service 级互斥锁跨 Domain 阻塞 → 域级分片锁
- `hub.go` muteCache key 无房间维度 → key 增加 room
- `bus` 发布失败事件指标未导出 → 接入 metrics 端点
- `nats_bus` 断线事件无重试/backoff → 重放策略细化
- `queue.go` 无死信队列 → JetStream DLQ 配置
- 前端 `voiceChatStore` 持久化加载竞态、`chatStore` 错误不提示、`domainStore` 吞错误
- `oauth_service` 自动建号绕过邮箱验证策略评估
- `message_repo.UpdateContent` 软删后仍可编辑 → 编辑时校验 deleted 状态
- 测试补 `t.Parallel()` 与 `-race` CI 配置

（第四批已将「fanout 并发/队头」与「speech 缓冲清理」移入已修复；「-race CI 配置」第三批已修复。）
