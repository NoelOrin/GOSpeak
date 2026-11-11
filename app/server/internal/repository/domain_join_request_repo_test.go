package repository

import (
	"testing"

	"GOSpeak/internal/model"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupJoinRequestRepoDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}, &model.DomainJoinRequest{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func seedJoinApplicant(t *testing.T, db *gorm.DB, name, displayName string) *model.User {
	t.Helper()
	u := &model.User{Name: name, DisplayName: displayName}
	if err := db.Create(u).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	return u
}

func TestDomainJoinRequestRepository_SubmitCreatesPending(t *testing.T) {
	db := setupJoinRequestRepoDB(t)
	repo := NewDomainJoinRequestRepository(db)
	got, err := repo.Submit("domain-1", "user-1", 7)
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if got.Status != model.JoinRequestPending {
		t.Fatalf("status = %q, want pending", got.Status)
	}
	if got.InviteID != 7 {
		t.Fatalf("invite id = %d, want 7", got.InviteID)
	}
	if got.ID == 0 {
		t.Fatal("submit must assign primary key")
	}
}

func TestDomainJoinRequestRepository_SubmitIsIdempotent(t *testing.T) {
	db := setupJoinRequestRepoDB(t)
	repo := NewDomainJoinRequestRepository(db)
	first, err := repo.Submit("domain-1", "user-1", 7)
	if err != nil {
		t.Fatalf("first submit: %v", err)
	}
	second, err := repo.Submit("domain-1", "user-1", 9)
	if err != nil {
		t.Fatalf("second submit: %v", err)
	}
	if second.ID != first.ID {
		t.Fatalf("second submit created a new row: %d != %d", second.ID, first.ID)
	}
	var count int64
	db.Model(&model.DomainJoinRequest{}).Count(&count)
	if count != 1 {
		t.Fatalf("rows = %d, want 1 (unique index must hold)", count)
	}
}

func TestDomainJoinRequestRepository_GetByDomainUser_NotFound(t *testing.T) {
	db := setupJoinRequestRepoDB(t)
	repo := NewDomainJoinRequestRepository(db)
	if _, err := repo.GetByDomainUser("d", "u"); err != gorm.ErrRecordNotFound {
		t.Fatalf("err = %v, want gorm.ErrRecordNotFound", err)
	}
}

func TestDomainJoinRequestRepository_Reopen(t *testing.T) {
	db := setupJoinRequestRepoDB(t)
	repo := NewDomainJoinRequestRepository(db)
	req, err := repo.Submit("domain-1", "user-1", 7)
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if _, err := repo.Review(req.ID, false, "admin-1"); err != nil {
		t.Fatalf("reject: %v", err)
	}
	if err := repo.Reopen(req.ID, 42); err != nil {
		t.Fatalf("reopen: %v", err)
	}
	got, err := repo.GetByID(req.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Status != model.JoinRequestPending {
		t.Fatalf("status = %q, want pending", got.Status)
	}
	if got.InviteID != 42 {
		t.Fatalf("invite id = %d, want 42 (must refresh)", got.InviteID)
	}
	if got.ReviewedBy != "" || got.ReviewedAt != nil {
		t.Fatalf("reopen must clear review fields: %+v", got)
	}
}

func TestDomainJoinRequestRepository_ReviewApprove(t *testing.T) {
	db := setupJoinRequestRepoDB(t)
	repo := NewDomainJoinRequestRepository(db)
	req, err := repo.Submit("domain-1", "user-1", 7)
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	ok, err := repo.Review(req.ID, true, "admin-1")
	if err != nil {
		t.Fatalf("review: %v", err)
	}
	if !ok {
		t.Fatal("first review must report affected")
	}
	got, err := repo.GetByID(req.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Status != model.JoinRequestApproved {
		t.Fatalf("status = %q, want approved", got.Status)
	}
	if got.ReviewedBy != "admin-1" || got.ReviewedAt == nil {
		t.Fatalf("review fields not written: %+v", got)
	}
}

func TestDomainJoinRequestRepository_ReviewTwiceFails(t *testing.T) {
	db := setupJoinRequestRepoDB(t)
	repo := NewDomainJoinRequestRepository(db)
	req, err := repo.Submit("domain-1", "user-1", 7)
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if _, err := repo.Review(req.ID, true, "admin-1"); err != nil {
		t.Fatalf("first review: %v", err)
	}
	ok, err := repo.Review(req.ID, false, "admin-2")
	if err != nil {
		t.Fatalf("second review: %v", err)
	}
	if ok {
		t.Fatal("second review must not affect any row")
	}
	got, _ := repo.GetByID(req.ID)
	if got.Status != model.JoinRequestApproved || got.ReviewedBy != "admin-1" {
		t.Fatalf("second review must not overwrite: %+v", got)
	}
}

func TestDomainJoinRequestRepository_DeleteByDomainUser(t *testing.T) {
	db := setupJoinRequestRepoDB(t)
	repo := NewDomainJoinRequestRepository(db)
	if _, err := repo.Submit("domain-1", "user-1", 7); err != nil {
		t.Fatalf("submit: %v", err)
	}
	if err := repo.DeleteByDomainUser("domain-1", "user-1"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := repo.GetByDomainUser("domain-1", "user-1"); err != gorm.ErrRecordNotFound {
		t.Fatalf("err = %v, want gorm.ErrRecordNotFound (physical delete)", err)
	}
}

func TestDomainJoinRequestRepository_ListByDomain_FiltersAndSearches(t *testing.T) {
	db := setupJoinRequestRepoDB(t)
	repo := NewDomainJoinRequestRepository(db)
	alice := seedJoinApplicant(t, db, "alice", "爱丽丝")
	bob := seedJoinApplicant(t, db, "bob", "鲍勃")
	if _, err := repo.Submit("domain-1", alice.UUID, 1); err != nil {
		t.Fatalf("submit alice: %v", err)
	}
	bobReq, err := repo.Submit("domain-1", bob.UUID, 1)
	if err != nil {
		t.Fatalf("submit bob: %v", err)
	}
	if _, err := repo.Submit("domain-2", bob.UUID, 1); err != nil {
		t.Fatalf("submit bob other domain: %v", err)
	}
	if _, err := repo.Review(bobReq.ID, true, "admin-1"); err != nil {
		t.Fatalf("approve bob: %v", err)
	}

	all, total, err := repo.ListByDomain("domain-1", "", "", 1, 20)
	if err != nil {
		t.Fatalf("list all: %v", err)
	}
	if total != 2 || len(all) != 2 {
		t.Fatalf("all = %d/%d, want 2/2", len(all), total)
	}

	pending, total, err := repo.ListByDomain("domain-1", model.JoinRequestPending, "", 1, 20)
	if err != nil {
		t.Fatalf("list pending: %v", err)
	}
	if total != 1 || pending[0].UserUUID != alice.UUID {
		t.Fatalf("pending = %+v total=%d", pending, total)
	}

	hit, total, err := repo.ListByDomain("domain-1", "", "爱丽", 1, 20)
	if err != nil {
		t.Fatalf("list keyword: %v", err)
	}
	if total != 1 || hit[0].Name != "alice" || hit[0].DisplayName != "爱丽丝" {
		t.Fatalf("keyword search = %+v total=%d", hit, total)
	}
}

func TestDomainJoinRequestRepository_CountByStatus(t *testing.T) {
	db := setupJoinRequestRepoDB(t)
	repo := NewDomainJoinRequestRepository(db)
	a := seedJoinApplicant(t, db, "a", "A")
	b := seedJoinApplicant(t, db, "b", "B")
	if _, err := repo.Submit("domain-1", a.UUID, 1); err != nil {
		t.Fatalf("submit a: %v", err)
	}
	req, err := repo.Submit("domain-1", b.UUID, 1)
	if err != nil {
		t.Fatalf("submit b: %v", err)
	}
	if _, err := repo.Review(req.ID, false, "admin-1"); err != nil {
		t.Fatalf("reject b: %v", err)
	}
	counts, err := repo.CountByStatus("domain-1")
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if counts[model.JoinRequestPending] != 1 || counts[model.JoinRequestRejected] != 1 {
		t.Fatalf("counts = %+v", counts)
	}
	if counts[model.JoinRequestApproved] != 0 {
		t.Fatalf("approved count = %d, want 0", counts[model.JoinRequestApproved])
	}
}
