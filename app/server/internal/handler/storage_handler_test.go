package handler

import (
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"GOSpeak/internal/config"
	"GOSpeak/internal/model"
	"GOSpeak/internal/repository"
	"GOSpeak/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestObjectKeyUserPrefix(t *testing.T) {
	if got := objectKeyUserPrefix("uploads/", "user-1"); got != "uploads/user-1/" {
		t.Fatalf("unexpected prefix: %s", got)
	}
	if got := objectKeyUserPrefix("files", "user-2"); got != "files/user-2/" {
		t.Fatalf("unexpected normalized prefix: %s", got)
	}
}

func TestGenerateObjectKeyIncludesUser(t *testing.T) {
	key := generateObjectKey("uploads/", "avatar", "a.png", "user-1")
	if !strings.HasPrefix(key, "uploads/user-1/avatar/") {
		t.Fatalf("object key must be owned by user, got %s", key)
	}
}

func TestUpload_RejectsForeignObjectKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.StorageConfig{}); err != nil {
		t.Fatalf("migrate storage config: %v", err)
	}
	repo := repository.NewStorageConfigRepository(db)
	svc := service.NewStorageService(repo, &config.Config{StorageType: "local", StoragePathPrefix: "uploads/"})
	h := NewStorageHandler(svc)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("username", "user-1")
		c.Set("user_uuid", "user-1")
		c.Next()
	})
	r.POST("/upload", h.Upload)

	var body strings.Builder
	w := multipart.NewWriter(&body)
	_ = w.WriteField("object_key", "uploads/avatar/other-user/123.png")
	fw, _ := w.CreateFormFile("file", "a.png")
	_, _ = fw.Write([]byte("hello"))
	_ = w.Close()

	req := httptest.NewRequest(http.MethodPost, "/upload", strings.NewReader(body.String()))
	req.Header.Set("Content-Type", w.FormDataContentType())
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if code := intCode(resp["code"]); code != 1013 {
		t.Fatalf("expected 1013 for foreign object key, got %d: %s", code, resp["msg"])
	}
}

func TestUpload_AcceptsOwnedObjectKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.StorageConfig{}); err != nil {
		t.Fatalf("migrate storage config: %v", err)
	}
	repo := repository.NewStorageConfigRepository(db)
	svc := service.NewStorageService(repo, &config.Config{StorageType: "local", StoragePathPrefix: "uploads/"})
	h := NewStorageHandler(svc)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("username", "user-1")
		c.Set("user_uuid", "user-1")
		c.Next()
	})
	r.POST("/upload", h.Upload)

	var body strings.Builder
	w := multipart.NewWriter(&body)
	_ = w.WriteField("object_key", "uploads/user-1/avatar/123.txt")
	fw, _ := w.CreateFormFile("file", "a.txt")
	_, _ = fw.Write([]byte("hello"))
	_ = w.Close()

	req := httptest.NewRequest(http.MethodPost, "/upload", strings.NewReader(body.String()))
	req.Header.Set("Content-Type", w.FormDataContentType())
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if code := intCode(resp["code"]); code != 0 {
		t.Fatalf("expected success for owned object key, got %d: %s", code, resp["msg"])
	}
}
