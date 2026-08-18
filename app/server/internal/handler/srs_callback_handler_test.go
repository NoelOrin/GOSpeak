package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"GOSpeak/internal/sfu"
	"GOSpeak/internal/sfu/providers/srs"
	"GOSpeak/internal/signal"

	"github.com/gin-gonic/gin"
)

func init() { gin.SetMode(gin.TestMode) }

func newCallbackHub() *signal.Hub {
	return signal.NewHub(nil, nil, nil, nil)
}

func srsStreamTokenForTest(stream, secret string) string {
	tok, err := srs.GenerateStreamToken(stream, secret)
	if err != nil {
		panic("srs.GenerateStreamToken failed in test: " + err.Error())
	}
	return tok
}

func postJSON(t *testing.T, h *SRSCallbackHandler, payload map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	r := gin.New()
	r.POST("/cb", h.HandleCallback)
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/cb", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestSrsCallback_OnPublish_ValidToken_RegistersStream(t *testing.T) {
	hub := newCallbackHub()
	h := NewSRSCallbackHandlerWithResolver(hub, func() string { return "secret" })

	stream := "gs-aaa"
	tok := srsStreamTokenForTest(stream, "secret")
	payload := map[string]string{
		"action": "on_publish",
		"stream": "live/" + stream,
		"param":  "app=live&stream=" + stream + "&token=" + tok,
	}
	w := postJSON(t, h, payload)
	if !strings.Contains(w.Body.String(), `"code":0`) {
		t.Fatalf("valid on_publish should return code 0, got %s", w.Body.String())
	}
	if !hub.IsStreamActive(stream) {
		t.Fatal("stream should be registered after valid on_publish")
	}
}

func TestSrsCallback_OnPublish_InvalidToken_Rejects(t *testing.T) {
	hub := newCallbackHub()
	h := NewSRSCallbackHandlerWithResolver(hub, func() string { return "secret" })
	payload := map[string]string{
		"action": "on_publish",
		"stream": "live/gs-aaa",
		"param":  "app=live&stream=gs-aaa&token=wrong",
	}
	w := postJSON(t, h, payload)
	// 鉴权失败必须返回非 0 code，让 SRS 拒绝推流。
	if !strings.Contains(w.Body.String(), `"code":1`) {
		t.Fatalf("invalid on_publish should return code 1, got %s", w.Body.String())
	}
	if hub.IsStreamActive("gs-aaa") {
		t.Fatal("stream should NOT be registered after invalid on_publish")
	}
}

func TestSrsCallback_OnPlay_ActiveStream_Allows(t *testing.T) {
	hub := newCallbackHub()
	hub.RegisterStream("gs-bbb")
	h := NewSRSCallbackHandlerWithResolver(hub, func() string { return "secret" })
	tok := srsStreamTokenForTest("gs-bbb", "secret")
	payload := map[string]string{
		"action": "on_play",
		"stream": "gs-bbb",
		"param":  "app=live&stream=gs-bbb&token=" + tok,
	}
	w := postJSON(t, h, payload)
	if !strings.Contains(w.Body.String(), `"code":0`) {
		t.Fatalf("on_play active stream should return 0, got %s", w.Body.String())
	}
}

func TestSrsCallback_OnPlay_MissingToken_Rejects(t *testing.T) {
	hub := newCallbackHub()
	hub.RegisterStream("gs-bbb")
	h := NewSRSCallbackHandlerWithResolver(hub, func() string { return "secret" })
	payload := map[string]string{
		"action": "on_play",
		"stream": "gs-bbb",
		"param":  "app=live&stream=gs-bbb",
	}
	w := postJSON(t, h, payload)
	// 缺失 token 必须拒绝，返回非 0 code。
	if !strings.Contains(w.Body.String(), `"code":1`) {
		t.Fatalf("on_play without token should return code 1, got %s", w.Body.String())
	}
}

func TestSrsCallback_ResolveSecret_UsesResolver(t *testing.T) {
	hub := newCallbackHub()
	h := NewSRSCallbackHandlerWithResolver(hub, func() string { return "dynamic-secret" })
	stream := "gs-dyn"
	tok := srsStreamTokenForTest(stream, "dynamic-secret")
	payload := map[string]string{
		"action": "on_publish",
		"stream": stream,
		"param":  "app=live&stream=" + stream + "&token=" + tok,
	}
	w := postJSON(t, h, payload)
	if !strings.Contains(w.Body.String(), `"code":0`) {
		t.Fatalf("resolver secret should validate publish, got %s", w.Body.String())
	}
}

func TestSrsCallback_OnPlay_InactiveStream_Rejects(t *testing.T) {
	hub := newCallbackHub()
	h := NewSRSCallbackHandlerWithResolver(hub, func() string { return "secret" })
	payload := map[string]string{
		"action": "on_play",
		"stream": "gs-ccc",
		"param":  "app=live&stream=gs-ccc",
	}
	w := postJSON(t, h, payload)
	// 不活跃流必须拒绝，返回非 0 code。
	if !strings.Contains(w.Body.String(), `"code":1`) {
		t.Fatalf("on_play inactive stream should return code 1, got %s", w.Body.String())
	}
}

func TestSrsCallback_OnUnpublish_UnregistersStream(t *testing.T) {
	hub := newCallbackHub()
	hub.RegisterStream("gs-ddd")
	h := NewSRSCallbackHandlerWithResolver(hub, func() string { return "secret" })
	payload := map[string]string{
		"action": "on_unpublish",
		"stream": "gs-ddd",
		"param":  "app=live&stream=gs-ddd&token=" + srsStreamTokenForTest("gs-ddd", "secret"),
	}
	postJSON(t, h, payload)
	if hub.IsStreamActive("gs-ddd") {
		t.Fatal("stream should be unregistered after on_unpublish")
	}
}

func TestSrsCallback_OnUnpublish_InvalidToken_Rejects(t *testing.T) {
	hub := newCallbackHub()
	hub.RegisterStream("gs-invalid")
	h := NewSRSCallbackHandlerWithResolver(hub, func() string { return "secret" })
	payload := map[string]string{
		"action": "on_unpublish",
		"stream": "gs-invalid",
		"param":  "app=live&stream=gs-invalid&token=wrong",
	}
	postJSON(t, h, payload)
	if !hub.IsStreamActive("gs-invalid") {
		t.Fatal("stream should remain registered after invalid on_unpublish")
	}
}

func TestSrsCallback_OnUnpublish_MissingToken_Rejects(t *testing.T) {
	hub := newCallbackHub()
	hub.RegisterStream("gs-notoken")
	h := NewSRSCallbackHandlerWithResolver(hub, func() string { return "secret" })
	payload := map[string]string{
		"action": "on_unpublish",
		"stream": "gs-notoken",
		"param":  "app=live&stream=gs-notoken",
	}
	postJSON(t, h, payload)
	if !hub.IsStreamActive("gs-notoken") {
		t.Fatal("stream should remain registered when on_unpublish token is missing")
	}
}

func TestSrsCallback_OnStop_DoesNotUnregister(t *testing.T) {
	hub := newCallbackHub()
	hub.RegisterStream("gs-eee")
	h := NewSRSCallbackHandlerWithResolver(hub, func() string { return "secret" })
	payload := map[string]string{
		"action": "on_stop",
		"stream": "gs-eee",
		"param":  "app=live&stream=gs-eee",
	}
	w := postJSON(t, h, payload)
	if !strings.Contains(w.Body.String(), `"code":0`) {
		t.Fatalf("on_stop should return code 0, got %s", w.Body.String())
	}
	if !hub.IsStreamActive("gs-eee") {
		t.Fatal("stream should remain registered after on_stop")
	}
}

func TestAuthorizeSRSPlay_RoomTokenSameRoom(t *testing.T) {
	stream := "gs-alice"
	tok, err := srs.GenerateToken("room-a", "bob", "secret")
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}
	ok := authorizeSRSPlay(stream, tok, "secret", func(s string) (string, bool) {
		if s != stream {
			return "", false
		}
		return "room-a", true
	})
	if !ok {
		t.Fatal("same-room room JWT should authorize play")
	}
}

func TestAuthorizeSRSPlay_RoomTokenWrongRoom(t *testing.T) {
	stream := "gs-alice"
	tok, err := srs.GenerateToken("room-b", "bob", "secret")
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}
	ok := authorizeSRSPlay(stream, tok, "secret", func(s string) (string, bool) {
		return "room-a", true
	})
	if ok {
		t.Fatal("cross-room room JWT must not authorize play")
	}
}

func TestAuthorizeSRSPlay_RoomTokenUnknownStream(t *testing.T) {
	tok, err := srs.GenerateToken("room-a", "bob", "secret")
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}
	ok := authorizeSRSPlay("gs-missing", tok, "secret", func(string) (string, bool) {
		return "", false
	})
	if ok {
		t.Fatal("unknown stream room mapping must not authorize play")
	}
}

func TestParseCallbackParams_DecodesQuery(t *testing.T) {
	got := parseCallbackParams("app=live&token=a%2Bb%2Fc&room=%E4%B8%AD%E6%96%87&dup=1&dup=2")
	if got["token"] != "a+b/c" {
		t.Errorf("expected decoded token, got %q", got["token"])
	}
	if got["room"] != "中文" {
		t.Errorf("expected decoded room, got %q", got["room"])
	}
	if got["dup"] != "1" {
		t.Errorf("expected first duplicate value, got %q", got["dup"])
	}
}

func TestParseCallbackParams_EmptyOrInvalidReturnsEmpty(t *testing.T) {
	if got := parseCallbackParams(""); len(got) != 0 {
		t.Fatalf("expected empty map for empty param, got %v", got)
	}
	if got := parseCallbackParams("%zz"); len(got) != 0 {
		t.Fatalf("expected empty map for invalid query, got %v", got)
	}
}

func TestSrsCallback_OnPublish_BlockedStream_Rejects(t *testing.T) {
	hub := newCallbackHub()
	h := NewSRSCallbackHandlerWithResolver(hub, func() string { return "secret" })
	store := newFakeMuteRuleStore()
	_ = store.Save(context.Background(), srs.PublishBlockKey("gs-aaa"), 1, 0)
	h.SetMuteRuleStore(store)

	stream := "gs-aaa"
	tok := srsStreamTokenForTest(stream, "secret")
	payload := map[string]string{
		"action": "on_publish",
		"stream": "live/" + stream,
		"param":  "app=live&stream=" + stream + "&token=" + tok,
	}
	w := postJSON(t, h, payload)
	if !strings.Contains(w.Body.String(), `"code":1`) {
		t.Fatalf("blocked on_publish should return code 1, got %s", w.Body.String())
	}
	if hub.IsStreamActive(stream) {
		t.Fatal("blocked stream must NOT be registered")
	}
}

func TestSrsCallback_OnPublish_CachedBlockedStream_Rejects(t *testing.T) {
	hub := newCallbackHub()
	h := NewSRSCallbackHandlerWithResolver(hub, func() string { return "secret" })
	shared := newFakeMuteRuleStore()
	cache := sfu.NewCachedMuteRuleStore(shared)
	if err := cache.Save(context.Background(), srs.PublishBlockKey("gs-cache"), 1, time.Minute); err != nil {
		t.Fatal(err)
	}
	h.SetMuteRuleStore(cache)

	stream := "gs-cache"
	tok := srsStreamTokenForTest(stream, "secret")
	payload := map[string]string{
		"action": "on_publish",
		"stream": "live/" + stream,
		"param":  "app=live&stream=" + stream + "&token=" + tok,
	}
	w := postJSON(t, h, payload)
	if !strings.Contains(w.Body.String(), `"code":1`) {
		t.Fatalf("cached blocked on_publish should return code 1, got %s", w.Body.String())
	}
	if hub.IsStreamActive(stream) {
		t.Fatal("cached blocked stream must NOT be registered")
	}
}

func TestSrsCallback_OnPublish_AfterUnmute_Allows(t *testing.T) {
	hub := newCallbackHub()
	h := NewSRSCallbackHandlerWithResolver(hub, func() string { return "secret" })
	store := newFakeMuteRuleStore()
	h.SetMuteRuleStore(store)
	stream := "gs-aaa"
	_ = store.Save(context.Background(), srs.PublishBlockKey(stream), 1, 0)
	_ = store.Delete(context.Background(), srs.PublishBlockKey(stream))

	tok := srsStreamTokenForTest(stream, "secret")
	payload := map[string]string{
		"action": "on_publish",
		"stream": "live/" + stream,
		"param":  "app=live&stream=" + stream + "&token=" + tok,
	}
	w := postJSON(t, h, payload)
	if !strings.Contains(w.Body.String(), `"code":0`) {
		t.Fatalf("unblocked on_publish should return code 0, got %s", w.Body.String())
	}
	if !hub.IsStreamActive(stream) {
		t.Fatal("stream should be registered after unmute")
	}
}

func TestSrsCallback_OnPublish_CrossInstanceUnmute_Allows(t *testing.T) {
	hub := newCallbackHub()
	h := NewSRSCallbackHandlerWithResolver(hub, func() string { return "secret" })
	shared := newFakeMuteRuleStore()
	cache := sfu.NewCachedMuteRuleStore(shared)
	stream := "gs-cross"
	if err := cache.Save(context.Background(), srs.PublishBlockKey(stream), 1, time.Minute); err != nil {
		t.Fatal(err)
	}
	h.SetMuteRuleStore(cache)
	// 另一实例 unmute 只删 shared；无 L1 遮挡，本实例立即可见。
	if err := shared.Delete(context.Background(), srs.PublishBlockKey(stream)); err != nil {
		t.Fatal(err)
	}
	if id, err := cache.Get(context.Background(), srs.PublishBlockKey(stream)); err != nil || id != 0 {
		t.Fatalf("after shared delete Get should see unmute (id=%d err=%v)", id, err)
	}

	tok := srsStreamTokenForTest(stream, "secret")
	payload := map[string]string{
		"action": "on_publish",
		"stream": "live/" + stream,
		"param":  "app=live&stream=" + stream + "&token=" + tok,
	}
	w := postJSON(t, h, payload)
	if !strings.Contains(w.Body.String(), `"code":0`) {
		t.Fatalf("cross-instance unmuted on_publish should return code 0, got %s", w.Body.String())
	}
	if !hub.IsStreamActive(stream) {
		t.Fatal("stream should be registered after cross-instance unmute")
	}
}

type failingMuteStore struct{}

func (failingMuteStore) Save(context.Context, string, int, time.Duration) error { return nil }
func (failingMuteStore) Get(context.Context, string) (int, error) {
	return 0, errors.New("kv down")
}
func (failingMuteStore) Delete(context.Context, string) error { return nil }
func (failingMuteStore) Backend() string                      { return "memory" }

func TestSrsCallback_OnPublish_MuteStoreError_Denies(t *testing.T) {
	hub := newCallbackHub()
	h := NewSRSCallbackHandlerWithResolver(hub, func() string { return "secret" })
	h.SetMuteRuleStore(failingMuteStore{})

	stream := "gs-fail"
	tok := srsStreamTokenForTest(stream, "secret")
	payload := map[string]string{
		"action": "on_publish",
		"stream": "live/" + stream,
		"param":  "app=live&stream=" + stream + "&token=" + tok,
	}
	w := postJSON(t, h, payload)
	// 黑名单存储故障必须 fail-closed：拒绝发布，而不是静默放行。
	if !strings.Contains(w.Body.String(), `"code":1`) {
		t.Fatalf("store error on_publish should deny (code 1), got %s", w.Body.String())
	}
	if hub.IsStreamActive(stream) {
		t.Fatal("stream should NOT be registered when mute store check fails")
	}
}


// fakeMuteRuleStore 是测试用 MuteRuleStore 实现（不进入生产代码，杜绝被误用作降级后端）。
type fakeMuteRuleStore struct {
	mu   sync.Mutex
	data map[string]fakeMuteEntry
}

type fakeMuteEntry struct {
	ruleID    int
	expiresAt time.Time
}

func newFakeMuteRuleStore() *fakeMuteRuleStore {
	return &fakeMuteRuleStore{data: map[string]fakeMuteEntry{}}
}

func (s *fakeMuteRuleStore) Backend() string { return "fake" }

func (s *fakeMuteRuleStore) Save(_ context.Context, key string, ruleID int, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if ruleID <= 0 {
		delete(s.data, key)
		return nil
	}
	e := fakeMuteEntry{ruleID: ruleID}
	if ttl > 0 {
		e.expiresAt = time.Now().Add(ttl)
	}
	s.data[key] = e
	return nil
}

func (s *fakeMuteRuleStore) Get(_ context.Context, key string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.data[key]
	if !ok {
		return 0, nil
	}
	if !e.expiresAt.IsZero() && time.Now().After(e.expiresAt) {
		delete(s.data, key)
		return 0, nil
	}
	return e.ruleID, nil
}

func (s *fakeMuteRuleStore) Delete(_ context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, key)
	return nil
}
