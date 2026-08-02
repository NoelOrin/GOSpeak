package user

import (
	"net/http"
	"testing"

	"GOSpeak/internal/handler"

	"github.com/gin-gonic/gin"
)

func TestRegisterIncludesPresetAvatars(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	Register(r.Group("/user"), &handler.UserHandler{})

	methods := map[string]bool{}
	for _, route := range r.Routes() {
		if route.Path == "/user/preset-avatars" {
			methods[route.Method] = true
		}
	}
	if !methods[http.MethodGet] {
		t.Fatal("expected GET /user/preset-avatars route")
	}
	if !methods[http.MethodPost] {
		t.Fatal("expected POST /user/preset-avatars route")
	}
}
