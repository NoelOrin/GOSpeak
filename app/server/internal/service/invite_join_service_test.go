package service

import (
	"testing"
	"time"

	"GOSpeak/internal/model"
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/repository"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupInviteJoinService(t *testing.T) (*InviteJoinService, *InviteService, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&model.User{}, &model.Domain{}, &model.DomainMember{},
		&model.DomainRole{}, &model.DomainRolePermission{},
		&model.DomainInvite{}, &model.DomainInviteRedemption{},
		&model.DomainJoinRequest{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	domainRepo := repository.NewDomainRepository(db)
	inviteRepo := repository.NewDomainInviteRepository(db)
	inviteSvc := NewInviteService(inviteRepo, domainRepo, testStorageKey, "jwt-key")
	joinReqSvc := NewJoinRequestService(repository.NewDBTxRunner(db), repository.NewDomainJoinRequestRepository(db), domainRepo, repository.NewUserRepository(db))
	svc := NewInviteJoinService(
		repository.NewDBTxRunner(db), inviteSvc, joinReqSvc, inviteRepo,
		domainRepo, repository.NewUserRepository(db),
		nil, nil,
	)
	return svc, inviteSvc, db
}

func seedJoinInviteDomain(t *testing.T, db *gorm.DB, requireApproval bool) (*model.Domain, *model.DomainInvite) {
	t.Helper()
	d := &model.Domain{Name: "JoinRealm", OwnerUUID: "owner-1", RequireJoinApproval: requireApproval}
	if err := db.Create(d).Error; err != nil {
		t.Fatalf("seed domain: %v", err)
	}
	key := DeriveInviteKey(testStorageKey, "jwt-key")
	invite := &model.DomainInvite{DomainUUID: d.UUID, Label: "测试", MaxUses: 0, CreatedBy: "owner-1"}
	if err := db.Create(invite).Error; err != nil {
		t.Fatalf("seed invite: %v", err)
	}
	code, err := EncodeInviteCode(key, InviteCode{
		DomainPrefix: DomainPrefixOf(d.UUID), InviteID: uint32(invite.ID),
	})
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	invite.Code = code
	if err := db.Save(invite).Error; err != nil {
		t.Fatalf("save code: %v", err)
	}
	return d, invite
}

func TestInviteJoinService_RegisterAndJoin_DirectJoin(t *testing.T) {
	svc, _, db := setupInviteJoinService(t)
	d, invite := seedJoinInviteDomain(t, db, false)

	res, err := svc.RegisterAndJoin(&InviteRegisterRequest{
		Code: invite.Code, Username: "newbie", Password: "password123", Nickname: "小明",
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if res.Status != JoinStatusJoined {
		t.Fatalf("status = %q, want joined", res.Status)
	}
	if res.Token == "" || res.RefreshToken == "" {
		t.Fatal("tokens must be issued")
	}
	member, err := repository.NewDomainRepository(db).GetMember(d.UUID, res.User.UUID)
	if err != nil {
		t.Fatalf("member missing: %v", err)
	}
	if member.RoleName != model.DomainRoleMember || member.Nickname != "小明" {
		t.Fatalf("unexpected member: %+v", member)
	}
}

func TestInviteJoinService_RegisterAndJoin_PendingWhenApprovalRequired(t *testing.T) {
	svc, _, db := setupInviteJoinService(t)
	d, invite := seedJoinInviteDomain(t, db, true)

	res, err := svc.RegisterAndJoin(&InviteRegisterRequest{
		Code: invite.Code, Username: "waiter", Password: "password123",
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if res.Status != JoinStatusPending {
		t.Fatalf("status = %q, want pending", res.Status)
	}
	if _, err := repository.NewDomainRepository(db).GetMember(d.UUID, res.User.UUID); err != gorm.ErrRecordNotFound {
		t.Fatalf("pending user must not be a member: %v", err)
	}
	req, err := repository.NewDomainJoinRequestRepository(db).GetByDomainUser(d.UUID, res.User.UUID)
	if err != nil {
		t.Fatalf("join request missing: %v", err)
	}
	if req.Status != model.JoinRequestPending || req.InviteID != invite.ID {
		t.Fatalf("unexpected request: %+v", req)
	}
}

func TestInviteJoinService_RegisterAndJoin_ExpiredCodeCreatesNoUser(t *testing.T) {
	svc, _, db := setupInviteJoinService(t)
	_, invite := seedJoinInviteDomain(t, db, false)
	// 把码换成已过期的签名串（MaxUses/InviteID 保持）。
	key := DeriveInviteKey(testStorageKey, "jwt-key")
	expired, err := EncodeInviteCode(key, InviteCode{
		DomainPrefix: DomainPrefixOf(invite.DomainUUID),
		InviteID:     uint32(invite.ID),
		ExpiresAt:    timeNowMinusHour(),
	})
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	_, err = svc.RegisterAndJoin(&InviteRegisterRequest{
		Code: expired, Username: "late", Password: "password123",
	})
	if !pkg.IsCode(err, pkg.INVALID_PARAMS) {
		t.Fatalf("err = %v, want INVALID_PARAMS", err)
	}
	var count int64
	db.Model(&model.User{}).Where("name = ?", "late").Count(&count)
	if count != 0 {
		t.Fatalf("expired code must not create a user, found %d", count)
	}
}

func TestInviteJoinService_RegisterAndJoin_ExhaustedCode(t *testing.T) {
	svc, _, db := setupInviteJoinService(t)
	_, invite := seedJoinInviteDomain(t, db, false)
	invite.MaxUses = 1
	invite.UsedCount = 1
	if err := db.Save(invite).Error; err != nil {
		t.Fatalf("exhaust invite: %v", err)
	}
	_, err := svc.RegisterAndJoin(&InviteRegisterRequest{
		Code: invite.Code, Username: "tooLate", Password: "password123",
	})
	if !pkg.IsCode(err, pkg.INVALID_PARAMS) {
		t.Fatalf("err = %v, want INVALID_PARAMS", err)
	}
}

func TestInviteJoinService_RegisterAndJoin_RejectsDuplicateUsername(t *testing.T) {
	svc, _, db := setupInviteJoinService(t)
	_, invite := seedJoinInviteDomain(t, db, false)
	if err := db.Create(&model.User{Name: "taken"}).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	_, err := svc.RegisterAndJoin(&InviteRegisterRequest{
		Code: invite.Code, Username: "taken", Password: "password123",
	})
	if !pkg.IsCode(err, pkg.USERNAME_EXISTS) {
		t.Fatalf("err = %v, want USERNAME_EXISTS", err)
	}
}

func TestInviteJoinService_Join_ByCodeDirect(t *testing.T) {
	svc, _, db := setupInviteJoinService(t)
	d, invite := seedJoinInviteDomain(t, db, false)
	user := &model.User{Name: "existing"}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	res, err := svc.Join(user.UUID, &InviteJoinRequest{Code: invite.Code})
	if err != nil {
		t.Fatalf("join: %v", err)
	}
	if res.Status != JoinStatusJoined || res.Domain.UUID != d.UUID {
		t.Fatalf("unexpected result: %+v", res)
	}
	if _, err := repository.NewDomainRepository(db).GetMember(d.UUID, user.UUID); err != nil {
		t.Fatalf("member missing: %v", err)
	}
}

func TestInviteJoinService_Join_ByPublicDomainPending(t *testing.T) {
	svc, _, db := setupInviteJoinService(t)
	d, _ := seedJoinInviteDomain(t, db, true)
	d.IsPublic = true
	if err := db.Save(d).Error; err != nil {
		t.Fatalf("make public: %v", err)
	}
	user := &model.User{Name: "publicSeeker"}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	res, err := svc.Join(user.UUID, &InviteJoinRequest{DomainUUID: d.UUID})
	if err != nil {
		t.Fatalf("join: %v", err)
	}
	if res.Status != JoinStatusPending {
		t.Fatalf("status = %q, want pending", res.Status)
	}
}

func TestInviteJoinService_Join_PrivateDomainRejected(t *testing.T) {
	svc, _, db := setupInviteJoinService(t)
	d, _ := seedJoinInviteDomain(t, db, false)
	user := &model.User{Name: "sneaker"}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	_, err := svc.Join(user.UUID, &InviteJoinRequest{DomainUUID: d.UUID})
	if !pkg.IsCode(err, pkg.INVALID_PARAMS) {
		t.Fatalf("err = %v, want INVALID_PARAMS", err)
	}
}

func TestInviteJoinService_Join_AlreadyMemberIsIdempotent(t *testing.T) {
	svc, _, db := setupInviteJoinService(t)
	d, invite := seedJoinInviteDomain(t, db, false)
	user := &model.User{Name: "oldMember"}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	if err := db.Create(&model.DomainMember{DomainUUID: d.UUID, UserUUID: user.UUID, RoleName: model.DomainRoleMember}).Error; err != nil {
		t.Fatalf("seed member: %v", err)
	}
	res, err := svc.Join(user.UUID, &InviteJoinRequest{Code: invite.Code})
	if err != nil {
		t.Fatalf("join: %v", err)
	}
	if res.Status != JoinStatusJoined {
		t.Fatalf("status = %q, want joined", res.Status)
	}
	if invite.UsedCount != 0 {
		t.Fatalf("already-member join must not consume the code, used=%d", invite.UsedCount)
	}
}

func TestInviteJoinService_MyRequestAndCancel(t *testing.T) {
	svc, _, db := setupInviteJoinService(t)
	d, _ := seedJoinInviteDomain(t, db, true)
	d.IsPublic = true
	if err := db.Save(d).Error; err != nil {
		t.Fatalf("make public: %v", err)
	}
	user := &model.User{Name: "canceller"}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	if _, err := svc.Join(user.UUID, &InviteJoinRequest{DomainUUID: d.UUID}); err != nil {
		t.Fatalf("join: %v", err)
	}
	view, err := svc.MyRequest(user.UUID, d.UUID)
	if err != nil {
		t.Fatalf("my request: %v", err)
	}
	if view.Status != model.JoinRequestPending || view.IsMember {
		t.Fatalf("unexpected view: %+v", view)
	}
	if err := svc.CancelRequest(user.UUID, d.UUID); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	after, err := svc.MyRequest(user.UUID, d.UUID)
	if err != nil {
		t.Fatalf("my request after cancel: %v", err)
	}
	if after.Request != nil || after.Status != "" {
		t.Fatalf("cancel must clear the request: %+v", after)
	}
}

func timeNowMinusHour() time.Time {
	return time.Now().Add(-time.Hour).UTC()
}
