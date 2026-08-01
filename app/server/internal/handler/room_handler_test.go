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

func TestRoomHandler_List_FiltersDomainUUID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Room{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	rooms := []model.Room{
		{Name: "lobby", DomainUUID: "domain-a"},
		{Name: "lobby", DomainUUID: "domain-b"},
	}
	if err := db.Create(&rooms).Error; err != nil {
		t.Fatalf("seed rooms: %v", err)
	}

	middleware.SetDomainChecker(func(domainUUID, userUUID string) bool { return true })
	t.Cleanup(func() { middleware.SetDomainChecker(nil) })

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("username", "user-1")
		c.Set("user_uuid", "user-1")
		c.Next()
	})
	h := NewRoomHandler(service.NewRoomService(repository.NewRoomRepository(db)), nil)
	r.POST("/api/v1/room/list", h.List)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/room/list", strings.NewReader(`{"domain_uuid":"domain-a"}`))
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
	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		t.Fatal("expected list data")
	}
	roomsData, ok := data["rooms"].([]interface{})
	if !ok || len(roomsData) != 1 {
		t.Fatalf("expected 1 filtered room, got %#v", data["rooms"])
	}
	roomData, ok := roomsData[0].(map[string]interface{})
	if !ok || roomData["domain_uuid"] != "domain-a" {
		t.Fatalf("unexpected room payload: %#v", roomsData[0])
	}
}

func TestRoomHandler_Create_PersistsDomainUUID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Room{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	middleware.SetDomainChecker(func(domainUUID, userUUID string) bool { return true })
	t.Cleanup(func() { middleware.SetDomainChecker(nil) })

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("username", "user-1")
		c.Set("user_uuid", "user-1")
		c.Next()
	})
	h := NewRoomHandler(service.NewRoomService(repository.NewRoomRepository(db)), nil)
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
	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		t.Fatal("expected room data")
	}
	if data["domain_uuid"] != "domain-a" {
		t.Fatalf("expected domain_uuid domain-a, got %v", data["domain_uuid"])
	}
}

func TestRoomHandler_List_NoDomainUUID_OnlyPlatformRooms(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Room{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	rooms := []model.Room{
		{Name: "lobby", DomainUUID: "domain-a"},
		{Name: "general"},
	}
	if err := db.Create(&rooms).Error; err != nil {
		t.Fatalf("seed rooms: %v", err)
	}

	r := gin.New()
	h := NewRoomHandler(service.NewRoomService(repository.NewRoomRepository(db)), nil)
	r.POST("/api/v1/room/list", h.List)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/room/list", strings.NewReader(`{}`))
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
	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		t.Fatal("expected list data")
	}
	roomsData, ok := data["rooms"].([]interface{})
	if !ok || len(roomsData) != 1 {
		t.Fatalf("expected only platform room, got %#v", data["rooms"])
	}
	roomData, ok := roomsData[0].(map[string]interface{})
	if !ok || roomData["domain_uuid"] != "" {
		t.Fatalf("unexpected room payload: %#v", roomsData[0])
	}
}

func TestRoomHandler_Create_RejectsNonDomainMember(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Room{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	middleware.SetDomainChecker(func(domainUUID, userUUID string) bool { return false })
	t.Cleanup(func() { middleware.SetDomainChecker(nil) })

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("username", "user-1")
		c.Set("user_uuid", "user-1")
		c.Next()
	})
	h := NewRoomHandler(service.NewRoomService(repository.NewRoomRepository(db)), nil)
	r.POST("/api/v1/room/create", h.Create)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/room/create", strings.NewReader(`{"name":"lobby","domain_uuid":"domain-a"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if code := intCode(resp["code"]); code != 1013 {
		t.Fatalf("expected 1013 for non-member create, got %d: %s", code, resp["msg"])
	}
}

func TestRoomHandler_List_RejectsNonDomainMember(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Room{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	middleware.SetDomainChecker(func(domainUUID, userUUID string) bool { return false })
	t.Cleanup(func() { middleware.SetDomainChecker(nil) })

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("username", "user-1")
		c.Set("user_uuid", "user-1")
		c.Next()
	})
	h := NewRoomHandler(service.NewRoomService(repository.NewRoomRepository(db)), nil)
	r.POST("/api/v1/room/list", h.List)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/room/list", strings.NewReader(`{"domain_uuid":"domain-a"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if code := intCode(resp["code"]); code != 1013 {
		t.Fatalf("expected 1013 for non-member list, got %d: %s", code, resp["msg"])
	}
}

func TestRoomHandler_CRUD_RejectsNonDomainMember(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Room{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	room := model.Room{Name: "lobby", DomainUUID: "domain-a", CreatedBy: "other"}
	if err := db.Create(&room).Error; err != nil {
		t.Fatalf("seed room: %v", err)
	}
	middleware.SetDomainChecker(func(domainUUID, userUUID string) bool { return false })
	t.Cleanup(func() { middleware.SetDomainChecker(nil) })

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("username", "other")
		c.Set("user_uuid", "other-uuid")
		c.Set("role", "user")
		c.Next()
	})
	h := NewRoomHandler(service.NewRoomService(repository.NewRoomRepository(db)), nil)
	r.POST("/api/v1/room/get", h.Get)
	r.POST("/api/v1/room/update", h.Update)
	r.POST("/api/v1/room/delete", h.Delete)

	tests := []struct {
		name string
		path string
		body string
	}{
		{"get", "/api/v1/room/get", `{"id":1}`},
		{"update", "/api/v1/room/update", `{"id":1,"name":"new"}`},
		{"delete", "/api/v1/room/delete", `{"id":1}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, tt.path, strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			var resp map[string]interface{}
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if code := intCode(resp["code"]); code != 1013 {
				t.Fatalf("expected 1013 for non-member %s, got %d: %s", tt.name, code, resp["msg"])
			}
		})
	}
}
