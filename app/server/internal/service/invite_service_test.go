package service

import (
	"fmt"
	"testing"
	"time"

	"GOSpeak/internal/model"
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/repository"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

const testStorageKey = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

// setupInviteService 使用共享缓存的匿名内存库，保证事务与并发用例看到同一份数据。
func setupInviteService(t *testing.T) (*InviteService, *gorm.DB) {
	t.Helper()
	dsn := fmt.Sprintf("file:invite_svc_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql db: %v", err)
	}
	sqlDB.SetMaxOpenConns(8)
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := db.AutoMigrate(&model.Domain{}, &model.DomainMember{}, &model.DomainInvite{}, &model.DomainInviteRedemption{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	svc := NewInviteService(
		db,
		repository.NewDomainInviteRepository(db),
		repository.NewDomainRepository(db),
		testStorageKey,
		"jwt-key",
	)
	return svc, db
}

func seedInviteDomain(t *testing.T, db *gorm.DB) *model.Domain {
	t.Helper()
	d := &model.Domain{Name: "InviteRealm", OwnerUUID: "00000000-0000-0000-0000-000000000001"}
	if err := db.Create(d).Error; err != nil {
		t.Fatalf("seed domain: %v", err)
	}
	return d
}

func TestInviteService_Create_GeneratesCode(t *testing.T) {
	svc, db := setupInviteService(t)
	d := seedInviteDomain(t, db)

	invite, err := svc.Create(d.UUID, "测试码", nil, 0, "user-1")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if len(invite.Code) != inviteCodeLength {
		t.Fatalf("code length = %d, want %d", len(invite.Code), inviteCodeLength)
	}
	if invite.ID == 0 {
		t.Fatal("invite must be persisted")
	}
}

func TestInviteService_Create_RejectsMaxUsesOverflow(t *testing.T) {
	svc, db := setupInviteService(t)
	d := seedInviteDomain(t, db)
	if _, err := svc.Create(d.UUID, "", nil, InviteMaxUsesLimit+1, "user-1"); err == nil {
		t.Fatal("max_uses over limit must be rejected")
	}
	if _, err := svc.Create(d.UUID, "", nil, -1, "user-1"); err == nil {
		t.Fatal("negative max_uses must be rejected")
	}
}

func TestInviteService_Create_RejectsPastExpiry(t *testing.T) {
	svc, db := setupInviteService(t)
	d := seedInviteDomain(t, db)
	past := time.Now().Add(-time.Hour)
	if _, err := svc.Create(d.UUID, "", &past, 0, "user-1"); err == nil {
		t.Fatal("expiry in the past must be rejected")
	}
}

func TestInviteService_Create_RejectsUnknownDomain(t *testing.T) {
	svc, _ := setupInviteService(t)
	if _, err := svc.Create("no-such-domain", "", nil, 0, "user-1"); !pkg.IsCode(err, pkg.NOT_FOUND) {
		t.Fatalf("err = %v, want NOT_FOUND", err)
	}
}

func TestInviteService_CreateDefaultForDomain(t *testing.T) {
	svc, db := setupInviteService(t)
	d := seedInviteDomain(t, db)
	invite, err := svc.CreateDefaultForDomain(d.UUID, "user-1")
	if err != nil {
		t.Fatalf("create default: %v", err)
	}
	if invite.Label != "默认邀请码" {
		t.Fatalf("label = %q, want 默认邀请码", invite.Label)
	}
	if invite.MaxUses != 0 || invite.ExpiresAt != nil {
		t.Fatal("default invite must be unlimited and never expire")
	}
}

func TestInviteService_Validate_Success(t *testing.T) {
	svc, db := setupInviteService(t)
	d := seedInviteDomain(t, db)
	invite, err := svc.Create(d.UUID, "", nil, 5, "user-1")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	got, err := svc.Validate(invite.Code)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if got.Domain.UUID != d.UUID {
		t.Fatalf("domain = %s, want %s", got.Domain.UUID, d.UUID)
	}
	if got.RemainingUses != 5 {
		t.Fatalf("remaining = %d, want 5", got.RemainingUses)
	}
}

func TestInviteService_Validate_UnlimitedRemaining(t *testing.T) {
	svc, db := setupInviteService(t)
	d := seedInviteDomain(t, db)
	invite, err := svc.Create(d.UUID, "", nil, 0, "user-1")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	got, err := svc.Validate(invite.Code)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if got.RemainingUses != -1 {
		t.Fatalf("remaining = %d, want -1 (unlimited)", got.RemainingUses)
	}
}

func TestInviteService_Validate_RejectsExpired(t *testing.T) {
	svc, db := setupInviteService(t)
	d := seedInviteDomain(t, db)
	// 直接构造一个已过期的码：绕过 Create 的过去时间校验，用底层编码器。
	key := DeriveInviteKey(testStorageKey, "jwt-key")
	expires := time.Now().Add(-time.Minute)
	raw, err := EncodeInviteCode(key, InviteCode{
		DomainPrefix: DomainPrefixOf(d.UUID), InviteID: 1, ExpiresAt: expires,
	})
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	_, err = svc.Validate(raw)
	if !pkg.IsCode(err, pkg.INVALID_PARAMS) {
		t.Fatalf("err = %v, want INVALID_PARAMS", err)
	}
}

func TestInviteService_Validate_RejectsDisabled(t *testing.T) {
	svc, db := setupInviteService(t)
	d := seedInviteDomain(t, db)
	invite, err := svc.Create(d.UUID, "", nil, 0, "user-1")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := svc.Disable(invite.ID, "user-1"); err != nil {
		t.Fatalf("disable: %v", err)
	}
	if _, err := svc.Validate(invite.Code); err == nil {
		t.Fatal("disabled invite must be rejected")
	}
}

func TestInviteService_Validate_RejectsExhausted(t *testing.T) {
	svc, db := setupInviteService(t)
	d := seedInviteDomain(t, db)
	invite, err := svc.Create(d.UUID, "", nil, 1, "user-1")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := svc.Consume(invite.Code, "u1", true); err != nil {
		t.Fatalf("first consume: %v", err)
	}
	if _, err := svc.Validate(invite.Code); err == nil {
		t.Fatal("exhausted invite must be rejected")
	}
}

func TestInviteService_Validate_RejectsTampered(t *testing.T) {
	svc, db := setupInviteService(t)
	d := seedInviteDomain(t, db)
	invite, err := svc.Create(d.UUID, "", nil, 0, "user-1")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	tampered := []byte(invite.Code)
	if tampered[0] == inviteAlphabet[0] {
		tampered[0] = inviteAlphabet[1]
	} else {
		tampered[0] = inviteAlphabet[0]
	}
	if _, err := svc.Validate(string(tampered)); err == nil {
		t.Fatal("tampered code must be rejected")
	}
}

func TestInviteService_Consume_WritesRedemption(t *testing.T) {
	svc, db := setupInviteService(t)
	d := seedInviteDomain(t, db)
	invite, err := svc.Create(d.UUID, "", nil, 2, "user-1")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	domain, err := svc.Consume(invite.Code, "u1", true)
	if err != nil {
		t.Fatalf("consume: %v", err)
	}
	if domain.UUID != d.UUID {
		t.Fatalf("domain = %s, want %s", domain.UUID, d.UUID)
	}
	var recs []model.DomainInviteRedemption
	if err := db.Find(&recs).Error; err != nil {
		t.Fatalf("query redemptions: %v", err)
	}
	if len(recs) != 1 || !recs[0].Success || recs[0].UserUUID != "u1" {
		t.Fatalf("unexpected redemptions: %+v", recs)
	}
}

func TestInviteService_Consume_FailureRecordsReason(t *testing.T) {
	svc, db := setupInviteService(t)
	d := seedInviteDomain(t, db)
	invite, err := svc.Create(d.UUID, "", nil, 1, "user-1")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := svc.Consume(invite.Code, "u1", true); err != nil {
		t.Fatalf("first consume: %v", err)
	}
	if _, err := svc.Consume(invite.Code, "u2", true); err == nil {
		t.Fatal("second consume must fail")
	}
	var recs []model.DomainInviteRedemption
	if err := db.Order("id ASC").Find(&recs).Error; err != nil {
		t.Fatalf("query redemptions: %v", err)
	}
	if len(recs) != 2 {
		t.Fatalf("redemptions = %d, want 2 (failure must be recorded)", len(recs))
	}
	if recs[1].Success || recs[1].FailReason != model.InviteFailExhausted {
		t.Fatalf("unexpected failure record: %+v", recs[1])
	}
}

func TestInviteService_CrossDomainIsolation(t *testing.T) {
	svc, db := setupInviteService(t)
	d1 := seedInviteDomain(t, db)
	d2 := seedInviteDomain(t, db)
	invite, err := svc.Create(d1.UUID, "", nil, 0, "user-1")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	got, err := svc.Validate(invite.Code)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if got.Domain.UUID == d2.UUID {
		t.Fatal("invite must resolve to its own domain")
	}
}

func TestInviteService_ListAndRedemptions(t *testing.T) {
	svc, db := setupInviteService(t)
	d := seedInviteDomain(t, db)
	if _, err := svc.Create(d.UUID, "a", nil, 0, "u1"); err != nil {
		t.Fatalf("create a: %v", err)
	}
	if _, err := svc.Create(d.UUID, "b", nil, 0, "u1"); err != nil {
		t.Fatalf("create b: %v", err)
	}
	list, err := svc.List(d.UUID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("list length = %d, want 2", len(list))
	}
	items, total, err := svc.ListRedemptions(list[0].ID, 1, 10)
	if err != nil {
		t.Fatalf("list redemptions: %v", err)
	}
	if total != 0 || len(items) != 0 {
		t.Fatalf("redemptions total=%d len=%d, want 0/0", total, len(items))
	}
}

func TestInviteService_Disable_UnknownID(t *testing.T) {
	svc, _ := setupInviteService(t)
	if err := svc.Disable(999, "u1"); !pkg.IsCode(err, pkg.NOT_FOUND) {
		t.Fatalf("err = %v, want NOT_FOUND", err)
	}
}
