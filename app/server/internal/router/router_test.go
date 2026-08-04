package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"GOSpeak/internal/config"
	"GOSpeak/internal/model"
)

func testHandlers() *Handlers {
	return &Handlers{}
}

func TestWorkerModeDoesNotRegisterBusinessWrites(t *testing.T) {
	gin.SetMode(gin.TestMode)
	config.SetCurrent(&config.Config{ClusterRole: model.ClusterRoleWorker})

	r := gin.New()
	SetupRoutes(r, testHandlers())
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/v1/room/create", nil)
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for worker write route, got %d: %s", rec.Code, rec.Body.String())
	}
}
