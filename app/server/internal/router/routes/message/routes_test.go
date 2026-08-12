package message

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"GOSpeak/internal/handler"
	"GOSpeak/internal/middleware"
	"GOSpeak/internal/model"
	"GOSpeak/internal/repository"
	"GOSpeak/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestRegisterProtectedMessageRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	RegisterProtected(r.Group("/message"), &handler.MessageHandler{})

	routes := r.Routes()
	counts := map[string]int{}
	for _, route := range routes {
		if route.Method != "POST" {
			t.Fatalf("unexpected method for %s: %s", route.Path, route.Method)
		}
		counts[route.Path]++
	}

	for _, path := range []string{
		"/message/messages/list",
		"/message/messages/search",
		"/message/messages/send",
		"/message/messages/edit",
		"/message/messages/delete",
		"/message/messages/react",
		"/message/messages/unreact",
	} {
		if counts[path] == 0 {
			t.Fatalf("missing route %s", path)
		}
		if counts[path] != 1 {
			t.Fatalf("expected exactly one route for %s, got %d", path, counts[path])
		}
	}
}

func TestMessageHandler_CRUD_AllowedThroughProtectedRoutesWithoutGlobalPermission(t *testing.T) {
	gin.SetMode(gin.TestMode)
	middleware.SetDomainChecker(func(_, _ string) bool { return true })
	t.Cleanup(func() { middleware.SetDomainChecker(nil) })

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
		&model.MessageMention{},
		&model.MessageReaction{},
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
	if err := repository.NewDomainRoleRepository(db).CreateRoleWithPermissions(
		&model.DomainRole{DomainUUID: domain.UUID, Name: "moderator"},
		[]string{model.PermMessageRead, model.PermMessageSend, model.PermMessageDeleteOthers},
	); err != nil {
		t.Fatalf("create role: %v", err)
	}
	if err := db.Create(&model.DomainMember{DomainUUID: domain.UUID, UserUUID: "mod-1", RoleName: "moderator"}).Error; err != nil {
		t.Fatalf("seed member: %v", err)
	}
	room := &model.Room{Name: "chat", DomainUUID: domain.UUID, Type: model.RoomTypeText}
	if err := db.Create(room).Error; err != nil {
		t.Fatalf("seed room: %v", err)
	}
	otherMsg := &model.Message{
		RoomUUID: room.UUID, AuthorID: "other", AuthorUUID: "other-uuid",
		Content: "other hello", ConversationType: model.ConversationTypeRoom,
	}
	if err := db.Create(otherMsg).Error; err != nil {
		t.Fatalf("seed other message: %v", err)
	}

	domainSvc := service.NewDomainService(repository.NewDomainRepository(db), repository.NewDomainRoleRepository(db))
	roomSvc := service.NewRoomService(repository.NewRoomRepository(db))
	msgSvc := service.NewMessageService(repository.NewMessageRepository(db), repository.NewRoomRepository(db), domainSvc)
	h := handler.NewMessageHandler(msgSvc, nil, roomSvc, domainSvc)

	r := gin.New()
	protected := r.Group("/message")
	protected.Use(func(c *gin.Context) {
		c.Set("username", "mod-1")
		c.Set("user_uuid", "mod-1")
		c.Set("role", "user")
		c.Set("domain_uuid", domain.UUID)
		c.Next()
	})
	RegisterProtected(protected, h)

	sendResp := postMessageJSON(t, r, "/message/messages/send", map[string]string{"room_uuid": room.UUID, "content": "hello"})
	if code := messageResponseCode(sendResp); code != 0 {
		t.Fatalf("send: expected code 0, got %d: %s", code, sendResp["msg"])
	}
	data, _ := sendResp["data"].(map[string]interface{})
	msgUUID, _ := data["uuid"].(string)
	if msgUUID == "" {
		t.Fatalf("send: response missing message uuid: %s", sendResp["msg"])
	}

	for _, tt := range []struct {
		name    string
		path    string
		payload map[string]string
	}{
		{name: "list", path: "/message/messages/list", payload: map[string]string{"room_uuid": room.UUID}},
		{name: "search", path: "/message/messages/search", payload: map[string]string{"room_uuid": room.UUID, "query": "hello"}},
		{name: "edit", path: "/message/messages/edit", payload: map[string]string{"room_uuid": room.UUID, "message_uuid": msgUUID, "content": "updated"}},
		{name: "react", path: "/message/messages/react", payload: map[string]string{"room_uuid": room.UUID, "message_uuid": msgUUID, "emoji": "👍"}},
		{name: "unreact", path: "/message/messages/unreact", payload: map[string]string{"room_uuid": room.UUID, "message_uuid": msgUUID, "emoji": "👍"}},
		{name: "delete-other", path: "/message/messages/delete", payload: map[string]string{"room_uuid": room.UUID, "message_uuid": otherMsg.UUID}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			resp := postMessageJSON(t, r, tt.path, tt.payload)
			if code := messageResponseCode(resp); code != 0 {
				t.Fatalf("expected code 0 for %s through protected routes, got %d: %s", tt.name, code, resp["msg"])
			}
		})
	}
}

func postMessageJSON(t *testing.T, r *gin.Engine, path string, payload map[string]string) map[string]interface{} {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return resp
}

func messageResponseCode(resp map[string]interface{}) int {
	code, _ := resp["code"].(float64)
	return int(code)
}
