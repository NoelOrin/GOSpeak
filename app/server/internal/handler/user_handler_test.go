package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestUserHandler_PresetAvatars_GETAndPOST(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := &UserHandler{}
	r.GET("/user/preset-avatars", h.PresetAvatars)
	r.POST("/user/preset-avatars", h.PresetAvatars)

	for _, method := range []string{http.MethodGet, http.MethodPost} {
		req := httptest.NewRequest(method, "/user/preset-avatars", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("%s: expected status 200, got %d", method, w.Code)
		}

		var resp struct {
			Code int `json:"code"`
			Data struct {
				Avatars []string `json:"avatars"`
			} `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("%s: decode response: %v", method, err)
		}
		if resp.Code != 0 {
			t.Fatalf("%s: expected code 0, got %d", method, resp.Code)
		}
		if len(resp.Data.Avatars) == 0 {
			t.Fatalf("%s: expected preset avatars, got empty list", method)
		}
		for _, url := range resp.Data.Avatars {
			if url == "" {
				t.Fatalf("%s: preset avatar URL must not be empty", method)
			}
		}
	}
}
