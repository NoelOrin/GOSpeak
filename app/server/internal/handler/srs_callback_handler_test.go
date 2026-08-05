package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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
	h := NewSRSCallbackHandler(hub, "secret")

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
	h := NewSRSCallbackHandler(hub, "secret")
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
	h := NewSRSCallbackHandler(hub, "secret")
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
	h := NewSRSCallbackHandler(hub, "secret")
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
	h := NewSRSCallbackHandler(hub, "secret")
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
	h := NewSRSCallbackHandler(hub, "secret")
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
	h := NewSRSCallbackHandler(hub, "secret")
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
	h := NewSRSCallbackHandler(hub, "secret")
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
	h := NewSRSCallbackHandler(hub, "secret")
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
