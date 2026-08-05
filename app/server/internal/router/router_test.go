package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"GOSpeak/internal/config"
	"GOSpeak/internal/model"
	"GOSpeak/internal/repository"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func testHandlers() *Handlers {
	return &Handlers{}
}

// newTestRouter 返回带内存 SQLite 的路由实例，供无需真实 DB 的路由测试使用。
func newTestRouter(t *testing.T) http.Handler {
	t.Helper()
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	oldDB := repository.DB
	repository.DB = db
	t.Cleanup(func() {
		repository.DB = oldDB
		if sqlDB, err := db.DB(); err == nil && sqlDB != nil {
			_ = sqlDB.Close()
		}
	})

	return SetupRoutes(gin.New(), testHandlers())
}

func TestRouter_Readyz(t *testing.T) {
	router := newTestRouter(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/readyz", nil)
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
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
