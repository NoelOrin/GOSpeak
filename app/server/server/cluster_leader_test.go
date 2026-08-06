package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"GOSpeak/internal/config"
	"GOSpeak/internal/model"
	"GOSpeak/internal/router"

	"github.com/gin-gonic/gin"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
)

func startLeaderTestNATS(t *testing.T) *nats.Conn {
	t.Helper()
	ns, err := server.NewServer(&server.Options{
		Host:      "127.0.0.1",
		Port:      -1,
		NoLog:     true,
		NoSigs:    true,
		JetStream: true,
		StoreDir:  t.TempDir(),
	})
	if err != nil {
		t.Fatalf("new nats server: %v", err)
	}
	ns.Start()
	if !ns.ReadyForConnections(5 * time.Second) {
		ns.Shutdown()
		t.Fatal("nats server not ready")
	}
	nc, err := nats.Connect(ns.ClientURL())
	if err != nil {
		ns.Shutdown()
		t.Fatalf("connect nats: %v", err)
	}
	t.Cleanup(func() {
		nc.Close()
		ns.Shutdown()
	})
	return nc
}

func TestAcquireAgentLeaderSingleWriter(t *testing.T) {
	nc := startLeaderTestNATS(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, ok, err := acquireAgentLeader(ctx, nc, "test", "agent-a")
	if err != nil {
		t.Fatalf("first acquire: %v", err)
	}
	if !ok {
		t.Fatal("expected first agent to acquire leader lock")
	}

	_, ok, err = acquireAgentLeader(ctx, nc, "test", "agent-b")
	if err != nil {
		t.Fatalf("second acquire: %v", err)
	}
	if ok {
		t.Fatal("expected second agent to be rejected")
	}

	_, ok, err = acquireAgentLeader(ctx, nil, "test", "agent-c")
	if err != nil || ok {
		t.Fatalf("nil nats must degrade without error: ok=%v err=%v", ok, err)
	}
}

func TestWorkerModeAPIFallback(t *testing.T) {
	gin.SetMode(gin.TestMode)
	config.SetCurrent(&config.Config{ClusterRole: model.ClusterRoleWorker})
	t.Cleanup(func() { config.SetCurrent(nil) })

	r := gin.New()
	router.SetupRoutes(r, &router.Handlers{Config: &config.Config{ClusterRole: model.ClusterRoleWorker}})

	cases := []struct {
		name   string
		method string
		path   string
		want   int
	}{
		{name: "post", method: http.MethodPost, path: "/api/v1/room/create", want: http.StatusForbidden},
		{name: "put", method: http.MethodPut, path: "/api/v1/room/update", want: http.StatusForbidden},
		{name: "patch", method: http.MethodPatch, path: "/api/v1/room/patch", want: http.StatusForbidden},
		{name: "delete", method: http.MethodDelete, path: "/api/v1/room/1", want: http.StatusForbidden},
		{name: "get", method: http.MethodGet, path: "/api/v1/room/list", want: http.StatusNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(tc.method, tc.path, nil)
			r.ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Fatalf("%s %s: got %d, want %d (body=%s)", tc.method, tc.path, rec.Code, tc.want, rec.Body.String())
			}
		})
	}

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/signal/rooms", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("registered signal route was shadowed by fallback: got %d, want 401 (body=%s)", rec.Code, rec.Body.String())
	}
}
