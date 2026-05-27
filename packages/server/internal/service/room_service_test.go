package service

import (
	"testing"

	"go_rtc/internal/model"
	"go_rtc/internal/pkg"
	"gorm.io/gorm"
)

// ─── Mock Room Repository ───

type mockRoomRepo struct {
	createFn   func(*model.Room) error
	getByIDFn  func(uint) (*model.Room, error)
	getByUUIDFn func(string) (*model.Room, error)
	getByNameFn func(string) (*model.Room, error)
	listFn     func(int, int) ([]model.Room, int64, error)
	updateFn   func(*model.Room) error
	deleteFn   func(uint) error
}

func (m *mockRoomRepo) Create(room *model.Room) error {
	if m.createFn != nil {
		return m.createFn(room)
	}
	return nil
}
func (m *mockRoomRepo) GetByID(id uint) (*model.Room, error) {
	if m.getByIDFn != nil {
		return m.getByIDFn(id)
	}
	return nil, gorm.ErrRecordNotFound
}
func (m *mockRoomRepo) GetByUUID(uuid string) (*model.Room, error) {
	if m.getByUUIDFn != nil {
		return m.getByUUIDFn(uuid)
	}
	return nil, gorm.ErrRecordNotFound
}
func (m *mockRoomRepo) GetByName(name string) (*model.Room, error) {
	if m.getByNameFn != nil {
		return m.getByNameFn(name)
	}
	return nil, gorm.ErrRecordNotFound
}
func (m *mockRoomRepo) List(page, pageSize int) ([]model.Room, int64, error) {
	if m.listFn != nil {
		return m.listFn(page, pageSize)
	}
	return []model.Room{}, 0, nil
}
func (m *mockRoomRepo) Update(room *model.Room) error {
	if m.updateFn != nil {
		return m.updateFn(room)
	}
	return nil
}
func (m *mockRoomRepo) Delete(id uint) error {
	if m.deleteFn != nil {
		return m.deleteFn(id)
	}
	return nil
}

// ─── Create Tests ───

func TestRoomService_Create_Success(t *testing.T) {
	repo := &mockRoomRepo{
		createFn: func(room *model.Room) error {
			room.ID = 1
			return nil
		},
	}
	svc := NewRoomService(repo)

	room := &model.Room{Name: "test-room", Limit: 10}
	err := svc.Create(room)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if room.ID != 1 {
		t.Errorf("expected ID to be set to 1, got %d", room.ID)
	}
}

func TestRoomService_Create_Error(t *testing.T) {
	repo := &mockRoomRepo{
		createFn: func(room *model.Room) error {
			return gorm.ErrInvalidDB
		},
	}
	svc := NewRoomService(repo)

	room := &model.Room{Name: "test-room"}
	err := svc.Create(room)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	appErr, ok := err.(*pkg.AppError)
	if !ok {
		t.Fatalf("expected AppError, got %T", err)
	}
	if appErr.Code != pkg.INTERNAL_ERROR {
		t.Errorf("expected INTERNAL_ERROR, got %d", appErr.Code)
	}
}

// ─── GetByID Tests ───

func TestRoomService_GetByID_Success(t *testing.T) {
	repo := &mockRoomRepo{
		getByIDFn: func(id uint) (*model.Room, error) {
			return &model.Room{ID: id, Name: "room-1"}, nil
		},
	}
	svc := NewRoomService(repo)

	room, err := svc.GetByID(1)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if room.ID != 1 {
		t.Errorf("expected ID 1, got %d", room.ID)
	}
	if room.Name != "room-1" {
		t.Errorf("expected name 'room-1', got %s", room.Name)
	}
}

func TestRoomService_GetByID_NotFound(t *testing.T) {
	repo := &mockRoomRepo{
		getByIDFn: func(id uint) (*model.Room, error) {
			return nil, gorm.ErrRecordNotFound
		},
	}
	svc := NewRoomService(repo)

	room, err := svc.GetByID(999)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	appErr, ok := err.(*pkg.AppError)
	if !ok {
		t.Fatalf("expected AppError, got %T", err)
	}
	if appErr.Code != pkg.NOT_FOUND {
		t.Errorf("expected NOT_FOUND, got %d", appErr.Code)
	}
	if room != nil {
		t.Errorf("expected nil room, got %v", room)
	}
}

// ─── GetByUUID Tests ───

func TestRoomService_GetByUUID_Success(t *testing.T) {
	uuid := "550e8400-e29b-41d4-a716-446655440000"
	repo := &mockRoomRepo{
		getByUUIDFn: func(u string) (*model.Room, error) {
			return &model.Room{UUID: u, Name: "room-1"}, nil
		},
	}
	svc := NewRoomService(repo)

	room, err := svc.GetByUUID(uuid)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if room.UUID != uuid {
		t.Errorf("expected UUID %s, got %s", uuid, room.UUID)
	}
}

func TestRoomService_GetByUUID_NotFound(t *testing.T) {
	repo := &mockRoomRepo{
		getByUUIDFn: func(u string) (*model.Room, error) {
			return nil, gorm.ErrRecordNotFound
		},
	}
	svc := NewRoomService(repo)

	room, err := svc.GetByUUID("nonexistent")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	appErr, ok := err.(*pkg.AppError)
	if !ok {
		t.Fatalf("expected AppError, got %T", err)
	}
	if appErr.Code != pkg.NOT_FOUND {
		t.Errorf("expected NOT_FOUND, got %d", appErr.Code)
	}
	if room != nil {
		t.Errorf("expected nil room, got %v", room)
	}
}

// ─── List Tests ───

func TestRoomService_List_Success(t *testing.T) {
	repo := &mockRoomRepo{
		listFn: func(page, pageSize int) ([]model.Room, int64, error) {
			rooms := []model.Room{
				{ID: 1, Name: "room-1"},
				{ID: 2, Name: "room-2"},
			}
			return rooms, 2, nil
		},
	}
	svc := NewRoomService(repo)

	rooms, total, err := svc.List(1, 10)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(rooms) != 2 {
		t.Errorf("expected 2 rooms, got %d", len(rooms))
	}
	if total != 2 {
		t.Errorf("expected total 2, got %d", total)
	}
}

func TestRoomService_List_DefaultPagination(t *testing.T) {
	capturedPage, capturedPageSize := 0, 0
	repo := &mockRoomRepo{
		listFn: func(page, pageSize int) ([]model.Room, int64, error) {
			capturedPage = page
			capturedPageSize = pageSize
			return []model.Room{}, 0, nil
		},
	}
	svc := NewRoomService(repo)

	svc.List(-1, -1)
	if capturedPage != 1 {
		t.Errorf("expected page 1 (default), got %d", capturedPage)
	}
	if capturedPageSize != 20 {
		t.Errorf("expected pageSize 20 (default), got %d", capturedPageSize)
	}
}

func TestRoomService_List_MaxPageSize(t *testing.T) {
	capturedPageSize := 0
	repo := &mockRoomRepo{
		listFn: func(page, pageSize int) ([]model.Room, int64, error) {
			capturedPageSize = pageSize
			return []model.Room{}, 0, nil
		},
	}
	svc := NewRoomService(repo)

	svc.List(1, 200)
	if capturedPageSize != 20 {
		t.Errorf("expected pageSize capped at 20, got %d", capturedPageSize)
	}
}

func TestRoomService_List_Error(t *testing.T) {
	repo := &mockRoomRepo{
		listFn: func(page, pageSize int) ([]model.Room, int64, error) {
			return nil, 0, gorm.ErrInvalidDB
		},
	}
	svc := NewRoomService(repo)

	rooms, total, err := svc.List(1, 10)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if rooms != nil {
		t.Errorf("expected nil rooms, got %v", rooms)
	}
	if total != 0 {
		t.Errorf("expected total 0, got %d", total)
	}
}

// ─── Update Tests ───

func TestRoomService_Update_Success(t *testing.T) {
	repo := &mockRoomRepo{
		updateFn: func(room *model.Room) error {
			return nil
		},
	}
	svc := NewRoomService(repo)

	room := &model.Room{ID: 1, Name: "updated-room"}
	err := svc.Update(room)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestRoomService_Update_Error(t *testing.T) {
	repo := &mockRoomRepo{
		updateFn: func(room *model.Room) error {
			return gorm.ErrInvalidDB
		},
	}
	svc := NewRoomService(repo)

	room := &model.Room{ID: 1}
	err := svc.Update(room)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	appErr, ok := err.(*pkg.AppError)
	if !ok {
		t.Fatalf("expected AppError, got %T", err)
	}
	if appErr.Code != pkg.INTERNAL_ERROR {
		t.Errorf("expected INTERNAL_ERROR, got %d", appErr.Code)
	}
}

// ─── Delete Tests ───

func TestRoomService_Delete_Success(t *testing.T) {
	repo := &mockRoomRepo{
		getByIDFn: func(id uint) (*model.Room, error) {
			return &model.Room{ID: id}, nil
		},
		deleteFn: func(id uint) error {
			return nil
		},
	}
	svc := NewRoomService(repo)

	err := svc.Delete(1)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestRoomService_Delete_NotFound(t *testing.T) {
	repo := &mockRoomRepo{
		getByIDFn: func(id uint) (*model.Room, error) {
			return nil, gorm.ErrRecordNotFound
		},
	}
	svc := NewRoomService(repo)

	err := svc.Delete(999)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	appErr, ok := err.(*pkg.AppError)
	if !ok {
		t.Fatalf("expected AppError, got %T", err)
	}
	if appErr.Code != pkg.NOT_FOUND {
		t.Errorf("expected NOT_FOUND, got %d", appErr.Code)
	}
}

func TestRoomService_Delete_Error(t *testing.T) {
	repo := &mockRoomRepo{
		getByIDFn: func(id uint) (*model.Room, error) {
			return &model.Room{ID: id}, nil
		},
		deleteFn: func(id uint) error {
			return gorm.ErrInvalidDB
		},
	}
	svc := NewRoomService(repo)

	err := svc.Delete(1)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	appErr, ok := err.(*pkg.AppError)
	if !ok {
		t.Fatalf("expected AppError, got %T", err)
	}
	if appErr.Code != pkg.INTERNAL_ERROR {
		t.Errorf("expected INTERNAL_ERROR, got %d", appErr.Code)
	}
}
