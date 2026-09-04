package handler

import (
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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

func TestConfirmUpload_Ownership(t *testing.T) {
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
		c.Set("username", "user-b")
		c.Set("user_uuid", "user-b")
		c.Next()
	})
	r.POST("/confirm", h.ConfirmUpload)

	body := `{"object_key":"uploads/user-a/avatar/x.png"}`
	req := httptest.NewRequest(http.MethodPost, "/confirm", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
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

func TestUpload_RejectsHugeBody(t *testing.T) {
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

	objectKey := "uploads/user-1/avatar/__huge__.txt"
	var body strings.Builder
	body.Grow(6 * 1024 * 1024)
	w := multipart.NewWriter(&body)
	_ = w.WriteField("object_key", objectKey)
	fw, _ := w.CreateFormFile("file", "huge.txt")
	chunk := make([]byte, 64*1024)
	for i := range chunk {
		chunk[i] = 'x'
	}
	for written := 0; written < 6*1024*1024; written += len(chunk) {
		_, _ = fw.Write(chunk)
	}
	_ = w.Close()

	req := httptest.NewRequest(http.MethodPost, "/upload", strings.NewReader(body.String()))
	req.Header.Set("Content-Type", w.FormDataContentType())
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if code := intCode(resp["code"]); code != 8103 {
		t.Fatalf("expected 8103 for oversized upload, got %d: %s", code, resp["msg"])
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for oversized upload, got %d", rec.Code)
	}

	// 超大文件必须在上传前被拒绝，不能落盘。
	diskPath := filepath.Join("uploads", "uploads", "user-1", "avatar", "__huge__.txt")
	t.Cleanup(func() {
		_ = os.Remove(diskPath)
	})
	if _, err := os.Stat(diskPath); err == nil {
		t.Fatalf("oversized upload was written to disk: %s", diskPath)
	}
}

func TestPresignUpload_RejectsInvalidCategory(t *testing.T) {
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
	r.POST("/presign", h.PresignUpload)

	body := `{"file_name":"a.png","content_type":"image/png","file_size":1024,"category":"avatar/../evil"}`
	req := httptest.NewRequest(http.MethodPost, "/presign", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if code := intCode(resp["code"]); code != 2001 {
		t.Fatalf("expected 2001 for invalid category, got %d: %s", code, resp["msg"])
	}
}
