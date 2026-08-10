package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"GOSpeak/internal/model"
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
		[]string{model.PermMessageDeleteOthers},
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
