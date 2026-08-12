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

type recordingRoomListBroadcaster struct {
	domains []string
}

func (r *recordingRoomListBroadcaster) BroadcastRoomList(domainUUID string) {
	r.domains = append(r.domains, domainUUID)
}

func TestRoomHandler_Create_BroadcastsRoomList(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Room{}, &model.Domain{}, &model.DomainMember{}, &model.DomainRole{}, &model.DomainRolePermission{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	domain := &model.Domain{UUID: "domain-a", Name: "Chat", OwnerUUID: "owner-1"}
	if err := db.Create(domain).Error; err != nil {
		t.Fatalf("seed domain: %v", err)
	}
	if err := repository.SeedDefaultDomainRoles(db, domain.UUID); err != nil {
		t.Fatalf("seed roles: %v", err)
	}
	if err := db.Create(&model.DomainMember{DomainUUID: domain.UUID, UserUUID: "user-1", RoleName: model.DomainRoleMember}).Error; err != nil {
		t.Fatalf("seed member: %v", err)
	}

	middleware.SetDomainChecker(func(domainUUID, userUUID string) bool { return true })
	t.Cleanup(func() { middleware.SetDomainChecker(nil) })

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("username", "user-1")
		c.Set("user_uuid", "user-1")
		c.Next()
	})
	broadcaster := &recordingRoomListBroadcaster{}
	domainSvc := service.NewDomainService(repository.NewDomainRepository(db), repository.NewDomainRoleRepository(db))
	h := NewRoomHandler(service.NewRoomService(repository.NewRoomRepository(db)), nil, domainSvc)
	h.SetRoomListBroadcaster(broadcaster)
	r.POST("/api/v1/room/create", h.Create)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/room/create", strings.NewReader(`{"name":"lobby","domain_uuid":"domain-a"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if code := intCode(resp["code"]); code != 0 {
		t.Fatalf("expected code 0, got %d: %s", code, resp["msg"])
	}
	if len(broadcaster.domains) != 1 || broadcaster.domains[0] != "domain-a" {
		t.Fatalf("expected room list broadcast for domain-a, got %v", broadcaster.domains)
	}
}
