package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
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

func newRoomDomainManageEnv(t *testing.T, userUUID, roleName, createdBy string, createRole bool, rolePerms []string) (*gorm.DB, *model.Domain, *service.DomainService, uint) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&model.Room{},
		&model.Domain{},
		&model.DomainMember{},
		&model.DomainRole{},
		&model.DomainRolePermission{},
		&model.Permission{},
		&model.RolePermission{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	domain := &model.Domain{Name: "Test", OwnerUUID: "owner-1"}
	if err := db.Create(domain).Error; err != nil {
		t.Fatalf("seed domain: %v", err)
	}
	if err := repository.SeedDefaultDomainRoles(db, domain.UUID); err != nil {
		t.Fatalf("seed roles: %v", err)
	}
	if createRole {
		if err := repository.NewDomainRoleRepository(db).CreateRoleWithPermissions(
			&model.DomainRole{DomainUUID: domain.UUID, Name: roleName},
			rolePerms,
		); err != nil {
			t.Fatalf("create role %s: %v", roleName, err)
		}
	}
	if err := db.Create(&model.DomainMember{DomainUUID: domain.UUID, UserUUID: userUUID, RoleName: roleName}).Error; err != nil {
		t.Fatalf("seed member: %v", err)
	}
	room := model.Room{Name: "lobby", DomainUUID: domain.UUID, CreatedBy: createdBy}
	if err := db.Create(&room).Error; err != nil {
		t.Fatalf("seed room: %v", err)
	}
	domainSvc := service.NewDomainService(repository.NewDomainRepository(db), repository.NewDomainRoleRepository(db))
	return db, domain, domainSvc, room.ID
}

func postRoomJSON(t *testing.T, r *gin.Engine, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func roomResponseCode(t *testing.T, w *httptest.ResponseRecorder) int {
	t.Helper()
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return intCode(resp["code"])
}

func TestRoomHandler_Update_AllowsDomainRolePermission(t *testing.T) {
	middleware.SetDomainChecker(func(_, _ string) bool { return true })
	t.Cleanup(func() { middleware.SetDomainChecker(nil) })

	db, _, domainSvc, roomID := newRoomDomainManageEnv(t, "mod-1", "moderator", "someone-else", true, []string{model.PermRoomUpdate})
	h := NewRoomHandler(service.NewRoomService(repository.NewRoomRepository(db)), nil, domainSvc)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("username", "mod-1")
		c.Set("user_uuid", "mod-1")
		c.Set("role", "user")
		c.Next()
	})
	r.POST("/room/update", h.Update)

	w := postRoomJSON(t, r, "/room/update", `{"id":`+strconv.FormatUint(uint64(roomID), 10)+`,"name":"renamed"}`)
	if code := roomResponseCode(t, w); code != 0 {
		t.Fatalf("expected code 0 without global permission, got %d: %s", code, w.Body.String())
	}
}

func TestRoomHandler_Delete_DomainPermissionDoesNotFallBackToGlobal(t *testing.T) {
	middleware.SetDomainChecker(func(_, _ string) bool { return true })
	t.Cleanup(func() { middleware.SetDomainChecker(nil) })

	db, _, domainSvc, roomID := newRoomDomainManageEnv(t, "guest-1", model.DomainRoleGuest, "someone-else", false, nil)

	permRepo := repository.NewPermissionRepository(db)
	for _, perm := range model.DefaultPermissions {
		if err := permRepo.CreateIfNotExists(&perm); err != nil {
			t.Fatalf("seed permission %s: %v", perm.Code, err)
		}
	}
	if err := permRepo.SeedRolePermissionsIfEmpty("user", []string{model.PermRoomDelete}); err != nil {
		t.Fatalf("seed global role permission: %v", err)
	}
	permSvc := service.NewPermissionService(permRepo)
	if err := permSvc.LoadCache(); err != nil {
		t.Fatalf("load permission cache: %v", err)
	}
	if !permSvc.HasPermission("user", model.PermRoomDelete) {
		t.Fatal("test precondition: global user role must have room:delete")
	}

	h := NewRoomHandler(service.NewRoomService(repository.NewRoomRepository(db)), permSvc, domainSvc)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("username", "guest-1")
		c.Set("user_uuid", "guest-1")
		c.Set("role", "user")
		c.Next()
	})
	r.POST("/room/delete", h.Delete)

	w := postRoomJSON(t, r, "/room/delete", `{"id":`+strconv.FormatUint(uint64(roomID), 10)+`}`)
	if code := roomResponseCode(t, w); code != 1013 {
		t.Fatalf("expected 1013 (no global fallback for domain room), got %d: %s", code, w.Body.String())
	}
}

func TestRoomHandler_Get_DomainReadPermissionRequired(t *testing.T) {
	middleware.SetDomainChecker(func(_, _ string) bool { return true })
	t.Cleanup(func() { middleware.SetDomainChecker(nil) })

	db, _, domainSvc, roomID := newRoomDomainManageEnv(t, "viewer-1", "viewer", "someone-else", true, nil)
	h := NewRoomHandler(service.NewRoomService(repository.NewRoomRepository(db)), nil, domainSvc)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("username", "viewer-1")
		c.Set("user_uuid", "viewer-1")
		c.Set("role", "user")
		c.Next()
	})
	r.POST("/room/get", h.Get)

	w := postRoomJSON(t, r, "/room/get", `{"id":`+strconv.FormatUint(uint64(roomID), 10)+`}`)
	if code := roomResponseCode(t, w); code != 1013 {
		t.Fatalf("expected 1013 without domain room:read, got %d: %s", code, w.Body.String())
	}
}

func TestRoomHandler_List_DomainReadPermissionRequired(t *testing.T) {
	middleware.SetDomainChecker(func(_, _ string) bool { return true })
	t.Cleanup(func() { middleware.SetDomainChecker(nil) })

	db, domain, domainSvc, _ := newRoomDomainManageEnv(t, "viewer-1", "viewer", "someone-else", true, nil)
	h := NewRoomHandler(service.NewRoomService(repository.NewRoomRepository(db)), nil, domainSvc)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("username", "viewer-1")
		c.Set("user_uuid", "viewer-1")
		c.Set("role", "user")
		c.Set("domain_uuid", domain.UUID)
		c.Next()
	})
	r.POST("/room/list", h.List)

	w := postRoomJSON(t, r, "/room/list", `{}`)
	if code := roomResponseCode(t, w); code != 1013 {
		t.Fatalf("expected 1013 without domain room:read, got %d: %s", code, w.Body.String())
	}
}
