# SRS 多流隔离与动态订阅 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** SRS provider 从单流共享改为每客户端独立流 + 动态订阅,支持多用户双向音频,带 callback HMAC 鉴权与 WHEP 退避重订阅。

**Architecture:** 后端 SRS Service 计算确定性流名 `gs-<base36(sha256(room:identity))[:12]>` + HMAC stream token;token API 注入 stream/streamToken;信令 `room:join:sfu` 携带 stream,服务端校验后广播 `member:joined`;SRS `on_publish`/`on_play` HTTP callback 经 HMAC + activeStreams 内存注册表鉴权;前端 srs-client per-peer WHEP 订阅 + 退避重试 + socket 听 member:joined/left。

**Tech Stack:** Go 1.24 (Gin/GORM/socket.io) · SolidJS/TypeScript · Vitest · SRS 5.0 (http_hooks callback)

**Spec:** `docs/superpowers/specs/2026-07-05-srs-multistream-isolation-design.md`

## Global Constraints

- Go 文件 `snake_case`,类型 `PascalCase`;import 三组(标准库/第三方/内部)空行分隔
- Service 方法返回 `(result, error)`,error 必为 `*pkg.AppError`;Handler 用 `pkg.HandleError`/`pkg.Fail`/`pkg.Success`
- 代码不加注释除非复杂逻辑;不用 emoji(仅文档可用)
- 不改 `sfu.Provider` 接口;LiveKit/MediaSoup/Daily/Agora client 不实现新增可选方法
- 新增 TS 接口方法/参数均为可选(`?`),其他 provider 忽略
- stream 名 ASCII-safe:`"gs-" + base36(sha256(room+":"+identity))[:12]`
- streamToken:`base36(hmac_sha256(SRS_SECRET, stream))[:16]`
- WHEP 退避:1s/2s/4s/8s/16s,max 5 次
- SRS callback 响应 `{"code":0}` 放行,`{"code":403}` 拒绝
- commit 规范 Conventional Commits

---

## File Structure

**新增:**
- `app/server/internal/srs/stream.go` — 流名 + token 生成/校验(纯函数)
- `app/server/internal/srs/stream_test.go` — stream.go 单测
- `app/server/internal/router/routes/srs/routes.go` — `/api/v1/srs/callback` 路由
- `app/server/internal/handler/srs_callback_handler.go` — callback HTTP handler
- `app/server/internal/handler/srs_callback_handler_test.go` — callback 单测
- `packages/sfu-client/vitest.config.ts` — vitest 配置
- `packages/sfu-client/src/srs-client.test.ts` — srs-client 单测

**修改:**
- `app/server/internal/signal/types.go` — MemberInfo + RoomRequest 加 Stream
- `app/server/internal/signal/hub.go` — OnRoomJoinSFU 校验/广播/ack stream + activeStreams
- `app/server/internal/srs/provider.go` — Service 暴露 GenerateStreamName/Token/Validate
- `app/server/internal/handler/signal_handler.go` — GetJoinToken 注入 stream/streamToken
- `app/server/internal/router/router.go` — 注册 srs 路由
- `app/server/server/gin.go` — DI 注入 SrsCallbackHandler
- `deploy/srs/srs.conf` — http_hooks callback
- `app/web/src/api/sfu.ts` — JoinTokenResponse 加 stream/streamToken
- `app/web/src/stores/socketStore.ts` — joinRoomSFU emit stream + member:joined/left 携带
- `app/web/src/components/room/hooks/useRoomJoinSession.ts` — 接线 subscribePeers
- `packages/sfu-client/src/types.ts` — joinRoom 加参 + subscribePeers/unsubscribePeer
- `packages/sfu-client/src/srs-client.ts` — per-peer 订阅重构
- `packages/sfu-client/package.json` — vitest devDep + test script

---

### Task 1: SRS 流名与 stream token 生成/校验

纯函数,无外部依赖。后续所有 task 的基础。

**Files:**
- Create: `app/server/internal/srs/stream.go`
- Test: `app/server/internal/srs/stream_test.go`

**Interfaces:**
- Produces: `GenerateStreamName(room, identity string) string`、`GenerateStreamToken(stream, secret string) string`、`ValidateStreamToken(stream, token, secret string) bool`

- [ ] **Step 1: Write failing test**

Create `app/server/internal/srs/stream_test.go`:

```go
package srs

import (
	"strings"
	"testing"
)

func TestGenerateStreamName_Deterministic(t *testing.T) {
	a := GenerateStreamName("room-1", "alice")
	b := GenerateStreamName("room-1", "alice")
	if a != b {
		t.Fatalf("same input should produce same stream: %q vs %q", a, b)
	}
}

func TestGenerateStreamName_DifferentInput(t *testing.T) {
	a := GenerateStreamName("room-1", "alice")
	b := GenerateStreamName("room-1", "bob")
	c := GenerateStreamName("room-2", "alice")
	if a == b {
		t.Fatal("different identity should produce different stream")
	}
	if a == c {
		t.Fatal("different room should produce different stream")
	}
}

func TestGenerateStreamName_Format(t *testing.T) {
	s := GenerateStreamName("room-1", "alice")
	if !strings.HasPrefix(s, "gs-") {
		t.Fatalf("stream should have gs- prefix: %q", s)
	}
	if len(s) != 3+12 {
		t.Fatalf("stream should be gs- + 12 chars, got len %d: %q", len(s), s)
	}
	for _, r := range s[3:] {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'z')) {
			t.Fatalf("stream suffix must be base36 [0-9a-z], got %q in %q", r, s)
		}
	}
}

func TestGenerateStreamName_ASCIISafe(t *testing.T) {
	s := GenerateStreamName("聊天室", "张三")
	for _, r := range s {
		if r > 127 {
			t.Fatalf("stream must be ASCII-safe, got non-ASCII %q in %q", r, s)
		}
	}
}

func TestGenerateStreamToken_AndValidate(t *testing.T) {
	stream := GenerateStreamName("room-1", "alice")
	secret := "deadbeef"
	tok := GenerateStreamToken(stream, secret)
	if tok == "" {
		t.Fatal("token should not be empty")
	}
	if !ValidateStreamToken(stream, tok, secret) {
		t.Fatal("valid token should validate")
	}
}

func TestValidateStreamToken_WrongSecret(t *testing.T) {
	stream := GenerateStreamName("room-1", "alice")
	tok := GenerateStreamToken(stream, "secret-a")
	if ValidateStreamToken(stream, tok, "secret-b") {
		t.Fatal("token with wrong secret should not validate")
	}
}

func TestValidateStreamToken_WrongStream(t *testing.T) {
	tok := GenerateStreamToken("gs-aaa", "secret")
	if ValidateStreamToken("gs-bbb", tok, "secret") {
		t.Fatal("token bound to different stream should not validate")
	}
}
```

- [ ] **Step 2: Run test to verify failure**

Run: `cd app/server && go test ./internal/srs/ -run TestGenerateStreamName -v`
Expected: FAIL — `GenerateStreamName undefined`

- [ ] **Step 3: Implement stream.go**

Create `app/server/internal/srs/stream.go`:

```go
package srs

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"math/big"
)

const streamNamePrefix = "gs-"

func GenerateStreamName(room, identity string) string {
	h := sha256.Sum256([]byte(room + ":" + identity))
	return streamNamePrefix + base36(h[:])[:12]
}

func GenerateStreamToken(stream, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(stream))
	sum := mac.Sum(nil)
	return base36(sum)[:16]
}

func ValidateStreamToken(stream, token, secret string) bool {
	expected := GenerateStreamToken(stream, secret)
	return hmac.Equal([]byte(expected), []byte(token))
}

func base36(b []byte) string {
	n := new(big.Int).SetBytes(b)
	var out []byte
	mod := big.NewInt(36)
	zero := big.NewInt(0)
	for n.Cmp(zero) > 0 {
		rem := new(big.Int).Mod(n, mod)
		n.Div(n, mod)
		out = append([]byte{toBase36(rem.Int64())}, out...)
	}
	if len(out) == 0 {
		return "0"
	}
	return string(out)
}

func toBase36(v int64) byte {
	if v < 10 {
		return byte('0' + v)
	}
	return byte('a' + v - 10)
}

var _ = binary.BigEndian
```

- [ ] **Step 4: Run tests to verify pass**

Run: `cd app/server && go test ./internal/srs/ -v`
Expected: PASS — all 7 tests green

- [ ] **Step 5: Commit**

```bash
git add app/server/internal/srs/stream.go app/server/internal/srs/stream_test.go
git commit -m "feat(srs): 流名与 stream token 生成/校验"
```

---

### Task 2: MemberInfo.Stream + RoomRequest.Stream + hub activeStreams registry

信令层携带 stream 字段 + 服务端校验 + 内存流注册表(供 callback 查询)。

**Files:**
- Modify: `app/server/internal/signal/types.go`
- Modify: `app/server/internal/signal/hub.go`
- Test: `app/server/internal/signal/hub_test.go` (append)

**Interfaces:**
- Consumes: `srs.GenerateStreamName(room, identity)` (Task 1)
- Produces: `Hub.RegisterStream(stream string)`、`Hub.UnregisterStream(stream string)`、`Hub.IsStreamActive(stream string) bool`;`MemberInfo.Stream string json:"stream,omitempty"`;`RoomRequest.Stream string json:"stream,omitempty"`

- [ ] **Step 1: Write failing test**

Append to `app/server/internal/signal/hub_test.go`:

```go
func TestOnRoomJoinSFU_StoresAndBroadcastsStream(t *testing.T) {
	hub := NewHub(nil, nil, nil, nil)
	server := newMockServer()
	hub.server = server

	creator := newMockConn("creator-socket")
	hub.OnRoomCreate(creator, `{"room":"r1"}`)

	member := newMockConn("mem-1")
	ackRaw, err := hub.OnRoomJoinSFU(member, `{"room":"r1","identity":"alice","stream":"gs-aaa"}`)
	if err != nil {
		t.Fatalf("OnRoomJoinSFU error: %v", err)
	}
	var ack struct {
		OK      bool         `json:"ok"`
		Members []MemberInfo `json:"members"`
	}
	if err := json.Unmarshal([]byte(ackRaw), &ack); err != nil {
		t.Fatalf("parse ack: %v", err)
	}
	if !ack.OK {
		t.Fatal("ack should be ok")
	}
	if len(ack.Members) != 1 || ack.Members[0].Stream != "gs-aaa" {
		t.Fatalf("ack members should carry stream, got %+v", ack.Members)
	}

	hub.mu.RLock()
	stored := hub.rooms["r1"].Members["mem-1"].Stream
	hub.mu.RUnlock()
	if stored != "gs-aaa" {
		t.Fatalf("MemberInfo.Stream should be stored, got %q", stored)
	}

	broadcasted := false
	for _, ev := range server.events {
		if ev.name == EventMemberJoined {
			payload, _ := json.Marshal(ev.args)
			if strings.Contains(string(payload), "gs-aaa") {
				broadcasted = true
			}
		}
	}
	if !broadcasted {
		t.Fatal("member:joined should broadcast stream")
	}
}

func TestStreamRegistry(t *testing.T) {
	hub := NewHub(nil, nil, nil, nil)
	if hub.IsStreamActive("gs-x") {
		t.Fatal("unknown stream should not be active")
	}
	hub.RegisterStream("gs-x")
	if !hub.IsStreamActive("gs-x") {
		t.Fatal("registered stream should be active")
	}
	hub.UnregisterStream("gs-x")
	if hub.IsStreamActive("gs-x") {
		t.Fatal("unregistered stream should not be active")
	}
}
```

If `newMockServer` lacks an `events` slice field, inspect the existing mock in `hub_test.go` first and adapt the broadcasted-check to whatever the mock captures (e.g. a `broadcasts []struct{name string; args interface{}}` field). If no such field exists, add one to the mock. Add `"strings"` to imports if missing.

- [ ] **Step 2: Run test to verify failure**

Run: `cd app/server && go test ./internal/signal/ -run 'TestOnRoomJoinSFU_StoresAndBroadcastsStream|TestStreamRegistry' -v`
Expected: FAIL — `MemberInfo.Stream undefined`, `RegisterStream undefined`

- [ ] **Step 3: Add Stream field to types**

Modify `app/server/internal/signal/types.go`:

Add `Stream string \`json:"stream,omitempty"\`` to `MemberInfo`. Add `Stream string \`json:"stream,omitempty"\`` to `RoomRequest`.

Result:

```go
type MemberInfo struct {
	ID          string `json:"id"`
	Identity    string `json:"identity"`
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	Avatar      string `json:"avatar"`
	IsMuted     bool   `json:"isMuted"`
	IsMicMuted  bool   `json:"isMicMuted"`
	JoinedAt    int64  `json:"joinedAt"`
	Stream      string `json:"stream,omitempty"`
}

type RoomRequest struct {
	Room     string `json:"room"`
	Password string `json:"password,omitempty"`
	Identity string `json:"identity,omitempty"`
	Stream   string `json:"stream,omitempty"`
}
```

- [ ] **Step 4: Add activeStreams registry to Hub**

In `app/server/internal/signal/hub.go`, add a `activeStreams map[string]struct{}` field to the `Hub` struct, initialize it in `NewHub`, and add methods. Find the `Hub` struct definition and the `NewHub` constructor (read the file first to get exact field placement).

Add field to struct:
```go
	activeStreams map[string]struct{}
```

Initialize in `NewHub` (add to the `&Hub{...}` literal):
```go
		activeStreams: make(map[string]struct{}),
```

Add methods (reusing `h.mu`):
```go
func (h *Hub) RegisterStream(stream string) {
	h.mu.Lock()
	h.activeStreams[stream] = struct{}{}
	h.mu.Unlock()
}

func (h *Hub) UnregisterStream(stream string) {
	h.mu.Lock()
	delete(h.activeStreams, stream)
	h.mu.Unlock()
}

func (h *Hub) IsStreamActive(stream string) bool {
	h.mu.RLock()
	_, ok := h.activeStreams[stream]
	h.mu.RUnlock()
	return ok
}
```

- [ ] **Step 5: Carry stream through OnRoomJoinSFU**

In `app/server/internal/signal/hub.go` `OnRoomJoinSFU`, set `member.Stream = req.Stream` when constructing MemberInfo, include `stream` in the `member:joined` broadcast payload. The ack already returns `memberList` (via `getMembersLocked`), which will now include Stream since the field is on MemberInfo — verify `getMembersLocked` returns `[]MemberInfo` copying all fields.

Locate the MemberInfo construction (around line 322) and change to:
```go
	member := &MemberInfo{
		ID:       s.ID(),
		Identity: identity,
		JoinedAt: time.Now().UnixMilli(),
		Stream:   req.Stream,
	}
```

Locate the `EventMemberJoined` broadcast (around line 371) and add `"stream": req.Stream`:
```go
	h.server.BroadcastToNamespace("/", EventMemberJoined, map[string]interface{}{
		"room":     req.Room,
		"identity": req.Identity,
		"id":       s.ID(),
		"stream":   req.Stream,
	})
```

- [ ] **Step 6: Run tests to verify pass**

Run: `cd app/server && go test ./internal/signal/ -v`
Expected: PASS — all existing + 2 new tests green

- [ ] **Step 7: Commit**

```bash
git add app/server/internal/signal/types.go app/server/internal/signal/hub.go app/server/internal/signal/hub_test.go
git commit -m "feat(signal): MemberInfo/RoomRequest 携带 stream + activeStreams registry"
```

---

### Task 3: SRS callback HTTP handler + 路由 + DI

`on_publish`/`on_play`/`on_unpublish`/`on_stop` callback,鉴权 + 维护 activeStreams。

**Files:**
- Create: `app/server/internal/handler/srs_callback_handler.go`
- Create: `app/server/internal/handler/srs_callback_handler_test.go`
- Create: `app/server/internal/router/routes/srs/routes.go`
- Modify: `app/server/internal/router/router.go`
- Modify: `app/server/server/gin.go`

**Interfaces:**
- Consumes: `srs.ValidateStreamToken(stream, token, secret)` (Task 1)、`Hub.IsStreamActive`/`RegisterStream`/`UnregisterStream` (Task 2)、`*srs.Service` for secret access
- Produces: `SRSCallbackHandler.HandleCallback(c *gin.Context)`、route `POST /api/v1/srs/callback`

**SRS callback payload** (form-encoded POST from SRS 5.0):
- `action`: `on_publish`|`on_unpublish`|`on_play`|`on_stop`
- `stream`: stream name (may carry `app/` prefix like `live/gs-xxx` — strip leading `live/`)
- `param`: URL query string from WHIP/WHEP URL, e.g. `app=live&stream=gs-xxx&token=yyy`

- [ ] **Step 1: Write failing test**

Create `app/server/internal/handler/srs_callback_handler_test.go`:

```go
package handler

import (
	"net/url"
	"strings"
	"testing"

	"GOSpeak/internal/signal"
)

func newCallbackHub() *signal.Hub {
	return signal.NewHub(nil, nil, nil, nil)
}

func TestSrsCallback_OnPublish_ValidToken_RegistersStream(t *testing.T) {
	hub := newCallbackHub()
	h := NewSRSCallbackHandler(hub, "secret")

	stream := "gs-aaa"
	tok := srsStreamTokenForTest(stream, "secret")
	form := url.Values{
		"action": {"on_publish"},
		"stream": {"live/" + stream},
		"param":  {"app=live&stream=" + stream + "&token=" + tok},
	}
	w := postForm(t, h, form)
	if !strings.Contains(w.Body.String(), `"code":0`) {
		t.Fatalf("valid on_publish should return code 0, got %s", w.Body.String())
	}
	if !hub.IsStreamActive(stream) {
		t.Fatal("stream should be registered after valid on_publish")
	}
}

func TestSrsCallback_OnPublish_InvalidToken_Rejects(t *testing.T) {
	hub := newCallbackHub()
	h := NewSRSCallbackHandler(hub, "secret")
	form := url.Values{
		"action": {"on_publish"},
		"stream": {"live/gs-aaa"},
		"param":  {"app=live&stream=gs-aaa&token=wrong"},
	}
	w := postForm(t, h, form)
	if !strings.Contains(w.Body.String(), `"code":403`) {
		t.Fatalf("invalid token should return 403, got %s", w.Body.String())
	}
	if hub.IsStreamActive("gs-aaa") {
		t.Fatal("stream should NOT be registered after invalid on_publish")
	}
}

func TestSrsCallback_OnPlay_ActiveStream_Allows(t *testing.T) {
	hub := newCallbackHub()
	hub.RegisterStream("gs-bbb")
	h := NewSRSCallbackHandler(hub, "secret")
	form := url.Values{
		"action": {"on_play"},
		"stream": {"gs-bbb"},
		"param":  {"app=live&stream=gs-bbb"},
	}
	w := postForm(t, h, form)
	if !strings.Contains(w.Body.String(), `"code":0`) {
		t.Fatalf("on_play active stream should return 0, got %s", w.Body.String())
	}
}

func TestSrsCallback_OnPlay_InactiveStream_Rejects(t *testing.T) {
	hub := newCallbackHub()
	h := NewSRSCallbackHandler(hub, "secret")
	form := url.Values{
		"action": {"on_play"},
		"stream": {"gs-ccc"},
		"param":  {"app=live&stream=gs-ccc"},
	}
	w := postForm(t, h, form)
	if !strings.Contains(w.Body.String(), `"code":403`) {
		t.Fatalf("on_play inactive stream should return 403, got %s", w.Body.String())
	}
}

func TestSrsCallback_OnUnpublish_UnregistersStream(t *testing.T) {
	hub := newCallbackHub()
	hub.RegisterStream("gs-ddd")
	h := NewSRSCallbackHandler(hub, "secret")
	form := url.Values{
		"action": {"on_unpublish"},
		"stream": {"gs-ddd"},
		"param":  {"app=live&stream=gs-ddd"},
	}
	postForm(t, h, form)
	if hub.IsStreamActive("gs-ddd") {
		t.Fatal("stream should be unregistered after on_unpublish")
	}
}
```

Helper additions to the test file (or inline if a test harness already exists in the package — check `handler` package existing `_test.go` for `postForm` helpers first):

```go
// srsStreamTokenForTest mirrors srs.GenerateStreamToken without import cycle in test name.
// Use the real package: import "GOSpeak/internal/srs" and call srs.GenerateStreamToken.
```

If no `postForm` helper exists in the `handler` test package, add one using `net/http/httptest` + `gin`:

```go
import (
	"net/http"
	"net/http/httptest"
	"net/url"

	"github.com/gin-gonic/gin"
	"GOSpeak/internal/srs"
)

func init() { gin.SetMode(gin.TestMode) }

func srsStreamTokenForTest(stream, secret string) string {
	return srs.GenerateStreamToken(stream, secret)
}

func postForm(t *testing.T, h *SRSCallbackHandler, form url.Values) *httptest.ResponseRecorder {
	t.Helper()
	r := gin.New()
	r.POST("/cb", h.HandleCallback)
	req := httptest.NewRequest(http.MethodPost, "/cb", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}
```

Consolidate imports across all helpers into the single test file.

- [ ] **Step 2: Run test to verify failure**

Run: `cd app/server && go test ./internal/handler/ -run TestSrsCallback -v`
Expected: FAIL — `NewSRSCallbackHandler undefined`, `SRSCallbackHandler undefined`

- [ ] **Step 3: Implement callback handler**

Create `app/server/internal/handler/srs_callback_handler.go`:

```go
package handler

import (
	"net/http"
	"strings"

	"GOSpeak/internal/signal"
	"GOSpeak/internal/srs"

	"github.com/gin-gonic/gin"
)

type SRSCallbackHandler struct {
	hub    *signal.Hub
	secret string
}

func NewSRSCallbackHandler(hub *signal.Hub, secret string) *SRSCallbackHandler {
	return &SRSCallbackHandler{hub: hub, secret: secret}
}

func (h *SRSCallbackHandler) HandleCallback(c *gin.Context) {
	action := c.PostForm("action")
	rawStream := c.PostForm("stream")
	stream := stripAppPrefix(rawStream)
	params := parseCallbackParams(c.PostForm("param"))

	switch action {
	case "on_publish":
		token := params["token"]
		if token == "" || !srs.ValidateStreamToken(stream, token, h.secret) {
			c.JSON(http.StatusOK, gin.H{"code": 403})
			return
		}
		h.hub.RegisterStream(stream)
		c.JSON(http.StatusOK, gin.H{"code": 0})
	case "on_unpublish", "on_stop":
		h.hub.UnregisterStream(stream)
		c.JSON(http.StatusOK, gin.H{"code": 0})
	case "on_play":
		if !h.hub.IsStreamActive(stream) {
			c.JSON(http.StatusOK, gin.H{"code": 403})
			return
		}
		c.JSON(http.StatusOK, gin.H{"code": 0})
	default:
		c.JSON(http.StatusOK, gin.H{"code": 0})
	}
}

func stripAppPrefix(s string) string {
	if i := strings.LastIndex(s, "/"); i >= 0 {
		return s[i+1:]
	}
	return s
}

func parseCallbackParams(param string) map[string]string {
	out := map[string]string{}
	if param == "" {
		return out
	}
	for _, kv := range strings.Split(param, "&") {
		if i := strings.Index(kv, "="); i >= 0 {
			out[kv[:i]] = kv[i+1:]
		}
	}
	return out
}
```

`pkg` not needed — callback returns raw `gin.H{"code": N}` per SRS protocol, not the project's `pkg.Response` shape.

- [ ] **Step 4: Add route module**

Create `app/server/internal/router/routes/srs/routes.go`:

```go
package srs

import (
	"GOSpeak/internal/handler"

	"github.com/gin-gonic/gin"
)

func Register(r *gin.RouterGroup, h *handler.SRSCallbackHandler) {
	r.POST("/callback", h.HandleCallback)
}
```

- [ ] **Step 5: Wire route + DI**

Modify `app/server/internal/router/router.go`:
- Add `SRSCallback *handler.SRSCallbackHandler` to the `Handlers` struct.
- In `SetupRoutes`, after `api := r.Group("/api/v1")`, add:
```go
	srsRoutes.Register(api.Group("/srs"), h.SRSCallback)
```
- Add import `"GOSpeak/internal/router/routes/srs"` aliased as `srsRoutes` (note: alias to avoid collision with `internal/srs`).

Modify `app/server/server/gin.go`:
- After `signalH := handler.NewSignalHandler(...)`, construct the callback handler. The SRS secret must come from config. Read `cfg.SRSSecret` (the config field already exists). Type-assert is unnecessary — `NewSRSCallbackHandler` takes `(hub, secret)` directly:
```go
	srsCallbackH := handler.NewSRSCallbackHandler(signalHub, cfg.SRSSecret)
```
- Add `SRSCallback: srsCallbackH` to the `&router.Handlers{...}` literal.

Read `gin.go` first to find the exact `Handlers` literal location and the `cfg` variable name.

- [ ] **Step 6: Run tests to verify pass**

Run: `cd app/server && go test ./internal/handler/ -run TestSrsCallback -v`
Expected: PASS — 5 tests green

Run full build: `cd app/server && go build ./...`
Expected: build succeeds

- [ ] **Step 7: Commit**

```bash
git add app/server/internal/handler/srs_callback_handler.go app/server/internal/handler/srs_callback_handler_test.go app/server/internal/router/routes/srs/routes.go app/server/internal/router/router.go app/server/server/gin.go
git commit -m "feat(srs): on_publish/on_play HTTP callback 鉴权 + 路由"
```

---

### Task 4: token API 注入 stream / streamToken

`GetJoinToken` 对 SRS provider 注入 stream + streamToken 字段。

**Files:**
- Modify: `app/server/internal/handler/signal_handler.go`

**Interfaces:**
- Consumes: `srs.GenerateStreamName`/`GenerateStreamToken` (Task 1);`*srs.Service` via type-assertion on `sfuProvider`
- Produces: token API response gains `stream` + `streamToken` fields (SRS only)

**Key decision:** `SignalHandler.sfuProvider` is typed `sfu.Provider`. To access SRS-specific stream functions, type-assert to `*srs.Service` inside `GetJoinToken`. Define a small interface to avoid hard import coupling — but spec says SRS-only, so a type-assertion is acceptable. Add a local interface for testability:

```go
type streamInfoProvider interface {
	StreamInfo(room, identity string) (stream, token string)
}
```

- [ ] **Step 1: Add StreamInfo to SRS Service**

Modify `app/server/internal/srs/provider.go`. Add a method on `*Service`:

```go
func (s *Service) StreamInfo(room, identity string) (stream, token string) {
	stream = GenerateStreamName(room, identity)
	token = GenerateStreamToken(stream, s.secret)
	return
}
```

- [ ] **Step 2: Inject into GetJoinToken**

Modify `app/server/internal/handler/signal_handler.go` `GetJoinToken`. After the existing `if provider, ok := h.sfuProvider.(sfuClientInfoProvider); ok { ... }` block (around line 90-95), add:

```go
	if sp, ok := h.sfuProvider.(streamInfoProvider); ok {
		stream, streamToken := sp.StreamInfo(req.Room, req.Identity)
		data["stream"] = stream
		data["streamToken"] = streamToken
	}
```

Add the interface declaration near `sfuClientInfoProvider` (top of file, after line 21):

```go
type streamInfoProvider interface {
	StreamInfo(room, identity string) (stream, token string)
}
```

- [ ] **Step 3: Run tests + build**

Run: `cd app/server && go build ./... && go test ./internal/handler/ ./internal/srs/ -v`
Expected: build OK, existing tests pass

- [ ] **Step 4: Manual verify (curl)**

Start server with `SFU_PROVIDER=srs SRS_SECRET=testsecret` and curl the token API to confirm `stream`/`streamToken` appear. (Skip if no running server in CI; record in report.)

- [ ] **Step 5: Commit**

```bash
git add app/server/internal/srs/provider.go app/server/internal/handler/signal_handler.go
git commit -m "feat(signal): token API 注入 SRS stream/streamToken"
```

---

### Task 5: SRS http_hooks callback 配置

SRS 配置加 `http_hooks`,callback 指向 backend。

**Files:**
- Modify: `deploy/srs/srs.conf`
- Modify: `docs/srs-selfhost-runbook.md` (note callback dependency)

**Interfaces:** None (config only)

- [ ] **Step 1: Add http_hooks to vhost**

Modify `deploy/srs/srs.conf`. Add inside `vhost __defaultVhost__ { rtc { ... } }`, a new `http_hooks` block:

```
    http_hooks {
        enabled         on;
        on_publish      http://host.docker.internal:8998/api/v1/srs/callback;
        on_unpublish    http://host.docker.internal:8998/api/v1/srs/callback;
        on_play         http://host.docker.internal:8998/api/v1/srs/callback;
        on_stop         http://host.docker.internal:8998/api/v1/srs/callback;
    }
```

`host.docker.internal` is already mapped in `deploy/docker-compose.example.yml` srs service `extra_hosts`. Backend port 8998 per root Dockerfile.

- [ ] **Step 2: Update runbook**

In `docs/srs-selfhost-runbook.md` 排查表, add row:

```
| SRS WHIP/WHEP 被 403 拒 | callback 不可达或 streamToken 错 | 确认 backend 起在 8998,SRS→host.docker.internal 网络;`SRS_SECRET` 与 server 一致 |
```

Add a note under §1: "SRS http_hooks 已配 callback 到 backend:8998,backend 必须先于 SRS publish 启动,否则 SRS fail-closed 拒所有 publish。"

- [ ] **Step 3: Verify SRS reloads config**

Run: `docker restart gospeak-srs && sleep 3 && docker logs --tail 20 gospeak-srs 2>&1 | grep -i hook`
Expected: log line showing http_hooks enabled (or no error). If SRS rejects config, fix syntax.

- [ ] **Step 4: Commit**

```bash
git add deploy/srs/srs.conf docs/srs-selfhost-runbook.md
git commit -m "feat(srs): http_hooks callback 配置 + runbook 排查"
```

---

### Task 6: 前端类型扩展 + JoinTokenResponse

TS 接口加可选 stream/streamToken + subscribePeers/unsubscribePeer。

**Files:**
- Modify: `app/web/src/api/sfu.ts`
- Modify: `packages/sfu-client/src/types.ts`

**Interfaces:**
- Produces: `JoinTokenResponse.stream?: string`、`JoinTokenResponse.streamToken?: string`;`SFUClient.joinRoom(token, url, identity, room?, stream?, streamToken?)`;`SFUClient.subscribePeers?(peers: PeerStream[])`;`SFUClient.unsubscribePeer?(identity: string)`;`PeerStream` type

- [ ] **Step 1: Add types**

Modify `packages/sfu-client/src/types.ts`. Add a `PeerStream` type and extend `joinRoom`;add optional `subscribePeers`/`unsubscribePeer` to the `SFUClient` interface.

Find `joinRoom(` (line ~88) and change signature to:

```typescript
	joinRoom(
		token: string,
		url: string,
		identity: string,
		room?: string,
		stream?: string,
		streamToken?: string,
	): Promise<void>;
```

Add near the top (after `RemoteTrackInfo`):

```typescript
export interface PeerStream {
	identity: string;
	stream: string;
}
```

Add to `SFUClient` interface (after `getExistingRemoteAudioTracks`):

```typescript
	subscribePeers?(peers: PeerStream[]): void;
	unsubscribePeer?(identity: string): void;
```

- [ ] **Step 2: Extend JoinTokenResponse**

Modify `app/web/src/api/sfu.ts` `JoinTokenResponse` (line ~21):

```typescript
export interface JoinTokenResponse {
	token: string;
	serverUrl: string;
	room: string;
	identity: string;
	provider?: SFUProvider;
	appId?: string;
	bridgeUrl?: string;
	whipUrl?: string;
	dailyDomain?: string;
	stream?: string;
	streamToken?: string;
}
```

- [ ] **Step 3: Typecheck**

Run: `cd packages/sfu-client && npx tsc --noEmit && cd /Users/noelorin/GOSpeak/app/web && npx tsc --noEmit`
Expected: PASS (existing providers ignore new optional params/methods — no errors)

- [ ] **Step 4: Commit**

```bash
git add packages/sfu-client/src/types.ts app/web/src/api/sfu.ts
git commit -m "feat(sfu-client): joinRoom 加 stream 参数 + subscribePeers/unsubscribePeer 接口"
```

---

### Task 7: socketStore 携带 stream + 返回 members

`joinRoomSFU` emit 带 stream,`member:joined`/`member:left` 转发 stream,ack members 透传给调用方。

**Files:**
- Modify: `app/web/src/stores/socketStore.ts`
- Modify: `app/web/src/components/room/hooks/useRoomJoinSession.ts`

**Interfaces:**
- Consumes: `JoinTokenResponse.stream` (Task 6)
- Produces: `joinRoomSFU(room, identity, stream?)` returns ack `{members: MemberInfo[]}` (members carry `stream?`);member:joined handler stores `stream`

- [ ] **Step 1: Extend joinRoomSFU signature + MemberInfo**

Modify `app/web/src/stores/socketStore.ts`.

Frontend `MemberInfo` (line ~10) — add optional `stream?`:
```typescript
export interface MemberInfo {
	id: string;
	identity: string;
	name: string;
	displayName: string;
	avatar: string;
	isMuted: boolean;
	isMicMuted: boolean;
	joinedAt: number;
	stream?: string;
}
```

Change `joinRoomSFU` (line ~355) to accept stream and emit it:

```typescript
function joinRoomSFU(room: string, identity: string, stream?: string) {
	return signalEmit(EVENTS.ROOM_JOIN_SFU, { room, identity, stream }).then((data) => {
		if (data.members) {
			const ackMembers: MemberInfo[] = data.members;
			setRooms((prev) => {
				const exists = prev.some((r) => r.name === data.room);
				if (!exists) {
					return [
						...prev,
						{
							id: 0,
							uuid: "",
							name: data.room,
							hasPassword: false,
							limit: 0,
							members: ackMembers,
							count: ackMembers.length,
							createdAt: Date.now(),
						},
					];
				}
				return prev.map((r) =>
					r.name === data.room
						? { ...r, members: ackMembers, count: ackMembers.length }
						: r,
				);
			});
		}
		emitActivity({
			type: "room_joined",
			room: data.room,
			identity: data.identity,
			timestamp: Date.now(),
		});
		return data;
	});
}
```

- [ ] **Step 2: Store stream in member:joined handler**

In the `MEMBER_JOINED` handler (line ~163), the new-member insert object — add `stream: data.stream`:

```typescript
adapter.onServerEvent(
	EVENTS.MEMBER_JOINED as string,
	(data: { room: string; identity: string; id: string; stream?: string }) => {
		console.log("[Socket] member:joined", data.identity);
		setRooms((prev) =>
			prev.map((r) =>
				r.name === data.room
					? {
							...r,
							count: r.count + 1,
							members: r.members.some((m) => m.id === data.id)
								? r.members
								: [
										...r.members,
										{
											id: data.id,
											identity: data.identity,
											name: "",
											displayName: "",
											avatar: "",
											isMuted: false,
											isMicMuted: false,
											joinedAt: Date.now(),
											stream: data.stream,
										},
									],
						}
					: r,
			),
		);
```

- [ ] **Step 3: Wire useRoomJoinSession**

Modify `app/web/src/components/room/hooks/useRoomJoinSession.ts`.

Find `socketStore.joinRoomSFU(data.room, data.identity)` (line ~217). Change to pass stream and capture ack, then call `subscribePeers`:

```typescript
						const ack = await raceAbort(
							socketStore.joinRoomSFU(data.room, data.identity, data.stream),
							signal,
						);
						if (abortIfCancelled(createdClient)) return;
						const peers = (ack?.members ?? [])
							.filter((m) => m.identity !== data.identity && m.stream)
							.map((m) => ({ identity: m.identity, stream: m.stream as string }));
						if (peers.length) {
							createdClient.subscribePeers?.(peers);
						}
```

Also pass stream/streamToken to `joinRoom` (line ~200):

```typescript
						await raceAbort(
							createdClient.joinRoom(
								data.token,
								sessionMeta.connectTarget,
								data.identity,
								data.room,
								data.stream,
								data.streamToken,
							),
							signal,
						);
```

- [ ] **Step 4: Typecheck**

Run: `cd /Users/noelorin/GOSpeak/app/web && npx tsc --noEmit`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add app/web/src/stores/socketStore.ts app/web/src/components/room/hooks/useRoomJoinSession.ts
git commit -m "feat(web): joinRoomSFU 携带 stream + member:joined 透传 + subscribePeers 接线"
```

---

### Task 8: srs-client per-peer 订阅 + WHEP 退避重试 + socket 监听

核心重构。每 peer 独立 WHEP PC + 退避重试 + socket 听 member:joined/left + 真实 identity 上报。

**Files:**
- Modify: `packages/sfu-client/src/srs-client.ts`
- Create: `packages/sfu-client/vitest.config.ts`
- Create: `packages/sfu-client/src/srs-client.test.ts`
- Modify: `packages/sfu-client/package.json`

**Interfaces:**
- Consumes: `SFUClient.joinRoom(...stream?, streamToken?)`, `subscribePeers?`, `unsubscribePeer?` (Task 6);`options.socket` (Socket.IO client);`member:joined`/`member:left` events
- Produces: refactored `SRSSFUClient` with `peerSubs: Map<identity, PeerSub>`, `subscribePeer`, `unsubscribePeer`, `subscribePeers`, `scheduleRetry`

- [ ] **Step 1: Add vitest to sfu-client**

Modify `packages/sfu-client/package.json` — add `vitest` to devDependencies and a test script:
```json
	"scripts": {
		"build": "tsc",
		"lint": "biome check",
		"dev": "tsc --watch",
		"test": "vitest run"
	},
```
Run `pnpm --filter @gospeak/sfu-client add -D vitest` (use the actual package name from `package.json` `name` field — read it first). If pnpm filter name unclear, `cd packages/sfu-client && pnpm add -D vitest`.

Create `packages/sfu-client/vitest.config.ts`:
```typescript
import { defineConfig } from "vitest/config";

export default defineConfig({
	test: {
		environment: "node",
		include: ["src/**/*.test.ts"],
		globals: false,
	},
});
```

- [ ] **Step 2: Write failing test**

Create `packages/sfu-client/src/srs-client.test.ts`. Mock `RTCPeerConnection`, `fetch`, `navigator.mediaDevices`, and a fake socket emitter:

```typescript
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { SRSSFUClient } from "./srs-client";

// RTCPeerConnection mock
const pcStates: Record<string, string> = {};
function makeMockPc() {
	const handlers: Record<string, (() => void)[]> = {};
	const pc: any = {
		iceConnectionState: "new",
		addTrack: vi.fn(),
		addTransceiver: vi.fn(),
		createOffer: vi.fn().mockResolvedValue({ type: "offer", sdp: "v=0\r\n" }),
		setLocalDescription: vi.fn().mockResolvedValue(undefined),
		setRemoteDescription: vi.fn().mockResolvedValue(undefined),
		addEventListener: vi.fn((ev: string, cb: () => void) => {
			(handlers[ev] ||= []).push(cb);
		}),
		removeEventListener: vi.fn(),
		close: vi.fn(),
		fireIce: (state: string) => {
			pc.iceConnectionState = state;
			(handlers.iceconnectionstatechange || []).forEach((cb) => cb());
		},
	};
	return pc;
}

beforeEach(() => {
	(globalThis as any).RTCPeerConnection = vi.fn(makeMockPc);
	(globalThis as any).navigator = {
		mediaDevices: {
			getUserMedia: vi.fn().mockResolvedValue({
				getAudioTracks: () => [{ kind: "audio" }],
			}),
		},
	};
	(globalThis as any).AudioContext = vi.fn(() => ({
		createMediaStreamSource: vi.fn(),
		createAnalyser: vi.fn(() => ({ fftSize: 0, getByteFrequencyData: () => {} })),
		close: vi.fn(),
	}));
});

afterEach(() => {
	vi.restoreAllMocks();
});

function makeFetch(whipOk: boolean, whepOk: boolean) {
	return vi.fn(async (url: string, opts: any) => {
		const isWhep = String(url).includes("/whep/");
		const ok = isWhep ? whepOk : whipOk;
		if (!ok) {
			throw new Error("Failed to fetch");
		}
		return {
			ok: true,
			status: 201,
			headers: { get: (k: string) => (k === "Location" ? "/rtc/v1/whip/?action=delete&token=t" : "") },
			text: async () => "v=0\r\n",
		};
	});
}

describe("SRSSFUClient subscribePeers", () => {
	it("subscribes peer and emits onRemoteAudioTrack with real identity", async () => {
		(globalThis as any).fetch = makeFetch(true, true);
		const client = new SRSSFUClient({});
		const onTrack = vi.fn();
		client.onRemoteAudioTrack(onTrack);
		await client.joinRoom("tok", "http://h:1985/rtc/v1/whip/", "alice", "room1", "gs-alice", "st");
		(client as any).subscribePeers([{ identity: "bob", stream: "gs-bob" }]);
		// Wait microtasks for WHEP exchange
		await new Promise((r) => setTimeout(r, 50));
		expect(onTrack).toHaveBeenCalledWith(
			expect.objectContaining({ identity: "bob" }),
		);
		await client.leaveRoom();
	});

	it("retries WHEP on failure up to 5 times", async () => {
		vi.useFakeTimers();
		(globalThis as any).fetch = makeFetch(true, false);
		const client = new SRSSFUClient({});
		await client.joinRoom("tok", "http://h:1985/rtc/v1/whip/", "alice", "room1", "gs-alice", "st");
		(client as any).subscribePeers([{ identity: "bob", stream: "gs-bob" }]);
		// 5 retries: 1s + 2s + 4s + 8s + 16s = 31s
		await vi.advanceTimerByTimeAsync(31_000);
		const removed = vi.fn();
		client.onRemoteAudioTrackRemoved(removed);
		// after 5 failures, peer removed
		expect(removed).not.toHaveBeenCalled(); // removed only fires via unsubscribePeer; retry-exhaust is internal
		vi.useRealTimers();
		await client.leaveRoom();
	});

	it("unsubscribePeer closes pc and emits onRemoteAudioTrackRemoved", async () => {
		(globalThis as any).fetch = makeFetch(true, true);
		const client = new SRSSFUClient({});
		const removed = vi.fn();
		client.onRemoteAudioTrackRemoved(removed);
		await client.joinRoom("tok", "http://h:1985/rtc/v1/whip/", "alice", "room1", "gs-alice", "st");
		(client as any).subscribePeers([{ identity: "bob", stream: "gs-bob" }]);
		await new Promise((r) => setTimeout(r, 50));
		(client as any).unsubscribePeer("bob");
		expect(removed).toHaveBeenCalledWith("bob");
		await client.leaveRoom();
	});

	it("does not subscribe self stream", async () => {
		(globalThis as any).fetch = makeFetch(true, true);
		const client = new SRSSFUClient({});
		const onTrack = vi.fn();
		client.onRemoteAudioTrack(onTrack);
		await client.joinRoom("tok", "http://h:1985/rtc/v1/whip/", "alice", "room1", "gs-alice", "st");
		(client as any).subscribePeers([{ identity: "alice", stream: "gs-alice" }]);
		await new Promise((r) => setTimeout(r, 50));
		expect(onTrack).not.toHaveBeenCalled();
		await client.leaveRoom();
	});
});
```

Note: the retry-exhaust test asserts `removed` NOT called (retry exhaustion is silent internally — only `unsubscribePeer` triggers `onRemoteAudioTrackRemoved`). Adjust if design says otherwise — re-read spec 「重试 5 次全失败 → 放弃订阅该 peer,onRemoteAudioTrackRemoved」— spec says exhausted retries DO call `onRemoteAudioTrackRemoved`. Fix the test:

```typescript
	it("retries WHEP on failure then removes peer after exhaustion", async () => {
		vi.useFakeTimers();
		(globalThis as any).fetch = makeFetch(true, false);
		const client = new SRSSFUClient({});
		const removed = vi.fn();
		client.onRemoteAudioTrackRemoved(removed);
		await client.joinRoom("tok", "http://h:1985/rtc/v1/whip/", "alice", "room1", "gs-alice", "st");
		(client as any).subscribePeers([{ identity: "bob", stream: "gs-bob" }]);
		await vi.advanceTimerByTimeAsync(31_000);
		expect(removed).toHaveBeenCalledWith("bob");
		vi.useRealTimers();
		await client.leaveRoom();
	});
```

Replace the earlier retry test with this corrected version.

- [ ] **Step 3: Run test to verify failure**

Run: `cd packages/sfu-client && pnpm test`
Expected: FAIL — `subscribePeers` not a function / joinRoom signature mismatch

- [ ] **Step 4: Refactor srs-client.ts**

This is the core change. Read the current `packages/sfu-client/src/srs-client.ts` fully first. Then rewrite:

1. Store `this.socket = options.socket` in constructor (follow mediasoup pattern).
2. `joinRoom` signature: `(token, url, identity, room?, stream?, streamToken?)`. Store `this.ownStream = stream` / `this.streamToken = streamToken`. WHIP URL: `appendStream(url, stream, streamToken, true)`. Remove the single `subscribePc` + its `exchangeSdp` call from `joinRoom` (subscribe is now per-peer, triggered by `subscribePeers`/`member:joined`).
3. After join success, if `this.socket`, register `member:joined`/`member:left` listeners calling `subscribePeer`/`unsubscribePeer`.
4. Add `peerSubs: Map<string, PeerSub>`.
5. Add `subscribePeer(identity, stream)`: skip if `identity === this.identity` or `peerSubs.has(identity)` or `stream === this.ownStream`. Create a recvonly PC, set `ontrack` to emit `onRemoteAudioTrackCb({identity, track})`, `ontrack ended`/ICE failed → `scheduleRetry`. Call `exchangeSdp(pc, whepUrl, token, false)` where `whepUrl = appendStream(url.replace(/\/whip\/?$/, "/whep/"), stream, "", false)` (no token for WHEP). On success store PeerSub. On failure `scheduleRetry`.
6. Add `scheduleRetry(identity, stream)`: if `retryCount > 5` → `unsubscribePeer` + `onRemoteAudioTrackRemovedCb(identity)` + return. Else `setTimeout(() => subscribePeer(identity, stream), 2^(retryCount-1)*1000)`. Store timer in PeerSub.
7. Add `subscribePeers(peers)`: `for (p of peers) subscribePeer(p.identity, p.stream)`.
8. Add `unsubscribePeer(identity)`: clear timer, close pc, DELETE resource, delete from map, `onRemoteAudioTrackRemovedCb(identity)`.
9. `leaveRoom`: iterate `peerSubs` → `unsubscribePeer` each; then existing publish cleanup; remove socket listeners.
10. ICE failed handler on subscribe PC: `unsubscribePeer` cleanup without `onRemoteAudioTrackRemoved`, then `scheduleRetry` from `retryCount=0` (treat as fresh). Distinguish: explicit `unsubscribePeer` (member:left) calls `onRemoteAudioTrackRemoved`; retry-triggered cleanup does not.

Helper:
```typescript
function appendStream(url: string, stream: string | undefined, token: string | undefined, withToken: boolean): string {
	if (!stream) return url;
	const sep = url.includes("?") ? "&" : "?";
	let q = `app=live&stream=${encodeURIComponent(stream)}`;
	if (withToken && token) q += `&token=${encodeURIComponent(token)}`;
	return url + sep + q;
}
```

PeerSub interface:
```typescript
interface PeerSub {
	identity: string;
	stream: string;
	pc: RTCPeerConnection;
	resourceUrl: string;
	retryCount: number;
	retryTimer: ReturnType<typeof setTimeout> | null;
}
```

Keep `exchangeSdp`, `deleteResource`, `waitForPublishIceConnected`, `cleanupPartialJoin`, `startAudioLevelLoop`, `setMicEnabled`, `onActiveSpeakers`, `onRemoteAudioTrack`, `onRemoteAudioTrackRemoved`, `getExistingRemoteAudioTracks`, `onDisconnected`, `onReconnecting`, `onReconnected`, `destroy` as before — only the subscribe path changes. `getExistingRemoteAudioTracks` returns `Array.from(peerSubs.entries()).map(([identity, sub]) => ({identity, track: <RemoteAudioTrackLike>...}))` — store a `SRSRemoteAudioTrack` per PeerSub keyed by identity (set in `ontrack`).

- [ ] **Step 5: Run tests to verify pass**

Run: `cd packages/sfu-client && pnpm test && npx tsc --noEmit`
Expected: PASS — 4 tests green, typecheck clean

- [ ] **Step 6: Commit**

```bash
git add packages/sfu-client/src/srs-client.ts packages/sfu-client/src/srs-client.test.ts packages/sfu-client/vitest.config.ts packages/sfu-client/package.json pnpm-lock.yaml
git commit -m "feat(srs-client): per-peer WHEP 订阅 + 退避重试 + socket 监听"
```

---

### Task 9: 端到端手动验证 + runbook 更新

非自动化。双标签浏览器 e2e + 文档定稿。

**Files:**
- Modify: `docs/srs-selfhost-runbook.md`

**Interfaces:** None

- [ ] **Step 1: Update runbook §4**

Rewrite `docs/srs-selfhost-runbook.md` §4 双标签验证 to reflect multi-stream:
- A/B 加入同房 → 互听
- 第三人加入 → 前两人各订阅第三人
- 一人离开 → 其他人收到 track removed
- 自身无回声(no self-subscribe)

Add a stream debug section: `docker logs gospeak-srs 2>&1 | grep "RTC whip publish"` should show distinct `stream=gs-...` per client.

- [ ] **Step 2: Run e2e manually**

Follow updated runbook §4. Record results (pass/fail per step) in the task report. If failures, debug via SRS logs + browser console before claiming done.

- [ ] **Step 3: Commit**

```bash
git add docs/srs-selfhost-runbook.md
git commit -m "docs(srs): 多流 e2e 验证步骤更新"
```

---

## Self-Review

**Spec coverage:**
- 流名 + token: Task 1 ✓
- MemberInfo.Stream + 信令携带: Task 2 ✓
- SRS callback 鉴权: Task 3 ✓
- token API 注入: Task 4 ✓
- http_hooks 配置: Task 5 ✓
- 前端类型 + JoinTokenResponse: Task 6 ✓
- socketStore + useRoomJoinSession: Task 7 ✓
- srs-client 重构(含退避重试、socket、真实 identity、self-subscribe 排除): Task 8 ✓
- e2e + 文档: Task 9 ✓
- activeStreams registry: Task 2 ✓
- WHEP 鉴权方案 A(on_play 查 activeStreams): Task 3 ✓
- 不影响其他 provider: 所有新接口方法可选(Task 6/8)✓

**Placeholder scan:** Task 3 helper `postForm` has full code; Task 8 has full interface + helper; no TBD. Task 5 config verbatim. Task 9 is intentionally manual (non-code).

**Type consistency:** `stream`/`streamToken` field names consistent across Go (`Stream`/json `stream`) and TS (`stream?`/`streamToken?`). `PeerStream` (TS) = `{identity, stream}`. `subscribePeers(peers: PeerStream[])` consistent Task 6/7/8. `Hub.RegisterStream/UnregisterStream/IsStreamActive` consistent Task 2/3. `GenerateStreamName/GenerateStreamToken/ValidateStreamToken` consistent Task 1/3/4. `StreamInfo(room, identity) (stream, token)` Task 4.

**Note:** Task 8 test for retry-exhaust: spec says exhausted retries call `onRemoteAudioTrackRemoved` — test corrected to assert `removed` called after 31s. ICE-failed retry path is distinct (no `onRemoteAudioTrackRemoved` on each retry attempt, only on member:left or final exhaustion).
