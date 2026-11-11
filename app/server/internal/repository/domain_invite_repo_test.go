package repository

import (
	"fmt"
	"testing"
	"time"

	"GOSpeak/internal/model"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// setupInviteRepoDB 使用共享缓存的匿名内存库：普通 :memory: 每个连接是独立库，
// 并发用例会因连接切换而看不到表。cache=shared 让所有连接共享同一个库。
func setupInviteRepoDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:invite_repo_%d?mode=memory&cache=shared", time.Now().UnixNano())
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
	if err := db.AutoMigrate(&model.DomainInvite{}, &model.DomainInviteRedemption{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestDomainInviteRepository_CreateAndGetByID(t *testing.T) {
	db := setupInviteRepoDB(t)
	repo := NewDomainInviteRepository(db)
	expires := time.Now().Add(time.Hour).UTC().Truncate(time.Second)
	invite := &model.DomainInvite{
		DomainUUID: "domain-1", Label: "默认邀请码", Code: "CODE1",
		ExpiresAt: &expires, MaxUses: 10, CreatedBy: "user-1",
	}
	if err := repo.Create(invite); err != nil {
		t.Fatalf("create: %v", err)
	}
	if invite.ID == 0 {
		t.Fatal("create must assign primary key")
	}

	got, err := repo.GetByID(invite.ID)
	if err != nil {
		t.Fatalf("get by id: %v", err)
	}
	if got.Code != "CODE1" || got.MaxUses != 10 || got.Label != "默认邀请码" {
		t.Fatalf("unexpected invite: %+v", got)
	}
}

func TestDomainInviteRepository_GetByID_NotFound(t *testing.T) {
	db := setupInviteRepoDB(t)
	repo := NewDomainInviteRepository(db)
	if _, err := repo.GetByID(999); err != gorm.ErrRecordNotFound {
		t.Fatalf("err = %v, want gorm.ErrRecordNotFound", err)
	}
}

func TestDomainInviteRepository_ConsumeUse_Unlimited(t *testing.T) {
	db := setupInviteRepoDB(t)
	repo := NewDomainInviteRepository(db)
	invite := &model.DomainInvite{DomainUUID: "d", Code: "C", MaxUses: 0}
	if err := repo.Create(invite); err != nil {
		t.Fatalf("create: %v", err)
	}
	for i := 0; i < 3; i++ {
		ok, err := repo.ConsumeUse(invite.ID)
		if err != nil {
			t.Fatalf("consume %d: %v", i, err)
		}
		if !ok {
			t.Fatalf("consume %d: unlimited invite must always succeed", i)
		}
	}
	got, _ := repo.GetByID(invite.ID)
	if got.UsedCount != 3 {
		t.Fatalf("used count = %d, want 3", got.UsedCount)
	}
}

func TestDomainInviteRepository_ConsumeUse_Exhausts(t *testing.T) {
	db := setupInviteRepoDB(t)
	repo := NewDomainInviteRepository(db)
	invite := &model.DomainInvite{DomainUUID: "d", Code: "C", MaxUses: 2}
	if err := repo.Create(invite); err != nil {
		t.Fatalf("create: %v", err)
	}
	for i := 0; i < 2; i++ {
		ok, err := repo.ConsumeUse(invite.ID)
		if err != nil || !ok {
			t.Fatalf("consume %d: ok=%v err=%v", i, ok, err)
		}
	}
	ok, err := repo.ConsumeUse(invite.ID)
	if err != nil {
		t.Fatalf("consume over limit: %v", err)
	}
	if ok {
		t.Fatal("consume beyond max_uses must report exhausted")
	}
	got, _ := repo.GetByID(invite.ID)
	if got.UsedCount != 2 {
		t.Fatalf("used count = %d, want 2 (over-limit consume must not increment)", got.UsedCount)
	}
}

func TestDomainInviteRepository_ConsumeUse_Disabled(t *testing.T) {
	db := setupInviteRepoDB(t)
	repo := NewDomainInviteRepository(db)
	invite := &model.DomainInvite{DomainUUID: "d", Code: "C"}
	if err := repo.Create(invite); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := repo.Disable(invite.ID, "user-1"); err != nil {
		t.Fatalf("disable: %v", err)
	}
	ok, err := repo.ConsumeUse(invite.ID)
	if err != nil {
		t.Fatalf("consume disabled: %v", err)
	}
	if ok {
		t.Fatal("disabled invite must not be consumable")
	}
}

func TestDomainInviteRepository_Disable_Idempotent(t *testing.T) {
	db := setupInviteRepoDB(t)
	repo := NewDomainInviteRepository(db)
	invite := &model.DomainInvite{DomainUUID: "d", Code: "C"}
	if err := repo.Create(invite); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := repo.Disable(invite.ID, "u1"); err != nil {
		t.Fatalf("first disable: %v", err)
	}
	first, _ := repo.GetByID(invite.ID)
	if first.DisabledAt == nil {
		t.Fatal("disabled_at must be set")
	}
	time.Sleep(2 * time.Millisecond)
	if err := repo.Disable(invite.ID, "u2"); err != nil {
		t.Fatalf("second disable: %v", err)
	}
	second, _ := repo.GetByID(invite.ID)
	if !second.DisabledAt.Equal(*first.DisabledAt) {
		t.Fatal("disable must be idempotent (disabled_at unchanged)")
	}
}

func TestDomainInviteRepository_ListByDomain(t *testing.T) {
	db := setupInviteRepoDB(t)
	repo := NewDomainInviteRepository(db)
	for _, code := range []string{"A", "B"} {
		if err := repo.Create(&model.DomainInvite{DomainUUID: "d1", Code: code}); err != nil {
			t.Fatalf("create %s: %v", code, err)
		}
	}
	if err := repo.Create(&model.DomainInvite{DomainUUID: "d2", Code: "C"}); err != nil {
		t.Fatalf("create d2: %v", err)
	}
	list, err := repo.ListByDomain("d1")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("list length = %d, want 2", len(list))
	}
}

func TestDomainInviteRepository_Redemptions(t *testing.T) {
	db := setupInviteRepoDB(t)
	repo := NewDomainInviteRepository(db)
	invite := &model.DomainInvite{DomainUUID: "d", Code: "C"}
	if err := repo.Create(invite); err != nil {
		t.Fatalf("create: %v", err)
	}
	records := []*model.DomainInviteRedemption{
		{InviteID: invite.ID, DomainUUID: "d", UserUUID: "u1", Code: "C", Success: true},
		{InviteID: invite.ID, DomainUUID: "d", UserUUID: "u2", Code: "C", Success: false, FailReason: "expired"},
	}
	for _, rec := range records {
		if err := repo.CreateRedemption(rec); err != nil {
			t.Fatalf("create redemption: %v", err)
		}
	}
	list, total, err := repo.ListRedemptions(invite.ID, 1, 10)
	if err != nil {
		t.Fatalf("list redemptions: %v", err)
	}
	if total != 2 || len(list) != 2 {
		t.Fatalf("total=%d len=%d, want 2/2", total, len(list))
	}
}

func TestDomainInviteRepository_ConcurrentConsume_NoOversell(t *testing.T) {
	db := setupInviteRepoDB(t)
	repo := NewDomainInviteRepository(db)
	invite := &model.DomainInvite{DomainUUID: "d", Code: "C", MaxUses: 5}
	if err := repo.Create(invite); err != nil {
		t.Fatalf("create: %v", err)
	}

	const workers = 20
	results := make(chan bool, workers)
	errs := make(chan error, workers)
	for i := 0; i < workers; i++ {
		go func() {
			ok, err := repo.ConsumeUse(invite.ID)
			results <- ok
			errs <- err
		}()
	}
	successes := 0
	for i := 0; i < workers; i++ {
		if err := <-errs; err != nil {
			t.Fatalf("concurrent consume: %v", err)
		}
		if <-results {
			successes++
		}
	}
	if successes != 5 {
		t.Fatalf("successes = %d, want exactly 5 (conditional update must prevent oversell)", successes)
	}
	got, _ := repo.GetByID(invite.ID)
	if got.UsedCount != 5 {
		t.Fatalf("used count = %d, want 5", got.UsedCount)
	}
}
