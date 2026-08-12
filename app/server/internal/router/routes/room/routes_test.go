package room

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
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

func TestRegisterProtectedRoomRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	RegisterProtected(r.Group("/room"), &handler.RoomHandler{})

	routes := r.Routes()
	counts := map[string]int{}
	for _, route := range routes {
		if route.Method != "POST" {
			t.Fatalf("unexpected method for %s: %s", route.Path, route.Method)
		}
		counts[route.Path]++
	}

	for _, path := range []string{"/room/create", "/room/list", "/room/get", "/room/update", "/room/delete"} {
		if counts[path] == 0 {
			t.Fatalf("missing route %s", path)
		}
	}
	if counts["/room/create"] != 1 {
		t.Fatalf("expected exactly one create route, got %d", counts["/room/create"])
	}
	if counts["/room/list"] != 1 {
		t.Fatalf("expected exactly one list route, got %d", counts["/room/list"])
	}
}

func TestRoomHandler_CRUD_AllowedThroughProtectedRoutesWithoutGlobalPermission(t *testing.T) {
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
		[]string{model.PermRoomCreate, model.PermRoomRead, model.PermRoomUpdate, model.PermRoomDelete},
	); err != nil {
		t.Fatalf("create role: %v", err)
	}
	if err := db.Create(&model.DomainMember{DomainUUID: domain.UUID, UserUUID: "mod-1", RoleName: "moderator"}).Error; err != nil {
		t.Fatalf("seed member: %v", err)
	}
	room := model.Room{Name: "lobby", DomainUUID: domain.UUID, CreatedBy: "someone-else"}
	if err := db.Create(&room).Error; err != nil {
		t.Fatalf("seed room: %v", err)
	}

	domainSvc := service.NewDomainService(repository.NewDomainRepository(db), repository.NewDomainRoleRepository(db))
	h := handler.NewRoomHandler(service.NewRoomService(repository.NewRoomRepository(db)), nil, domainSvc)

	r := gin.New()
	protected := r.Group("/room")
	protected.Use(func(c *gin.Context) {
		c.Set("username", "mod-1")
		c.Set("user_uuid", "mod-1")
		c.Set("role", "user")
		c.Set("domain_uuid", domain.UUID)
		c.Next()
	})
	RegisterProtected(protected, h)

	for _, tt := range []struct {
		name string
		path string
		body string
	}{
		{name: "create", path: "/room/create", body: `{"name":"created-room","domain_uuid":"` + domain.UUID + `"}`},
		{name: "get", path: "/room/get", body: `{"id":` + strconv.FormatUint(uint64(room.ID), 10) + `}`},
		{name: "list", path: "/room/list", body: `{}`},
		{name: "update", path: "/room/update", body: `{"id":` + strconv.FormatUint(uint64(room.ID), 10) + `,"name":"renamed"}`},
		{name: "delete", path: "/room/delete", body: `{"id":` + strconv.FormatUint(uint64(room.ID), 10) + `}`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, tt.path, strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			var resp map[string]interface{}
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			code, _ := resp["code"].(float64)
			if int(code) != 0 {
				t.Fatalf("expected code 0 for %s through protected routes, got %d: %s", tt.name, int(code), w.Body.String())
			}
		})
	}
}
