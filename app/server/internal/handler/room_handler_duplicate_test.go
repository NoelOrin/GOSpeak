package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"GOSpeak/internal/middleware"
	"GOSpeak/internal/model"
	"GOSpeak/internal/repository"
	"GOSpeak/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestRoomHandler_Create_RejectsDuplicateNameInSameDomain(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Room{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := db.Create(&model.Room{Name: "lobby", DomainUUID: "domain-a", CreatedBy: "user-1"}).Error; err != nil {
		t.Fatalf("seed room: %v", err)
	}

	middleware.SetDomainChecker(func(domainUUID, userUUID string) bool { return true })
	t.Cleanup(func() { middleware.SetDomainChecker(nil) })

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("username", "user-1")
		c.Set("user_uuid", "user-1")
		c.Set("domain_uuid", "domain-a")
		c.Next()
	})
	h := NewRoomHandler(service.NewRoomService(repository.NewRoomRepository(db)), nil, nil)
	r.POST("/api/v1/room/create", h.Create)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/room/create", strings.NewReader(`{"name":"lobby","domain_uuid":"domain-a"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if code := intCode(resp["code"]); code != 3002 {
		t.Fatalf("expected 3002 for duplicate room name, got %d: %s", code, resp["msg"])
	}
}
