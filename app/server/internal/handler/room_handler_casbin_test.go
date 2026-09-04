package handler

import (
	"encoding/json"
	"strconv"
	"testing"

	"GOSpeak/internal/middleware"
	"GOSpeak/internal/model"
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/repository"
	"GOSpeak/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

type roomCasbinDTO struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
}

func newRoomCasbinEnv(t *testing.T) (*gorm.DB, *model.Domain, *service.DomainService, uint) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&model.User{},
		&model.Room{},
		&model.Domain{},
		&model.DomainMember{},
		&model.DomainRole{},
		&model.DomainRolePermission{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	domain := &model.Domain{Name: "Foreign", OwnerUUID: "owner-human"}
	if err := db.Create(domain).Error; err != nil {
		t.Fatalf("create domain: %v", err)
	}
	if err := repository.SeedDefaultDomainRoles(db, domain.UUID); err != nil {
		t.Fatalf("seed roles: %v", err)
	}
	admin := &model.User{
		UUID:   "platform-admin",
		Name:   "platform-admin",
		Role:   "admin",
		Status: model.UserStatusActive,
	}
	if err := db.Create(admin).Error; err != nil {
		t.Fatalf("create platform admin: %v", err)
	}
	owner := &model.DomainMember{
		DomainUUID: domain.UUID,
		UserUUID:   "owner-human",
		RoleName:   model.DomainRoleOwner,
	}
	if err := db.Create(owner).Error; err != nil {
		t.Fatalf("create owner member: %v", err)
	}
	room := model.Room{Name: "foreign-room", DomainUUID: domain.UUID, CreatedBy: "someone-else"}
	if err := db.Create(&room).Error; err != nil {
		t.Fatalf("create room: %v", err)
	}

	domainSvc := service.NewDomainService(repository.NewDomainRepository(db), repository.NewDomainRoleRepository(db))
	if err := domainSvc.UseCasbin(repository.NewDomainCasbinAdapter(db)); err != nil {
		t.Fatalf("use casbin: %v", err)
	}
	return db, domain, domainSvc, room.ID
}

func TestRoomRoutes_PlatformAdminManagesForeignDomain(t *testing.T) {
	middleware.SetDomainChecker(func(_, _ string) bool { return false })
	t.Cleanup(func() { middleware.SetDomainChecker(nil) })

	db, domain, domainSvc, roomID := newRoomCasbinEnv(t)
	h := NewRoomHandler(service.NewRoomService(repository.NewRoomRepository(db)), nil, domainSvc)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("username", "platform-admin")
		c.Set("user_uuid", "platform-admin")
		c.Set("role", "admin")
		c.Set("claims", &pkg.Claims{
			UserUUID:  "platform-admin",
			Username:  "platform-admin",
			Role:      "admin",
			TokenType: pkg.AccessTokenType,
		})
		c.Next()
	})
	r.POST("/room/create", middleware.RequirePlatformAdminOrDomainMember(), h.Create)
	r.POST("/room/list", middleware.RequirePlatformAdminOrDomainMemberIfProvided(), h.List)
	r.POST("/room/get", h.Get)
	r.POST("/room/update", h.Update)
	r.POST("/room/delete", h.Delete)

	w := postRoomJSON(t, r, "/room/create", `{"name":"admin-created","domain_uuid":"`+domain.UUID+`","type":"voice"}`)
	if code := roomResponseCode(t, w); code != 0 {
		t.Fatalf("platform admin create: expected 0, got %d: %s", code, w.Body.String())
	}

	w = postRoomJSON(t, r, "/room/list", `{"domain_uuid":"`+domain.UUID+`"}`)
	var envelope struct {
		Code int            `json:"code"`
		Data roomCasbinList `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if envelope.Code != 0 || len(envelope.Data.Rooms) != 2 {
		t.Fatalf("platform admin list: expected code 0 and 2 rooms, got code=%d rooms=%d: %s", envelope.Code, len(envelope.Data.Rooms), w.Body.String())
	}

	w = postRoomJSON(t, r, "/room/update", `{"id":`+strconv.FormatUint(uint64(roomID), 10)+`,"name":"renamed-by-admin"}`)
	if code := roomResponseCode(t, w); code != 0 {
		t.Fatalf("platform admin update: expected 0, got %d: %s", code, w.Body.String())
	}
	w = postRoomJSON(t, r, "/room/get", `{"id":`+strconv.FormatUint(uint64(roomID), 10)+`}`)
	if code := roomResponseCode(t, w); code != 0 {
		t.Fatalf("platform admin get: expected 0, got %d", code)
	}
	w = postRoomJSON(t, r, "/room/delete", `{"id":`+strconv.FormatUint(uint64(roomID), 10)+`}`)
	if code := roomResponseCode(t, w); code != 0 {
		t.Fatalf("platform admin delete: expected 0, got %d", code)
	}
}

type roomCasbinList struct {
	Rooms []roomCasbinDTO `json:"rooms"`
}

func TestRoomRoutes_DomainOwnerManagesForeignCreatedRoom(t *testing.T) {
	db, _, domainSvc, roomID := newRoomCasbinEnv(t)
	middleware.SetDomainChecker(domainSvc.IsMember)
	t.Cleanup(func() { middleware.SetDomainChecker(nil) })

	h := NewRoomHandler(service.NewRoomService(repository.NewRoomRepository(db)), nil, domainSvc)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("username", "owner-human")
		c.Set("user_uuid", "owner-human")
		c.Set("role", "user")
		c.Set("claims", &pkg.Claims{UserUUID: "owner-human", Role: "user", TokenType: pkg.AccessTokenType})
		c.Next()
	})
	r.POST("/room/create", middleware.RequirePlatformAdminOrDomainMember(), h.Create)
	r.POST("/room/list", middleware.RequirePlatformAdminOrDomainMemberIfProvided(), h.List)
	r.POST("/room/get", h.Get)
	r.POST("/room/update", h.Update)
	r.POST("/room/delete", h.Delete)

	w := postRoomJSON(t, r, "/room/update", `{"id":`+strconv.FormatUint(uint64(roomID), 10)+`,"name":"renamed-by-owner"}`)
	if code := roomResponseCode(t, w); code != 0 {
		t.Fatalf("owner update: expected 0, got %d: %s", code, w.Body.String())
	}
	w = postRoomJSON(t, r, "/room/delete", `{"id":`+strconv.FormatUint(uint64(roomID), 10)+`}`)
	if code := roomResponseCode(t, w); code != 0 {
		t.Fatalf("owner delete: expected 0, got %d", code)
	}
}

func TestRoomRoutes_BotAdminTokenDoesNotInheritPlatformAdmin(t *testing.T) {
	db, _, domainSvc, roomID := newRoomCasbinEnv(t)
	middleware.SetDomainChecker(func(_, _ string) bool { return true })
	t.Cleanup(func() { middleware.SetDomainChecker(nil) })

	h := NewRoomHandler(service.NewRoomService(repository.NewRoomRepository(db)), nil, domainSvc)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("username", "bot-admin")
		c.Set("user_uuid", "bot-admin")
		c.Set("role", "admin")
		c.Set("claims", &pkg.Claims{UserUUID: "bot-admin", Role: "admin", TokenType: pkg.BotTokenType})
		c.Next()
	})
	r.POST("/room/create", middleware.RequirePlatformAdminOrDomainMember(), h.Create)
	r.POST("/room/list", middleware.RequirePlatformAdminOrDomainMemberIfProvided(), h.List)
	r.POST("/room/get", h.Get)
	r.POST("/room/update", h.Update)
	r.POST("/room/delete", h.Delete)

	w := postRoomJSON(t, r, "/room/update", `{"id":`+strconv.FormatUint(uint64(roomID), 10)+`,"name":"renamed-by-bot"}`)
	if code := roomResponseCode(t, w); code != 1013 {
		t.Fatalf("bot update: expected 1013, got %d: %s", code, w.Body.String())
	}
}
