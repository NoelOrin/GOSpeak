package service

import (
	"testing"

	"GOSpeak/internal/model"
	"GOSpeak/internal/repository"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupJoinRequestService(t *testing.T) (*JoinRequestService, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&model.User{}, &model.Domain{}, &model.DomainMember{},
		&model.DomainRole{}, &model.DomainRolePermission{},
		&model.DomainJoinRequest{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	svc := NewJoinRequestService(
		repository.NewDBTxRunner(db),
		repository.NewDomainJoinRequestRepository(db),
		repository.NewDomainRepository(db),
		repository.NewUserRepository(db),
	)
	return svc, db
}

func seedJoinDomain(t *testing.T, db *gorm.DB, requireApproval bool) *model.Domain {
	t.Helper()
	d := &model.Domain{Name: "JoinRealm", OwnerUUID: "owner-1", RequireJoinApproval: requireApproval}
	if err := db.Create(d).Error; err != nil {
		t.Fatalf("seed domain: %v", err)
	}
	return d
}

func seedJoinUser(t *testing.T, db *gorm.DB, name string) *model.User {
	t.Helper()
	u := &model.User{Name: name, DisplayName: name}
	if err := db.Create(u).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	return u
}

func TestJoinRequestService_SubmitCreatesPending(t *testing.T) {
	svc, db := setupJoinRequestService(t)
	d := seedJoinDomain(t, db, true)
	u := seedJoinUser(t, db, "applicant")

	req, err := svc.Submit(d.UUID, u.UUID, 11)
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if req.Status != model.JoinRequestPending || req.InviteID != 11 {
		t.Fatalf("unexpected request: %+v", req)
	}
}

func TestJoinRequestService_SubmitTwiceIsIdempotent(t *testing.T) {
	svc, db := setupJoinRequestService(t)
	d := seedJoinDomain(t, db, true)
	u := seedJoinUser(t, db, "applicant")

	first, err := svc.Submit(d.UUID, u.UUID, 11)
	if err != nil {
		t.Fatalf("first submit: %v", err)
	}
	second, err := svc.Submit(d.UUID, u.UUID, 12)
	if err != nil {
		t.Fatalf("second submit: %v", err)
	}
	if second.ID != first.ID || second.InviteID != 11 {
		t.Fatalf("repeat submit must be idempotent: first=%+v second=%+v", first, second)
	}
}

func TestJoinRequestService_SubmitRejectsMember(t *testing.T) {
	svc, db := setupJoinRequestService(t)
	d := seedJoinDomain(t, db, true)
	u := seedJoinUser(t, db, "member")
	if err := db.Create(&model.DomainMember{DomainUUID: d.UUID, UserUUID: u.UUID, RoleName: model.DomainRoleMember}).Error; err != nil {
		t.Fatalf("seed member: %v", err)
	}
	if _, err := svc.Submit(d.UUID, u.UUID, 0); err != ErrAlreadyMember {
		t.Fatalf("err = %v, want ErrAlreadyMember", err)
	}
}

func TestJoinRequestService_SubmitAfterRejectReopens(t *testing.T) {
	svc, db := setupJoinRequestService(t)
	d := seedJoinDomain(t, db, true)
	u := seedJoinUser(t, db, "applicant")

	req, err := svc.Submit(d.UUID, u.UUID, 11)
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if _, err := svc.Review(d.UUID, req.ID, false, "admin-1"); err != nil {
		t.Fatalf("reject: %v", err)
	}
	reopened, err := svc.Submit(d.UUID, u.UUID, 22)
	if err != nil {
		t.Fatalf("resubmit: %v", err)
	}
	if reopened.Status != model.JoinRequestPending || reopened.InviteID != 22 {
		t.Fatalf("resubmit must reopen with fresh invite: %+v", reopened)
	}
}

func TestJoinRequestService_SubmitAfterApprovedAndLeftReopens(t *testing.T) {
	svc, db := setupJoinRequestService(t)
	d := seedJoinDomain(t, db, true)
	u := seedJoinUser(t, db, "applicant")

	req, err := svc.Submit(d.UUID, u.UUID, 11)
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if _, err := svc.Review(d.UUID, req.ID, true, "admin-1"); err != nil {
		t.Fatalf("approve: %v", err)
	}
	// 曾离开域：成员关系被删除，再次申请必须回到 pending。
	if err := db.Where("domain_uuid = ? AND user_uuid = ?", d.UUID, u.UUID).Delete(&model.DomainMember{}).Error; err != nil {
		t.Fatalf("remove member: %v", err)
	}
	reopened, err := svc.Submit(d.UUID, u.UUID, 33)
	if err != nil {
		t.Fatalf("resubmit: %v", err)
	}
	if reopened.Status != model.JoinRequestPending {
		t.Fatalf("status = %q, want pending", reopened.Status)
	}
}

func TestJoinRequestService_CancelDeletesRecord(t *testing.T) {
	svc, db := setupJoinRequestService(t)
	d := seedJoinDomain(t, db, true)
	u := seedJoinUser(t, db, "applicant")

	if _, err := svc.Submit(d.UUID, u.UUID, 11); err != nil {
		t.Fatalf("submit: %v", err)
	}
	if err := svc.Cancel(d.UUID, u.UUID); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	found, err := svc.Find(d.UUID, u.UUID)
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if found != nil {
		t.Fatalf("cancel must physically delete the record: %+v", found)
	}
}

func TestJoinRequestService_CancelRejectsReviewed(t *testing.T) {
	svc, db := setupJoinRequestService(t)
	d := seedJoinDomain(t, db, true)
	u := seedJoinUser(t, db, "applicant")

	req, err := svc.Submit(d.UUID, u.UUID, 11)
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if _, err := svc.Review(d.UUID, req.ID, false, "admin-1"); err != nil {
		t.Fatalf("reject: %v", err)
	}
	if err := svc.Cancel(d.UUID, u.UUID); err != ErrJoinRequestNotPending {
		t.Fatalf("err = %v, want ErrJoinRequestNotPending", err)
	}
}
