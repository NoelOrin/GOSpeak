package service

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"GOSpeak/internal/model"
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/repository"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

type fakeAvatarStorage struct {
	cfg         *model.StorageConfig
	contentType string
	objectKey   string
	size        int64
	calls       int
}

func (f *fakeAvatarStorage) GetConfig() (*model.StorageConfig, error) {
	return f.cfg, nil
}

func (f *fakeAvatarStorage) UploadFromReader(key string, reader io.Reader, size int64, contentType string) (string, error) {
	f.calls++
	f.objectKey = key
	f.size = size
	f.contentType = contentType
	return "/uploads/" + key, nil
}

func setupUploadAvatarService(t *testing.T) (*UserService, *fakeAvatarStorage, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}); err != nil {
		t.Fatalf("migrate user: %v", err)
	}
	repo := repository.NewUserRepository(db)
	store := &fakeAvatarStorage{cfg: &model.StorageConfig{MaxFileSize: 5, PathPrefix: "uploads/"}}
	return NewUserService(repo, store), store, db
}

func createAvatarTestUser(t *testing.T, db *gorm.DB, uuid string) {
	t.Helper()
	user := &model.User{UUID: uuid, Name: uuid}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
}

func assertAppErrorCode(t *testing.T, err error, want pkg.ErrCode) {
	t.Helper()
	var appErr *pkg.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("expected *pkg.AppError, got %T: %v", err, err)
	}
	if appErr.Code != want {
		t.Fatalf("expected code %d, got %d: %v", want, appErr.Code, err)
	}
}

func TestUploadAvatar_RejectsNonImageSniffedContent(t *testing.T) {
	svc, store, db := setupUploadAvatarService(t)
	createAvatarTestUser(t, db, "user-1")

	// 客户端声明 image/png，但文件头是纯文本，必须被魔数嗅探拒绝。
	_, _, err := svc.UploadAvatar("user-1", "avatar.png", "image/png", 5, bytes.NewReader([]byte("not an image")))
	assertAppErrorCode(t, err, pkg.INVALID_PARAMS)
	if store.calls != 0 {
		t.Fatalf("storage must not be called for invalid image, got %d calls", store.calls)
	}
}

func TestUploadAvatar_UsesSniffedContentType(t *testing.T) {
	svc, store, db := setupUploadAvatarService(t)
	createAvatarTestUser(t, db, "user-1")

	png := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00, 0x00, 0x0D}
	avatarURL, _, err := svc.UploadAvatar("user-1", "avatar.jpg", "image/jpeg", int64(len(png)), bytes.NewReader(png))
	if err != nil {
		t.Fatalf("upload avatar: %v", err)
	}
	if avatarURL == "" {
		t.Fatal("expected non-empty avatar URL")
	}
	if store.calls != 1 {
		t.Fatalf("expected one storage call, got %d", store.calls)
	}
	if store.contentType != "image/png" {
		t.Fatalf("expected sniffed image/png, got %s", store.contentType)
	}
	if len(store.objectKey) == 0 {
		t.Fatal("expected object key")
	}
}

func TestUploadAvatar_RejectsHugeFile(t *testing.T) {
	svc, store, db := setupUploadAvatarService(t)
	createAvatarTestUser(t, db, "user-1")

	_, _, err := svc.UploadAvatar("user-1", "avatar.png", "image/png", 6*1024*1024, bytes.NewReader(nil))
	assertAppErrorCode(t, err, pkg.STORAGE_FILE_TOO_LARGE)
	if store.calls != 0 {
		t.Fatalf("storage must not be called for huge avatar, got %d calls", store.calls)
	}
}
