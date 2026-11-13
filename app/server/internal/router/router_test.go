package router

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"GOSpeak/internal/handler"
)

func testHandlers() *Handlers {
	return &Handlers{
		SystemVersion: handler.NewSystemVersionHandler(),
	}
}

// newTestRouter 返回带内存 SQLite 的路由实例，供无需真实 DB 的路由测试使用。
func newTestRouter(t *testing.T) http.Handler {
	t.Helper()
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil && sqlDB != nil {
			_ = sqlDB.Close()
		}
	})

	handlers := testHandlers()
	handlers.ReadyCheck = func() error {
		if _, err := db.DB(); err != nil {
			return err
		}
		return nil
	}

	return SetupRoutes(gin.New(), handlers)
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

// 版本端点必须公开可读：登录页与「关于」页在未登录时也要展示后端版本。
// 更新检查（/system/update/*）保持 admin-only，两者互不影响。
func TestRouter_SystemVersionIsPublic(t *testing.T) {
	router := newTestRouter(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/v1/system/version", nil)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for public version endpoint, got %d (%s)", rec.Code, rec.Body.String())
	}

	var body struct {
		Code int `json:"code"`
		Data struct {
			Version string `json:"version"`
			Commit  string `json:"commit"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Code != 0 {
		t.Fatalf("expected code 0, got %d", body.Code)
	}
	if body.Data.Version == "" {
		t.Fatal("expected a non-empty version")
	}
}
