package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"GOSpeak/internal/model"
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/repository"
	"GOSpeak/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestMessageHandler_Delete_AllowsDomainRolePermission(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Room{}, &model.Domain{}, &model.DomainMember{}, &model.DomainRole{}, &model.DomainRolePermission{}, &model.Message{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	domain := &model.Domain{Name: "Chat", OwnerUUID: "owner-1"}
	if err := db.Create(domain).Error; err != nil {
		t.Fatalf("seed domain: %v", err)
	}
	if err := repository.SeedDefaultDomainRoles(db, domain.UUID); err != nil {
		t.Fatalf("seed roles: %v", err)
	}
	if err := db.Create(&model.DomainMember{DomainUUID: domain.UUID, UserUUID: "mod-1", RoleName: "moderator"}).Error; err != nil {
		t.Fatalf("seed member: %v", err)
	}
	if err := repository.NewDomainRoleRepository(db).CreateRoleWithPermissions(
		&model.DomainRole{DomainUUID: domain.UUID, Name: "moderator"},
		[]string{model.PermMessageSend, model.PermMessageDeleteOthers},
	); err != nil {
		t.Fatalf("create role: %v", err)
	}
	room := &model.Room{Name: "chat", DomainUUID: domain.UUID, Type: model.RoomTypeText}
	if err := db.Create(room).Error; err != nil {
		t.Fatalf("seed room: %v", err)
	}
	msg := &model.Message{
		RoomUUID: room.UUID, AuthorID: "other", AuthorUUID: "other-uuid",
		Content: "hello", ConversationType: model.ConversationTypeRoom,
	}
	if err := db.Create(msg).Error; err != nil {
		t.Fatalf("seed message: %v", err)
	}

	roomSvc := service.NewRoomService(repository.NewRoomRepository(db))
	domainSvc := service.NewDomainService(repository.NewDomainRepository(db), repository.NewDomainRoleRepository(db))
	h := NewMessageHandler(service.NewMessageService(repository.NewMessageRepository(db), repository.NewRoomRepository(db), domainSvc), nil, roomSvc, domainSvc)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("username", "mod-1")
		c.Set("user_uuid", "mod-1")
		c.Set("role", "user")
		c.Next()
	})
	r.POST("/delete", h.Delete)

	body, _ := json.Marshal(map[string]string{"room_uuid": room.UUID, "message_uuid": msg.UUID})
	req := httptest.NewRequest(http.MethodPost, "/delete", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body %s", w.Code, w.Body.String())
	}
}

func TestMessageHandler_Send_DomainPermissionRequired(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Room{}, &model.Domain{}, &model.DomainMember{}, &model.DomainRole{}, &model.DomainRolePermission{}, &model.Message{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	domain := &model.Domain{Name: "Chat", OwnerUUID: "owner-1"}
	if err := db.Create(domain).Error; err != nil {
		t.Fatalf("seed domain: %v", err)
	}
	if err := repository.SeedDefaultDomainRoles(db, domain.UUID); err != nil {
		t.Fatalf("seed roles: %v", err)
	}
	if err := db.Create(&model.DomainMember{DomainUUID: domain.UUID, UserUUID: "guest-1", RoleName: model.DomainRoleGuest}).Error; err != nil {
		t.Fatalf("seed guest: %v", err)
	}
	room := &model.Room{Name: "chat", DomainUUID: domain.UUID, Type: model.RoomTypeText}
	if err := db.Create(room).Error; err != nil {
		t.Fatalf("seed room: %v", err)
	}

	roomSvc := service.NewRoomService(repository.NewRoomRepository(db))
	domainSvc := service.NewDomainService(repository.NewDomainRepository(db), repository.NewDomainRoleRepository(db))
	msgSvc := service.NewMessageService(repository.NewMessageRepository(db), repository.NewRoomRepository(db), domainSvc)
	h := NewMessageHandler(msgSvc, nil, roomSvc, domainSvc)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("username", "guest-1")
		c.Set("user_uuid", "guest-1")
		c.Set("role", "user")
		c.Next()
	})
	r.POST("/send", h.Send)

	body, _ := json.Marshal(map[string]string{"room_uuid": room.UUID, "content": "hello"})
	req := httptest.NewRequest(http.MethodPost, "/send", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if code := intCode(resp["code"]); code != 1013 {
		t.Fatalf("guest must not send: expected 1013, got %d: %s", code, resp["msg"])
	}
}

func TestMessageHandler_Delete_DomainPermissionDoesNotFallBackToGlobal(t *testing.T) {
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
		&model.Message{},
		&model.Permission{},
		&model.RolePermission{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	domain := &model.Domain{Name: "Chat", OwnerUUID: "owner-1"}
	if err := db.Create(domain).Error; err != nil {
		t.Fatalf("seed domain: %v", err)
	}
	if err := repository.SeedDefaultDomainRoles(db, domain.UUID); err != nil {
		t.Fatalf("seed roles: %v", err)
	}
	if err := db.Create(&model.DomainMember{DomainUUID: domain.UUID, UserUUID: "member-1", RoleName: model.DomainRoleMember}).Error; err != nil {
		t.Fatalf("seed member: %v", err)
	}
	room := &model.Room{Name: "chat", DomainUUID: domain.UUID, Type: model.RoomTypeText}
	if err := db.Create(room).Error; err != nil {
		t.Fatalf("seed room: %v", err)
	}
	msg := &model.Message{
		RoomUUID: room.UUID, AuthorID: "other", AuthorUUID: "other-uuid",
		Content: "hello", ConversationType: model.ConversationTypeRoom,
	}
	if err := db.Create(msg).Error; err != nil {
		t.Fatalf("seed message: %v", err)
	}

	permRepo := repository.NewPermissionRepository(db)
	for _, perm := range model.DefaultPermissions {
		if err := permRepo.CreateIfNotExists(&perm); err != nil {
			t.Fatalf("seed permission %s: %v", perm.Code, err)
		}
	}
	if err := permRepo.SeedRolePermissionsIfEmpty("user", []string{model.PermMessageDeleteOthers}); err != nil {
		t.Fatalf("seed global role permission: %v", err)
	}
	permSvc := service.NewPermissionService(permRepo)
	if err := permSvc.LoadCache(); err != nil {
		t.Fatalf("load permission cache: %v", err)
	}
	if !permSvc.HasPermission("user", model.PermMessageDeleteOthers) {
		t.Fatal("test precondition: global user role must have message:delete_others")
	}

	roomSvc := service.NewRoomService(repository.NewRoomRepository(db))
	domainSvc := service.NewDomainService(repository.NewDomainRepository(db), repository.NewDomainRoleRepository(db))
	msgSvc := service.NewMessageService(repository.NewMessageRepository(db), repository.NewRoomRepository(db), domainSvc)
	h := NewMessageHandler(msgSvc, permSvc, roomSvc, domainSvc)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("username", "member-1")
		c.Set("user_uuid", "member-1")
		c.Set("role", "user")
		c.Next()
	})
	r.POST("/delete", h.Delete)

	body, _ := json.Marshal(map[string]string{"room_uuid": room.UUID, "message_uuid": msg.UUID})
	req := httptest.NewRequest(http.MethodPost, "/delete", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if code := intCode(resp["code"]); code != 1013 {
		t.Fatalf("domain room must not fall back to global delete_others: expected 1013, got %d: %s", code, resp["msg"])
	}
}

func TestMessageHandler_BotClaimsPermissionsOnPlatformRoom(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Room{}, &model.Message{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	room := &model.Room{Name: "chat", DomainUUID: "", Type: model.RoomTypeText}
	if err := db.Create(room).Error; err != nil {
		t.Fatalf("seed room: %v", err)
	}

	roomSvc := service.NewRoomService(repository.NewRoomRepository(db))
	domainSvc := service.NewDomainService(repository.NewDomainRepository(db), repository.NewDomainRoleRepository(db))
	msgSvc := service.NewMessageService(repository.NewMessageRepository(db), repository.NewRoomRepository(db), domainSvc)
	h := NewMessageHandler(msgSvc, nil, roomSvc, domainSvc)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("username", "bot-1")
		c.Set("user_uuid", "bot-1")
		c.Set("role", "user")
		c.Set("claims", &pkg.Claims{Role: "user", Permissions: []string{"message:read"}})
		c.Next()
	})
	r.POST("/send", h.Send)
	r.POST("/list", h.List)

	sendBody, _ := json.Marshal(map[string]string{"room_uuid": room.UUID, "content": "hello"})
	sendReq := httptest.NewRequest(http.MethodPost, "/send", bytes.NewReader(sendBody))
	sendReq.Header.Set("Content-Type", "application/json")
	sendW := httptest.NewRecorder()
	r.ServeHTTP(sendW, sendReq)

	var sendResp map[string]interface{}
	if err := json.Unmarshal(sendW.Body.Bytes(), &sendResp); err != nil {
		t.Fatalf("decode send response: %v", err)
	}
	if code := intCode(sendResp["code"]); code != 1013 {
		t.Fatalf("bot without message:send must not send: expected 1013, got %d: %s", code, sendResp["msg"])
	}

	listBody, _ := json.Marshal(map[string]string{"room_uuid": room.UUID})
	listReq := httptest.NewRequest(http.MethodPost, "/list", bytes.NewReader(listBody))
	listReq.Header.Set("Content-Type", "application/json")
	listW := httptest.NewRecorder()
	r.ServeHTTP(listW, listReq)

	var listResp map[string]interface{}
	if err := json.Unmarshal(listW.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if code := intCode(listResp["code"]); code != 0 {
		t.Fatalf("bot with message:read must read: expected 0, got %d: %s", code, listResp["msg"])
	}
}

func TestMessageHandler_Delete_BotClaimsMissingDeleteOthersDoesNotFallBackToGlobal(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Room{}, &model.Message{}, &model.Permission{}, &model.RolePermission{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	room := &model.Room{Name: "chat", DomainUUID: "", Type: model.RoomTypeText}
	if err := db.Create(room).Error; err != nil {
		t.Fatalf("seed room: %v", err)
	}
	msg := &model.Message{
		RoomUUID: room.UUID, AuthorID: "other", AuthorUUID: "other-uuid",
		Content: "hello", ConversationType: model.ConversationTypeRoom,
	}
	if err := db.Create(msg).Error; err != nil {
		t.Fatalf("seed message: %v", err)
	}

	permRepo := repository.NewPermissionRepository(db)
	for _, perm := range model.DefaultPermissions {
		if err := permRepo.CreateIfNotExists(&perm); err != nil {
			t.Fatalf("seed permission %s: %v", perm.Code, err)
		}
	}
	if err := permRepo.SeedRolePermissionsIfEmpty("user", []string{model.PermMessageDeleteOthers}); err != nil {
		t.Fatalf("seed global role permission: %v", err)
	}
	permSvc := service.NewPermissionService(permRepo)
	if err := permSvc.LoadCache(); err != nil {
		t.Fatalf("load permission cache: %v", err)
	}
	if !permSvc.HasPermission("user", model.PermMessageDeleteOthers) {
		t.Fatal("test precondition: global user role must have message:delete_others")
	}

	roomSvc := service.NewRoomService(repository.NewRoomRepository(db))
	domainSvc := service.NewDomainService(repository.NewDomainRepository(db), repository.NewDomainRoleRepository(db))
	msgSvc := service.NewMessageService(repository.NewMessageRepository(db), repository.NewRoomRepository(db), domainSvc)
	h := NewMessageHandler(msgSvc, permSvc, roomSvc, domainSvc)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("username", "bot-1")
		c.Set("user_uuid", "bot-1")
		c.Set("role", "user")
		c.Set("claims", &pkg.Claims{Role: "user", Permissions: []string{"message:send"}})
		c.Next()
	})
	r.POST("/delete", h.Delete)

	body, _ := json.Marshal(map[string]string{"room_uuid": room.UUID, "message_uuid": msg.UUID})
	req := httptest.NewRequest(http.MethodPost, "/delete", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if code := intCode(resp["code"]); code != 1013 {
		t.Fatalf("expected 1013 when claims lack message:delete_others despite global role: got %d: %s", code, resp["msg"])
	}
}

func TestMessageHandler_Send_GuestMessagingDisabledGate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Room{}, &model.Domain{}, &model.DomainMember{}, &model.DomainRole{}, &model.DomainRolePermission{}, &model.Message{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	domain := &model.Domain{Name: "Gate", OwnerUUID: "owner-1", AllowGuest: true}
	if err := db.Create(domain).Error; err != nil {
		t.Fatalf("seed domain: %v", err)
	}
	if err := repository.SeedDefaultDomainRoles(db, domain.UUID); err != nil {
		t.Fatalf("seed roles: %v", err)
	}
	room := &model.Room{Name: "gate-room", DomainUUID: domain.UUID, Type: model.RoomTypeText}
	if err := db.Create(room).Error; err != nil {
		t.Fatalf("seed room: %v", err)
	}

	roomSvc := service.NewRoomService(repository.NewRoomRepository(db))
	domainSvc := service.NewDomainService(repository.NewDomainRepository(db), repository.NewDomainRoleRepository(db))
	msgSvc := service.NewMessageService(repository.NewMessageRepository(db), repository.NewRoomRepository(db), domainSvc)
	h := NewMessageHandler(msgSvc, nil, roomSvc, domainSvc)
	// 访客策略：身份是访客，但发消息开关关闭 —— 门禁先于权限判决触发。
	h.SetGuestPolicy(
		func(string) bool { return true },
		func(string) (bool, bool, bool) { return true, true, false },
	)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("username", "guest-gate")
		c.Set("user_uuid", "guest-gate")
		c.Next()
	})
	r.POST("/send", h.Send)

	body, _ := json.Marshal(map[string]string{"room_uuid": room.UUID, "content": "hi"})
	req := httptest.NewRequest(http.MethodPost, "/send", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if code := intCode(resp["code"]); code != 1013 {
		t.Fatalf("expect 1013, got %d: %s", code, w.Body.String())
	}
	if msg, _ := resp["msg"].(string); !strings.Contains(msg, "guest messaging") {
		t.Fatalf("expect guest messaging gate message, got %q", resp["msg"])
	}
}

func TestMessageHandler_Send_GuestMessagingEnabledBypassesDomainPerm(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Room{}, &model.Domain{}, &model.DomainMember{}, &model.DomainRole{}, &model.DomainRolePermission{}, &model.Message{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	domain := &model.Domain{Name: "Gate2", OwnerUUID: "owner-1", AllowGuest: true}
	if err := db.Create(domain).Error; err != nil {
		t.Fatalf("seed domain: %v", err)
	}
	if err := repository.SeedDefaultDomainRoles(db, domain.UUID); err != nil {
		t.Fatalf("seed roles: %v", err)
	}
	room := &model.Room{Name: "gate-room2", DomainUUID: domain.UUID, Type: model.RoomTypeText}
	if err := db.Create(room).Error; err != nil {
		t.Fatalf("seed room: %v", err)
	}
	// 访客需先成为域成员，否则消息链路会因“非域成员”先于访客门禁拦截。
	if err := db.Create(&model.DomainMember{DomainUUID: domain.UUID, UserUUID: "guest-enable", Nickname: "访客", RoleName: model.DomainRoleGuest}).Error; err != nil {
		t.Fatalf("seed guest member: %v", err)
	}
	// 访客默认角色只有 message:read，没有 message:send，但 GuestCanMessage 开关打开时
	// 应以访客能力短路，允许发送（不会因缺 message:send 而被拒）。
	roomSvc := service.NewRoomService(repository.NewRoomRepository(db))
	domainSvc := service.NewDomainService(repository.NewDomainRepository(db), repository.NewDomainRoleRepository(db))
	msgRepo := repository.NewMessageRepository(db)
	msgSvc := service.NewMessageService(msgRepo, repository.NewRoomRepository(db), domainSvc)
	h := NewMessageHandler(msgSvc, nil, roomSvc, domainSvc)
	h.SetGuestPolicy(
		func(string) bool { return true },
		func(string) (bool, bool, bool) { return true, true, true },
	)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("username", "guest-enable")
		c.Set("user_uuid", "guest-enable")
		c.Next()
	})
	r.POST("/send", h.Send)

	body, _ := json.Marshal(map[string]string{"room_uuid": room.UUID, "content": "hi"})
	req := httptest.NewRequest(http.MethodPost, "/send", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if code := intCode(resp["code"]); code != 0 {
		t.Fatalf("expect success (0), got %d: %s", code, w.Body.String())
	}
}
