package metrics

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestMetricsHandlerExposesSnapshot(t *testing.T) {
	srv := New(func() Snapshot {
		return Snapshot{
			HubRoomCount:        3,
			HubParticipantCount: 7,
			DBConnected:         true,
			EventBusConnected:   true,
			ClusterReadyNodes:   2,
		}
	})

	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	for _, metric := range []string{
		"gospeak_up 1",
		"gospeak_ws_rooms 3",
		"gospeak_ws_participants 7",
		"gospeak_db_connected 1",
		"gospeak_eventbus_connected 1",
		"gospeak_cluster_ready_nodes 2",
	} {
		if !strings.Contains(body, metric) {
			t.Fatalf("metrics missing %q", metric)
		}
	}
}

func TestMiddlewareRecordsHTTPRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	srv := New(nil)
	r.Use(srv.Middleware())
	r.GET("/ping", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/ping", nil))

	labels := prometheus.Labels{"method": http.MethodGet, "path": "/ping", "status": "200"}
	if got := testutil.ToFloat64(srv.requestsTotal.With(labels)); got != 1 {
		t.Fatalf("expected 1 request, got %v", got)
	}
}

func TestRequireToken(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	h := RequireToken(inner, "secret")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without token, got %d", rec.Code)
	}

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	req.Header.Set("Authorization", "Bearer secret")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204 with token, got %d", rec.Code)
	}
}
