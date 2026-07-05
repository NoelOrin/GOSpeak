package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"GOSpeak/internal/signal"
	"GOSpeak/internal/srs"

	"github.com/gin-gonic/gin"
)

func init() { gin.SetMode(gin.TestMode) }

func newCallbackHub() *signal.Hub {
	return signal.NewHub(nil, nil, nil, nil)
}

func srsStreamTokenForTest(stream, secret string) string {
	return srs.GenerateStreamToken(stream, secret)
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
	payload := map[string]string{
		"action": "on_play",
		"stream": "gs-bbb",
		"param":  "app=live&stream=gs-bbb",
	}
	w := postJSON(t, h, payload)
	if !strings.Contains(w.Body.String(), `"code":0`) {
		t.Fatalf("on_play active stream should return 0, got %s", w.Body.String())
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
	if !strings.Contains(w.Body.String(), `"code":403`) {
		t.Fatalf("on_play inactive stream should return 403, got %s", w.Body.String())
	}
}

func TestSrsCallback_OnUnpublish_UnregistersStream(t *testing.T) {
	hub := newCallbackHub()
	hub.RegisterStream("gs-ddd")
	h := NewSRSCallbackHandler(hub, "secret")
	payload := map[string]string{
		"action": "on_unpublish",
		"stream": "gs-ddd",
		"param":  "app=live&stream=gs-ddd",
	}
	postJSON(t, h, payload)
	if hub.IsStreamActive("gs-ddd") {
		t.Fatal("stream should be unregistered after on_unpublish")
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
