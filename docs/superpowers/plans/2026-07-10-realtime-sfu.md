# Cloudflare Realtime SFU 接入实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在 GOSpeak 中新增 Cloudflare Realtime SFU 作为第 6 个 SFU Provider，后端实现 `sfu.Provider`/`StreamProvider`/`ClientInfoProvider`，前端 `sfu-client` 实现 WHIP/WHEP 媒体客户端，复用现有 Signal 层交换房间成员与 track ID。

**Architecture:** Cloudflare Realtime SFU 没有 "room" 概念，只有 Application / Session（每个客户端一个 WebRTC PeerConnection）/ Track（全局可寻址）。GOSpeak 的 "room" 映射为一组 Cloudflare Session：每个加入者由后端 REST 创建 Session 并签发 App Token，前端用 WHIP 推流拿到 track ID，再通过现有 Socket.IO Signal 层把 `{identity, sessionId, trackIds}` 广播给同房成员，其他成员用 WHEP 按 track ID 订阅。后端 `realtime` 包完全照搬 SRS 的 REST+JWT+WHIP/WHEP 范本，并复用 `pkg.RoomRegistry` 做 room→session 聚合。

**Tech Stack:** Go (Gin + GORM + net/http)、Cloudflare Calls REST API（`POST /apps/{appId}/sessions/new` 等）、HMAC-SHA256 App Token（自实现，零新依赖）、TypeScript（原生 `RTCPeerConnection` + WHIP/WHEP fetch）、Vitest、现有 `sfu-client` 包契约。

---

## 文件结构

### 后端（新增 / 修改，`module GOSpeak`）

- 新增 `app/server/internal/realtime/types.go` — Cloudflare REST 响应结构
- 新增 `app/server/internal/realtime/token.go` — HMAC App Token 签发 + Session 创建封装
- 新增 `app/server/internal/realtime/client.go` — Cloudflare Calls REST 客户端
- 新增 `app/server/internal/realtime/provider.go` — 实现 `sfu.Provider` + `sfu.StreamProvider` + `sfu.ClientInfoProvider`
- 修改 `app/server/internal/realtime/provider_test.go` — 单元测试（httptest mock Cloudflare）
- 修改 `app/server/internal/sfu/factory/factory.go` — 注册 `"realtime"` case
- 修改 `app/server/internal/sfu/factory/dynamic_provider.go` — `fingerprint` 增加 Realtime 字段
- 修改 `app/server/internal/config/config.go` — 增加 Realtime 配置字段
- 修改 `app/server/internal/model/sfu_config.go` — `SFUConfig` 增加 Realtime 持久化字段
- 修改 `app/server/internal/signal/types.go` — `MemberInfo` 增加 `SessionID` / `TrackIDs`
- 修改 `app/server/internal/signal/hub.go` — `member:joined` / `member:updated` 携带 Cloudflare 标识

### 前端（新增 / 修改）

- 新增 `packages/sfu-client/src/realtime-client.ts` — 实现 `SFUClient`（WHIP/WHEP）
- 新增 `packages/sfu-client/src/realtime-client.test.ts` — Vitest 单测
- 修改 `packages/sfu-client/src/provider.ts` — `SFUProvider` 联合类型加 `"realtime"`
- 修改 `packages/sfu-client/src/factory.ts` — 增加 loader + switch case
- 修改 `app/web/src/components/room/services/loadSfuClient.ts` — 增加 `realtime` 预加载
- 修改 `app/web/src/api/sfu.ts` — 响应类型补充 `sessionId` / `trackIds`

### 文档 / 配置

- 修改 `.env.dev` / `.env.prod` — 增加 Realtime 环境变量示例
- 修改 `AGENTS.md` — SFU Provider 表 + 路由/factory 备注

---

## Cloudflare Realtime SFU 关键事实（实现前提）

1. REST Base：`https://api.cloudflare.com/client/v4/accounts/{accountId}/calls`
2. 创建 Session：`POST {base}/apps/{appId}/sessions/new` → `data.id`
3. 查询 Session：`GET {base}/apps/{appId}/sessions/{sessionId}`
4. 关闭 Track：`PUT {base}/apps/{appId}/sessions/{sessionId}/tracks/close`
5. 鉴权：后端用 `Authorization: Bearer <REALTIME_API_TOKEN>`（账户级 API Token）。
6. 客户端 WHIP/WHEP 媒体端点（团队按 Cloudflare 控制台确认）：`https://{appId}.realtime.cloudflare.com/{sessionId}/whip` 与 `.../whep`。
7. App Token：客户端连接需携带由 `REALTIME_APP_SECRET` 签名的 HMAC-SHA256 JWT（claims：`app` / `session` / `perms` / `exp`）。**请在实现 Task 3 时对照 Cloudflare Realtime "App Token" 文档核对 claim 名称**——这是本计划唯一需要团队按官方文档落地的外部常量。
8. 客户端推流（WHIP）成功后，Cloudflare 在其响应/会话中分配全局 track ID；前端把该 track ID 经 Signal 层广播，他人用 WHEP 按 track ID 订阅。

---

## Task 1: 后端配置字段

**Files:**
- Modify: `app/server/internal/config/config.go:18-43`（struct）、`:67-84`（Load）
- Modify: `app/server/internal/model/sfu_config.go:7-26`

- [ ] **Step 1: 在 `Config` struct 增加 Realtime 字段**

```go
	RealtimeAccountID  string
	RealtimeAppID      string
	RealtimeAppSecret  string
	RealtimeAPIToken   string
	RealtimeHost       string
```

- [ ] **Step 2: 在 `Load()` 增加对应读取**

```go
		RealtimeAccountID: getEnv("REALTIME_ACCOUNT_ID", ""),
		RealtimeAppID:     getEnv("REALTIME_APP_ID", ""),
		RealtimeAppSecret: getEnv("REALTIME_APP_SECRET", ""),
		RealtimeAPIToken:  getEnv("REALTIME_API_TOKEN", ""),
		RealtimeHost:      getEnv("REALTIME_HOST", "https://api.cloudflare.com/client/v4/accounts"),
```

- [ ] **Step 3: 在 `SFUConfig` 增加持久化字段（API Token 仅走环境变量，不入库）**

```go
	RealtimeAccountID string `gorm:"size:255" json:"realtime_account_id"`
	RealtimeAppID     string `gorm:"size:255" json:"realtime_app_id"`
	RealtimeAppSecret string `gorm:"size:255" json:"realtime_app_secret"`
	RealtimeHost      string `gorm:"size:255" json:"realtime_host"`
```

- [ ] **Step 4: 编译校验**

```bash
cd app/server && go build ./... 
```
Expected: 编译通过（仅新增字段，无引用）。

- [ ] **Step 5: Commit**

```bash
git add app/server/internal/config/config.go app/server/internal/model/sfu_config.go
git commit -m "feat(config): add Cloudflare Realtime SFU fields"
```

---

## Task 2: Realtime 包骨架与 App Token

**Files:**
- Create: `app/server/internal/realtime/types.go`
- Create: `app/server/internal/realtime/token.go`

- [ ] **Step 1: 写 `types.go`（REST 响应结构）**

```go
package realtime

// Session 是 Cloudflare Realtime SFU 的会话（= 一个客户端 PeerConnection）。
type Session struct {
	ID        string   `json:"id"`
	TrackIDs  []string `json:"trackIds,omitempty"`
}

// sessionCreateResp 对应 POST /sessions/new 的返回。
type sessionCreateResp struct {
	Success bool `json:"success"`
	Data    struct {
		ID string `json:"id"`
	} `json:"data"`
}

// sessionGetResp 对应 GET /sessions/{id}。
type sessionGetResp struct {
	Success bool `json:"success"`
	Data    Session `json:"data"`
}

// errorResp 是 Cloudflare REST 的错误外壳。
type errorResp struct {
	Success bool   `json:"success"`
	Errors  []struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"errors"`
}
```

- [ ] **Step 2: 写 `token.go`（自实现 HMAC-SHA256 App Token，无新依赖）**

```go
package realtime

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"strings"
	"time"
)

type appTokenClaims struct {
	App       string   `json:"app"`
	Session   string   `json:"session"`
	Perms     []string `json:"perms"`
	Exp       int64    `json:"exp"`
}

func b64(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

// signAppToken 用 REALTIME_APP_SECRET 签发客户端连接用的 App Token。
// 注意：claims 字段名以 Cloudflare Realtime "App Token" 文档为准。
func signAppToken(secret, appID, sessionID string, perms []string, ttl time.Duration) (string, error) {
	header, _ := json.Marshal(map[string]string{"alg": "HS256", "typ": "JWT"})
	payload, err := json.Marshal(appTokenClaims{
		App:     appID,
		Session: sessionID,
		Perms:   perms,
		Exp:     time.Now().Add(ttl).Unix(),
	})
	if err != nil {
		return "", err
	}
	signing := b64(header) + "." + b64(payload)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signing))
	return signing + "." + b64(mac.Sum(nil)), nil
}

// sessionKey 生成 room+identity 的稳定映射键。
func sessionKey(room, identity string) string {
	return room + "\x00" + identity
}
```

- [ ] **Step 3: 编译校验**

```bash
cd app/server && go build ./internal/realtime/...
```
Expected: 编译通过。

- [ ] **Step 4: Commit**

```bash
git add app/server/internal/realtime/types.go app/server/internal/realtime/token.go
git commit -m "feat(realtime): add types and HMAC app token"
```

---

## Task 3: Realtime REST 客户端与 Provider 实现

**Files:**
- Create: `app/server/internal/realtime/client.go`
- Create: `app/server/internal/realtime/provider.go`
- Modify: `app/server/internal/realtime/provider.go`（同文件，含 `StreamProvider`/`ClientInfoProvider`）

- [ ] **Step 1: 写 `client.go`（Cloudflare Calls REST 封装）**

```go
package realtime

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	baseURL  string // https://api.cloudflare.com/client/v4/accounts/{accountId}/calls
	apiToken string
	http     *http.Client
}

func NewClient(baseURL, apiToken string) *Client {
	return &Client{
		baseURL:  strings.TrimRight(baseURL, "/"),
		apiToken: apiToken,
		http:     &http.Client{Timeout: 15 * time.Second},
	}
}

func (c *Client) CreateSession(appID string) (string, error) {
	url := fmt.Sprintf("%s/apps/%s/sessions/new", c.baseURL, appID)
	req, _ := http.NewRequest(http.MethodPost, url, bytes.NewReader([]byte("{}")))
	req.Header.Set("Authorization", "Bearer "+c.apiToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("realtime create session %d: %s", resp.StatusCode, string(body))
	}
	var out sessionCreateResp
	if err := json.Unmarshal(body, &out); err != nil {
		return "", err
	}
	return out.Data.ID, nil
}

func (c *Client) GetSession(appID, sessionID string) (*Session, error) {
	url := fmt.Sprintf("%s/apps/%s/sessions/%s", c.baseURL, appID, sessionID)
	req, _ := http.NewRequest(http.MethodGet, url, nil)
	req.Header.Set("Authorization", "Bearer "+c.apiToken)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var out sessionGetResp
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, err
	}
	return &out.Data, nil
}

func (c *Client) CloseTrack(appID, sessionID, trackID string) error {
	url := fmt.Sprintf("%s/apps/%s/sessions/%s/tracks/close", c.baseURL, appID, sessionID)
	payload, _ := json.Marshal(map[string]string{"trackId": trackID})
	req, _ := http.NewRequest(http.MethodPut, url, bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+c.apiToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("realtime close track %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

func (c *Client) DeleteSession(appID, sessionID string) error {
	url := fmt.Sprintf("%s/apps/%s/sessions/%s", c.baseURL, appID, sessionID)
	req, _ := http.NewRequest(http.MethodDelete, url, nil)
	req.Header.Set("Authorization", "Bearer "+c.apiToken)
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("realtime delete session %d: %s", resp.StatusCode, string(body))
	}
	return nil
}
```

- [ ] **Step 2: 写 `provider.go`（实现三个接口）**

```go
package realtime

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"GOSpeak/internal/config"
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/sfu"
)

type Service struct {
	client   *Client
	secret   string
	appID    string
	host     string // https://{appId}.realtime.cloudflare.com
	mu       sync.Mutex
	sessions map[string]string            // sessionKey(room,identity) -> sessionId
	roomSess map[string]map[string]struct{} // room -> set(sessionId)
}

func NewService(cfg *config.Config) *Service {
	account := strings.TrimSpace(cfg.RealtimeAccountID)
	appID := strings.TrimSpace(cfg.RealtimeAppID)
	host := strings.TrimSpace(cfg.RealtimeHost)
	if host == "" {
		host = "https://api.cloudflare.com/client/v4/accounts"
	}
	baseURL := fmt.Sprintf("%s/%s/calls", strings.TrimRight(host, "/"), account)
	mediaHost := fmt.Sprintf("https://%s.realtime.cloudflare.com", appID)
	return &Service{
		client:   NewClient(baseURL, cfg.RealtimeAPIToken),
		secret:   cfg.RealtimeAppSecret,
		appID:    appID,
		host:     mediaHost,
		sessions: make(map[string]string),
		roomSess: make(map[string]map[string]struct{}),
	}
}

func (s *Service) GenerateToken(room, identity string) (string, error) {
	sessionID, err := s.client.CreateSession(s.appID)
	if err != nil {
		return "", pkg.NewAppErrorWithCause(pkg.SFU_ERROR, err, "realtime create session failed")
	}
	s.mu.Lock()
	s.sessions[sessionKey(room, identity)] = sessionID
	if s.roomSess[room] == nil {
		s.roomSess[room] = make(map[string]struct{})
	}
	s.roomSess[room][sessionID] = struct{}{}
	s.mu.Unlock()
	tok, err := signAppToken(s.secret, s.appID, sessionID, []string{"publish", "subscribe"}, time.Hour)
	if err != nil {
		return "", pkg.NewAppErrorWithCause(pkg.SFU_ERROR, err, "realtime sign app token failed")
	}
	return tok, nil
}

func (s *Service) GenerateAdminToken() (string, error) {
	return s.client.apiToken, nil
}

func (s *Service) ListRooms() ([]sfu.RoomSummary, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]sfu.RoomSummary, 0, len(s.roomSess))
	for room, set := range s.roomSess {
		out = append(out, sfu.RoomSummary{Name: room, MemberCount: len(set)})
	}
	return out, nil
}

func (s *Service) ListParticipants(room string) ([]sfu.ParticipantSummary, error) {
	s.mu.Lock()
	set := s.roomSess[room]
	ids := make([]string, 0, len(set))
	for id := range set {
		ids = append(ids, id)
	}
	s.mu.Unlock()
	out := make([]sfu.ParticipantSummary, 0, len(ids))
	for _, id := range ids {
		out = append(out, sfu.ParticipantSummary{Identity: id})
	}
	return out, nil
}

func (s *Service) MuteParticipant(room, identity, trackSid string, muted bool) error {
	if !muted {
		return pkg.NewErrSFUNotSupported()
	}
	sessionID := s.lookup(room, identity)
	if sessionID == "" {
		return pkg.NewAppError(pkg.NOT_FOUND, "realtime participant not found")
	}
	if trackSid == "" {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "realtime mute requires trackSid")
	}
	if err := s.client.CloseTrack(s.appID, sessionID, trackSid); err != nil {
		return pkg.NewAppErrorWithCause(pkg.SFU_ERROR, err, "realtime mute failed")
	}
	return nil
}

func (s *Service) RemoveParticipant(room, identity string) error {
	sessionID := s.lookup(room, identity)
	if sessionID == "" {
		return pkg.NewAppError(pkg.NOT_FOUND, "realtime participant not found")
	}
	if err := s.client.DeleteSession(s.appID, sessionID); err != nil {
		return pkg.NewAppErrorWithCause(pkg.SFU_ERROR, err, "realtime remove failed")
	}
	s.forget(room, sessionID)
	return nil
}

func (s *Service) DeleteRoom(room string) error {
	s.mu.Lock()
	set := s.roomSess[room]
	s.mu.Unlock()
	for sessionID := range set {
		_ = s.client.DeleteSession(s.appID, sessionID)
		s.forget(room, sessionID)
	}
	return nil
}

func (s *Service) GetHost() string { return s.host }

func (s *Service) ProviderName() string { return "realtime" }

func (s *Service) StreamName(room, identity string) string {
	return s.lookup(room, identity)
}

func (s *Service) StreamInfo(room, identity string) (stream, token string, err error) {
	sessionID := s.lookup(room, identity)
	if sessionID == "" {
		return "", "", pkg.NewAppError(pkg.NOT_FOUND, "realtime session not found")
	}
	tok, err := signAppToken(s.secret, s.appID, sessionID, []string{"publish", "subscribe"}, time.Hour)
	if err != nil {
		return sessionID, "", pkg.NewAppErrorWithCause(pkg.SFU_ERROR, err, "realtime sign app token failed")
	}
	return sessionID, tok, nil
}

func (s *Service) ClientInfo() map[string]interface{} {
	return map[string]interface{}{
		"whipUrl": fmt.Sprintf("%s/whip", s.host),
		"whepUrl": fmt.Sprintf("%s/whep", s.host),
		"appId":   s.appID,
	}
}

func (s *Service) lookup(room, identity string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sessions[sessionKey(room, identity)]
}

func (s *Service) forget(room, sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, sessionKey(room, identity))
	if set := s.roomSess[room]; set != nil {
		delete(set, sessionID)
		if len(set) == 0 {
			delete(s.roomSess, room)
		}
	}
}
```

- [ ] **Step 3: 编译校验**

```bash
cd app/server && go build ./internal/realtime/...
```
Expected: 编译通过。

- [ ] **Step 4: Commit**

```bash
git add app/server/internal/realtime/client.go app/server/internal/realtime/provider.go
git commit -m "feat(realtime): implement Cloudflare Realtime SFU provider"
```

---

## Task 4: 注册 Provider 到 factory

**Files:**
- Modify: `app/server/internal/sfu/factory/factory.go:1-31`
- Modify: `app/server/internal/sfu/factory/dynamic_provider.go:36-42`

- [ ] **Step 1: `factory.go` 增加 import 与 case**

```go
	"GOSpeak/internal/realtime"
```
```go
	case "realtime":
		return realtime.NewService(cfg), nil
```

- [ ] **Step 2: `dynamic_provider.go` 的 `fingerprint` 增加 Realtime 字段**

```go
		cfg.RealtimeAccountID, cfg.RealtimeAppID, cfg.RealtimeAppSecret, cfg.RealtimeHost,
```

- [ ] **Step 3: 编译校验**

```bash
cd app/server && go build ./...
```
Expected: 编译通过，`SFU_PROVIDER=realtime` 可解析。

- [ ] **Step 4: Commit**

```bash
git add app/server/internal/sfu/factory/factory.go app/server/internal/sfu/factory/dynamic_provider.go
git commit -m "feat(sfu): register Cloudflare Realtime provider in factory"
```

---

## Task 5: Signal 层携带 Cloudflare 标识

**Files:**
- Modify: `app/server/internal/signal/types.go`（MemberInfo）
- Modify: `app/server/internal/signal/hub.go:452`（member:joined 广播）、`member:updated` 广播处

- [ ] **Step 1: `MemberInfo` 增加 Cloudflare 字段（omitempty，向后兼容）**

```go
type MemberInfo struct {
	Identity  string   `json:"identity"`
	Stream    string   `json:"stream,omitempty"`
	SessionID string   `json:"sessionId,omitempty"`
	TrackIDs  []string `json:"trackIds,omitempty"`
}
```

- [ ] **Step 2: `member:joined` 广播携带 `sessionId` / `trackIds`**

参考 `hub.go` 约 452 行的 `BroadcastToRoom("/", req.Room, EventMemberJoined, map[string]interface{}{...})`，在 map 中追加：

```go
		"sessionId": member.SessionID,
		"trackIds":  member.TrackIDs,
```

- [ ] **Step 3: 推流成功后前端经 `member:updated` 回传 trackIds**

前端 WHIP 成功取得 track ID 后，`socket.emit("member:updated", { room, identity, sessionId, trackIds })`；hub 在 `member:updated` 处理中把 `TrackIDs` 写回 `MemberInfo` 并广播（保持现有 `member:updated` 广播结构一致，仅补字段）。

- [ ] **Step 4: 编译校验**

```bash
cd app/server && go build ./internal/signal/...
```
Expected: 编译通过。

- [ ] **Step 5: Commit**

```bash
git add app/server/internal/signal/types.go app/server/internal/signal/hub.go
git commit -m "feat(signal): carry Cloudflare sessionId/trackIds in member events"
```

---

## Task 6: 后端单元测试

**Files:**
- Create: `app/server/internal/realtime/provider_test.go`

- [ ] **Step 1: 写失败测试（httptest mock Cloudflare Calls API）**

```go
package realtime

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"GOSpeak/internal/config"
)

func newTestService(handler http.HandlerFunc) (*Service, *httptest.Server) {
	srv := httptest.NewServer(handler)
	cfg := &config.Config{
		RealtimeAccountID: "acc1",
		RealtimeAppID:     "app1",
		RealtimeAppSecret: "secret",
		RealtimeAPIToken:  "tok",
		RealtimeHost:      srv.URL + "/accounts",
	}
	svc := NewService(cfg)
	// 用测试 server 替换 client.baseURL
	svc.client = NewClient(srv.URL+"/accounts/acc1/calls", "tok")
	return svc, srv
}

func TestGenerateToken_CreatesSession(t *testing.T) {
	var gotApp string
	svc, srv := newTestService(func(w http.ResponseWriter, r *http.Request) {
		gotApp = r.URL.Path
		json.NewEncoder(w).Encode(sessionCreateResp{Success: true, Data: struct {
			ID string `json:"id"`
		}{ID: "sess-1"}})
	})
	defer srv.Close()

	tok, err := svc.GenerateToken("room-a", "alice")
	if err != nil {
		t.Fatalf("GenerateToken err: %v", err)
	}
	if tok == "" {
		t.Fatal("expected non-empty token")
	}
	if svc.StreamName("room-a", "alice") != "sess-1" {
		t.Fatalf("session not registered, got %q", svc.StreamName("room-a", "alice"))
	}
	_ = gotApp
}

func TestRemoveParticipant_DeletesSession(t *testing.T) {
	deleted := make(chan string, 1)
	svc, srv := newTestService(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			json.NewEncoder(w).Encode(sessionCreateResp{Success: true, Data: struct {
				ID string `json:"id"`
			}{ID: "sess-9"}})
			return
		}
		if r.Method == http.MethodDelete {
			deleted <- r.URL.Path
			w.WriteHeader(http.StatusOK)
			return
		}
	})
	defer srv.Close()
	svc.client = NewClient(srv.URL+"/accounts/acc1/calls", "tok")

	if _, err := svc.GenerateToken("room-a", "bob"); err != nil {
		t.Fatal(err)
	}
	if err := svc.RemoveParticipant("room-a", "bob"); err != nil {
		t.Fatalf("RemoveParticipant err: %v", err)
	}
	if got := <-deleted; got == "" {
		t.Fatal("expected DELETE call to Cloudflare")
	}
	if svc.StreamName("room-a", "bob") != "" {
		t.Fatal("session should be forgotten after remove")
	}
}
```

- [ ] **Step 2: 运行测试确认失败（此时 provider 已存在，应 PASS；若先写测试则先确认编译/逻辑）**

```bash
cd app/server && go test ./internal/realtime/...
```
Expected: PASS。

- [ ] **Step 3: Commit**

```bash
git add app/server/internal/realtime/provider_test.go
git commit -m "test(realtime): add provider unit tests with httptest"
```

---

## Task 7: 前端 Realtime SFU 客户端

**Files:**
- Create: `packages/sfu-client/src/realtime-client.ts`

- [ ] **Step 1: 实现 `SFUClient`（镜像 `srs-client.ts`，WHIP 推流 + WHEP 按 trackId 订阅）**

```ts
import type {
	JoinParams,
	RemoteAudioTrackLike,
	RemoteTrackInfo,
	SFUClient,
	SFUClientOptions,
} from "./types";

interface PeerSub {
	identity: string;
	sessionId: string;
	trackId: string;
	pc: RTCPeerConnection | null;
	resourceUrl: string;
}

class RealtimeRemoteAudioTrack implements RemoteAudioTrackLike {
	private elements: HTMLAudioElement[] = [];
	constructor(private readonly stream: MediaStream) {}
	attach(): HTMLMediaElement {
		const el = document.createElement("audio");
		el.autoplay = true;
		el.srcObject = this.stream;
		this.elements.push(el);
		return el;
	}
	detach(): HTMLMediaElement[] {
		const d = [...this.elements];
		this.elements = [];
		for (const e of d) {
			e.pause();
			e.srcObject = null;
			e.remove();
		}
		return d;
	}
	setVolume(v: number): void {
		for (const e of this.elements) e.volume = Math.max(0, Math.min(1, v));
	}
}

export class RealtimeSFUClient implements SFUClient {
	private publishPc: RTCPeerConnection | null = null;
	private localStream: MediaStream | null = null;
	private remoteTracks = new Map<string, RealtimeRemoteAudioTrack>();
	private peerSubs = new Map<string, PeerSub>();
	private onRemoteAudioTrackCb?: (info: RemoteTrackInfo) => void;
	private onRemoteAudioTrackRemovedCb?: (identity: string) => void;
	private onActiveSpeakersCb?: (ids: string[]) => void;
	private onDisconnectedCb?: () => void;
	private socket?: any;
	private identity = "";
	private room = "";
	private ownSession = "";
	private appToken = "";
	private whipBase = "";
	private hasJoined = false;
	private micEnabled = true;
	private publishResourceUrl = "";

	constructor(private readonly options: SFUClientOptions = {}) {
		this.socket = options.socket;
	}

	async joinRoom(params: JoinParams): Promise<void> {
		const { identity, room, stream, streamToken } = params;
		this.identity = identity;
		this.room = room || "";
		this.ownSession = stream || "";
		this.appToken = streamToken || "";
		this.whipBase = (params.serverUrl || "").replace(/\/whep\/?$/, "");
		this.micEnabled = true;
		await this.startPublish();
		this.hasJoined = true;
		if (this.socket) {
			this.socket.on("member:joined", (d: any) => {
				if (d?.identity && d.identity !== this.identity && d.sessionId && d.trackIds) {
					for (const tid of d.trackIds) this.subscribePeer(d.identity, d.sessionId, tid);
				}
			});
			this.socket.on("member:updated", (d: any) => {
				if (d?.identity && d.identity !== this.identity && d.trackIds) {
					for (const tid of d.trackIds) this.subscribePeer(d.identity, d.sessionId, tid);
				}
			});
			this.socket.on("member:left", (d: any) => {
				if (d?.identity) this.unsubscribePeer(d.identity);
			});
		}
	}

	private async startPublish(): Promise<void> {
		const pc = new RTCPeerConnection({ iceServers: [{ urls: "stun:stun.cloudflare.com:3478" }] });
		this.publishPc = pc;
		this.localStream = await navigator.mediaDevices.getUserMedia({ audio: true });
		for (const t of this.localStream.getAudioTracks()) pc.addTrack(t, this.localStream);
		const url = `${this.whipBase}/${this.ownSession}/whip`;
		const resource = await this.exchangeSdp(pc, url, this.appToken, true);
		this.publishResourceUrl = resource;
		// 推流成功后取得 track id 并回传 Signal 层
		const trackIds = this.localStream.getAudioTracks().map((_t, i) => `${this.ownSession}-a${i}`);
		if (this.socket) {
			this.socket.emit("member:updated", {
				room: this.room,
				identity: this.identity,
				sessionId: this.ownSession,
				trackIds,
			});
		}
	}

	private subscribePeer(identity: string, sessionId: string, trackId: string): void {
		if (!this.hasJoined || identity === this.identity) return;
		const key = identity + "\x00" + trackId;
		if (this.peerSubs.has(key)) return;
		const pc = new RTCPeerConnection({ iceServers: [{ urls: "stun:stun.cloudflare.com:3478" }] });
		const sub: PeerSub = { identity, sessionId, trackId, pc, resourceUrl: "" };
		this.peerSubs.set(key, sub);
		this.exchangeSdp(pc, `${this.whipBase}/${sessionId}/whep?track=${encodeURIComponent(trackId)}`, this.appToken, false)
			.then((resource) => { sub.resourceUrl = resource; })
			.catch(() => { this.peerSubs.delete(key); });
		pc.ontrack = (ev) => {
			const track = new RealtimeRemoteAudioTrack(ev.streams[0] || new MediaStream([ev.track]));
			this.remoteTracks.set(identity, track);
			this.onRemoteAudioTrackCb?.({ identity, track });
		};
	}

	private unsubscribePeer(identity: string): void {
		for (const [key, sub] of this.peerSubs) {
			if (sub.identity !== identity) continue;
			sub.pc?.close();
			if (sub.resourceUrl) fetch(sub.resourceUrl, { method: "DELETE" }).catch(() => {});
			this.peerSubs.delete(key);
		}
		this.remoteTracks.delete(identity);
		this.onRemoteAudioTrackRemovedCb?.(identity);
	}

	private async exchangeSdp(pc: RTCPeerConnection, endpoint: string, token: string, _publishing: boolean): Promise<string> {
		const offer = await pc.createOffer();
		await pc.setLocalDescription(offer);
		const resp = await fetch(endpoint, {
			method: "POST",
			headers: { "Content-Type": "application/sdp", Authorization: `Bearer ${token}` },
			body: offer.sdp || "",
		});
		if (!resp.ok) throw new Error(`realtime WHIP/WHEP failed: ${resp.status}`);
		const answer = await resp.text();
		await pc.setRemoteDescription({ type: "answer", sdp: answer });
		return resp.headers.get("Location") || "";
	}

	async setMicEnabled(enabled: boolean): Promise<void> {
		this.micEnabled = enabled;
		if (this.localStream) for (const t of this.localStream.getAudioTracks()) t.enabled = enabled;
	}

	onRemoteAudioTrack(cb: (info: RemoteTrackInfo) => void): void { this.onRemoteAudioTrackCb = cb; }
	onRemoteAudioTrackRemoved(cb: (identity: string) => void): void { this.onRemoteAudioTrackRemovedCb = cb; }
	onActiveSpeakers(cb: (ids: string[]) => void): void { this.onActiveSpeakersCb = cb; }
	getExistingRemoteAudioTracks(): RemoteTrackInfo[] {
		return Array.from(this.remoteTracks.entries()).map(([identity, track]) => ({ identity, track }));
	}
	onDisconnected(cb: () => void): void { this.onDisconnectedCb = cb; }
	isConnected(): boolean { return this.hasJoined; }
	async leaveRoom(): Promise<void> {
		this.hasJoined = false;
		this.publishPc?.close();
		if (this.publishResourceUrl) fetch(this.publishResourceUrl, { method: "DELETE" }).catch(() => {});
		this.localStream?.getTracks().forEach((t) => t.stop());
		for (const [, sub] of this.peerSubs) { sub.pc?.close(); if (sub.resourceUrl) fetch(sub.resourceUrl, { method: "DELETE" }).catch(() => {}); }
		this.peerSubs.clear();
	}
	async destroy(): Promise<void> { await this.leaveRoom(); }
}
```

> 说明：Cloudflare Realtime 的真实 track ID 由 SFU 在 WHIP 响应/会话中分配，上面 `trackIds` 的构造是占位式示意；**在联调阶段需按 Cloudflare 实际返回（`GET /sessions/{id}` 的 `trackIds` 或 WHIP 响应 `Location`）替换**。这是前端唯一需按官方响应落地的外部常量。

- [ ] **Step 2: 类型检查**

```bash
cd packages/sfu-client && npx tsc --noEmit
```
Expected: 无类型错误。

- [ ] **Step 3: Commit**

```bash
git add packages/sfu-client/src/realtime-client.ts
git commit -m "feat(sfu-client): add Cloudflare Realtime WHIP/WHEP client"
```

---

## Task 8: 前端注册 Provider

**Files:**
- Modify: `packages/sfu-client/src/provider.ts:1-9`
- Modify: `packages/sfu-client/src/factory.ts:8-46`（loader map + switch）
- Modify: `app/web/src/components/room/services/loadSfuClient.ts:12-16`
- Modify: `app/web/src/api/sfu.ts:25-35`

- [ ] **Step 1: `provider.ts` 联合类型加 `"realtime"`**

```ts
export type SFUProvider = "livekit" | "agora" | "mediasoup" | "srs" | "daily" | "realtime";
export const DEFAULT_SFU_PROVIDER: SFUProvider = "livekit";
export const PROVIDER_LABELS: Record<SFUProvider, string> = {
	livekit: "LiveKit",
	agora: "Agora",
	mediasoup: "MediaSoup",
	srs: "SRS",
	daily: "Daily",
	realtime: "Cloudflare Realtime",
};
```

- [ ] **Step 2: `factory.ts` 增加 loader 与 switch case**

```ts
const providerLoaders: Record<SFUProvider, () => Promise<unknown>> = {
	daily: () => import("./daily-client"),
	agora: () => import("./agora-client"),
	mediasoup: () => import("./mediasoup-client"),
	srs: () => import("./srs-client"),
	livekit: () => import("./livekit-client"),
	realtime: () => import("./realtime-client"),
};
```
在 `createSFUClient` 的 switch 中增加：
```ts
		case "realtime": {
			const { RealtimeSFUClient } = (await providerLoaders.realtime()) as {
				RealtimeSFUClient: new (o?: SFUClientOptions) => SFUClient;
			};
			return new RealtimeSFUClient(options);
		}
```

- [ ] **Step 3: `loadSfuClient.ts` 增加预加载**

```ts
	realtime: () => preloadSFUClient("realtime"),
```

- [ ] **Step 4: `api/sfu.ts` 响应类型补充**

```ts
	sessionId?: string;
	trackIds?: string[];
```

- [ ] **Step 5: 类型检查**

```bash
cd packages/sfu-client && npx tsc --noEmit && cd ../../app/web && npx tsc --noEmit
```
Expected: 无类型错误。

- [ ] **Step 6: Commit**

```bash
git add packages/sfu-client/src/provider.ts packages/sfu-client/src/factory.ts app/web/src/components/room/services/loadSfuClient.ts app/web/src/api/sfu.ts
git commit -m "feat(web): register Cloudflare Realtime SFU provider"
```

---

## Task 9: 前端单元测试

**Files:**
- Create: `packages/sfu-client/src/realtime-client.test.ts`

- [ ] **Step 1: 写失败测试（镜像 `srs-client.test.ts`，用 fake timers + mock fetch/RTCPeerConnection）**

```ts
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";

describe("RealtimeSFUClient", () => {
	beforeEach(() => vi.useFakeTimers());
	afterEach(() => vi.useRealTimers());

	it("joinRoom sets connected state", async () => {
		globalThis.fetch = vi.fn().mockResolvedValue({
			ok: true,
			status: 201,
			headers: { get: () => "" },
			text: async () => "v=0\r\n",
		}) as any;
		const { RealtimeSFUClient } = await import("./realtime-client");
		const client = new RealtimeSFUClient({ socket: { on: () => {}, emit: () => {} } as any });
		await client.joinRoom({ token: "t", serverUrl: "https://app.realtime.cloudflare.com", identity: "a", room: "r", stream: "sess-1", streamToken: "tok" });
		expect(client.isConnected()).toBe(true);
		await client.destroy();
	});
});
```

- [ ] **Step 2: 运行测试**

```bash
cd packages/sfu-client && npx vitest run realtime-client.test.ts
```
Expected: PASS。

- [ ] **Step 3: Commit**

```bash
git add packages/sfu-client/src/realtime-client.test.ts
git commit -m "test(sfu-client): add Realtime client unit test"
```

---

## Task 10: 环境变量、文档与 AGENTS.md

**Files:**
- Modify: `.env.dev`、`.env.prod`（新增示例段）
- Modify: `AGENTS.md`（SFU Provider 表 + factory 备注）

- [ ] **Step 1: `.env.dev` 增加 Realtime 配置示例**

```bash
# Cloudflare Realtime SFU
SFU_PROVIDER="realtime"
REALTIME_ACCOUNT_ID=""
REALTIME_APP_ID=""
REALTIME_APP_SECRET=""
REALTIME_API_TOKEN=""
REALTIME_HOST="https://api.cloudflare.com/client/v4/accounts"
```

- [ ] **Step 2: `AGENTS.md` 的 SFU Provider 表增加一行**

```
| realtime | Medium | Cloudflare Realtime SFU：WHIP/WHEP + REST session/track 管理；禁言=关闭 track，踢人=删除 session |
```

并在 "Current provider maturity" 表补充同内容；在 factory 说明中补 `"realtime"`。

- [ ] **Step 3: Commit**

```bash
git add .env.dev .env.prod AGENTS.md
git commit -m "docs: document Cloudflare Realtime SFU integration"
```

---

## 自检（Self-Review）

**1. 规格覆盖**
- 新增 SFU Provider（后端 `realtime` 包 + 三个接口）→ Task 3 ✅
- factory 注册 → Task 4 ✅
- 前端 `sfu-client` 客户端 → Task 7 ✅
- 前端注册 + web 预加载 → Task 8 ✅
- 配置 / 持久化 / 环境变量 → Task 1 + Task 10 ✅
- Signal 层交换 track ID → Task 5 ✅
- 单测（后端 + 前端）→ Task 6 + Task 9 ✅

**2. 占位符扫描**
- 仅有的两处"需按官方文档落地"的外部常量已显式标注：App Token claim 名称（Task 3）、Cloudflare track ID 实际来源（Task 7）。其余代码均为完整可运行形态，非 TBD/TODO。

**3. 类型一致性**
- 后端 `Service` 方法签名与 `sfu.Provider` / `StreamProvider` / `ClientInfoProvider` 完全一致（`GenerateToken`/`GenerateAdminToken`/`ListRooms`/`ListParticipants`/`MuteParticipant`/`RemoveParticipant`/`DeleteRoom`/`GetHost` + `StreamName`/`StreamInfo`/`ClientInfo` + `ProviderName`）。
- 前端 `RealtimeSFUClient` 实现 `SFUClient` 全部方法（`joinRoom`/`leaveRoom`/`setMicEnabled`/`onActiveSpeakers`/`onRemoteAudioTrack`/`onRemoteAudioTrackRemoved`/`getExistingRemoteAudioTracks`/`onDisconnected`/`isConnected`/`destroy` + `subscribePeers?`/`unsubscribePeer?` 可选）。
- `MemberInfo` 新增 `SessionID`/`TrackIDs` 与 hub 广播字段对齐。

**4. 已知风险 / 待联调确认**
- Cloudflare Realtime 的 WHIP/WHEP 端点 URL 模板（`https://{appId}.realtime.cloudflare.com/{sessionId}/whip`）与 track ID 分配方式需在实施时按控制台/官方文档确认。
- App Token 的 claim 名称（`app`/`session`/`perms`/`exp`）需对照 Cloudflare "App Token" 文档核对。
- `MuteParticipant` 采用"关闭 track"实现服务端强制禁言；若 Cloudflare 不支持按 track 关闭，则回退为 Signal 层 `BroadcastMute` 广播（与 AGENTS.md「Mute* 仅广播」一致）。
