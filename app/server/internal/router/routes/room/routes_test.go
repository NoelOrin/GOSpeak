package room

import (
	"testing"

	"GOSpeak/internal/handler"

	"github.com/gin-gonic/gin"
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
