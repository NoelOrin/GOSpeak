package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"GOSpeak/internal/pkg"
)

func TestGetJoinToken_WorkerURL(t *testing.T) {
	r := setupRouterFull(&mockSFU{}, func(domainUUID, userUUID string) bool {
		return true
	}, func(domainUUID string) (string, error) {
		if domainUUID != "domain-1" {
			t.Fatalf("unexpected domain uuid %q", domainUUID)
		}
		return "wss://worker.example", nil
	})

	body := `{"room":"test-room","domain_uuid":"domain-1"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/token", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	resp := parseResp(t, w.Body.String())
	if resp.Code != pkg.SUCCESS {
		t.Fatalf("expected code 0, got %d", resp.Code)
	}
	data := resp.Data.(map[string]interface{})
	if data["workerUrl"] != "wss://worker.example" {
		t.Fatalf("expected workerUrl, got %v", data["workerUrl"])
	}
	if data["sfuRoom"] != "domain-1:test-room" {
		t.Fatalf("expected sfuRoom 'domain-1:test-room', got %v", data["sfuRoom"])
	}
	if data["domain_uuid"] != "domain-1" {
		t.Fatalf("expected domain_uuid 'domain-1', got %v", data["domain_uuid"])
	}
}
