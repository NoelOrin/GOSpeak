package service

import (
	"strings"
	"testing"

	"GOSpeak/internal/model"
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/repository"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func setupGuestServiceDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}, &model.Domain{}, &model.DomainMember{}, &model.DomainGuestBan{}, &model.DomainRole{}, &model.DomainRolePermission{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func newGuestService(db *gorm.DB, onlineCount func(string) int) *GuestService {
	return NewGuestService(
		db,
		repository.NewUserRepository(db),
		repository.NewDomainRepository(db),
		repository.NewGuestBanRepo(db),
		NewAuthService(repository.NewUserRepository(db), nil, nil),
		onlineCount,
	)
}

func seedGuestDomain(t *testing.T, db *gorm.DB, allowGuest bool) *model.Domain {
	t.Helper()
	d := &model.Domain{Name: "GuestRealm", OwnerUUID: "00000000-0000-0000-0000-000000000001", AllowGuest: allowGuest}
	if err := db.Create(d).Error; err != nil {
		t.Fatalf("seed domain: %v", err)
	}
	return d
}

func seedGuestMember(t *testing.T, db *gorm.DB, d *model.Domain, nickname string) *model.User {
	t.Helper()
	u := &model.User{Name: "seed_guest_" + uuid.NewString()[:12], DisplayName: nickname, IsGuest: true}
	if err := db.Create(u).Error; err != nil {
		t.Fatalf("seed guest user: %v", err)
	}
	m := &model.DomainMember{DomainUUID: d.UUID, UserUUID: u.UUID, Nickname: nickname, RoleName: model.DomainRoleGuest}
	if err := db.Create(m).Error; err != nil {
		t.Fatalf("seed guest member: %v", err)
	}
	return u
}

func TestGuestService_Join(t *testing.T) {
	db := setupGuestServiceDB(t)
	svc := newGuestService(db, nil)

	t.Run("invite code join", func(t *testing.T) {
		d := seedGuestDomain(t, db, true)
		resp, err := svc.Join(&GuestJoinRequest{Nickname: "玩家甲", InviteCode: d.InviteCode})
		if err != nil {
			t.Fatalf("join failed: %v", err)
		}
		if resp.User == nil || !resp.User.IsGuest {
			t.Fatalf("expect guest user: %+v", resp)
		}
		if resp.AccessToken == "" || resp.RefreshToken == "" {
			t.Fatal("expect both tokens")
		}
		if resp.Domain == nil || resp.Domain.UUID != d.UUID {
			t.Fatalf("expect joined domain: %+v", resp.Domain)
		}
	})

	t.Run("public domain no invite code", func(t *testing.T) {
		d := seedGuestDomain(t, db, true)
		d.IsPublic = true
		if err := db.Save(d).Error; err != nil {
			t.Fatal(err)
		}
		resp, err := svc.Join(&GuestJoinRequest{Nickname: "玩家乙", DomainUUID: d.UUID})
		if err != nil {
			t.Fatalf("public domain must accept without code: %v", err)
		}
		if resp.User == nil {
			t.Fatal("expect user")
		}
	})

	t.Run("non-public domain rejected by uuid only", func(t *testing.T) {
		d := seedGuestDomain(t, db, true)
		_, err := svc.Join(&GuestJoinRequest{Nickname: "玩家丙", DomainUUID: d.UUID})
		assertAppErrorCode(t, err, pkg.INVALID_PARAMS)
	})

	t.Run("disabled domain rejected", func(t *testing.T) {
		d := seedGuestDomain(t, db, false)
		_, err := svc.Join(&GuestJoinRequest{Nickname: "x", InviteCode: d.InviteCode})
		assertAppErrorCode(t, err, pkg.FORBIDDEN)
	})

	t.Run("nickname over 24 chars rejected", func(t *testing.T) {
		d := seedGuestDomain(t, db, true)
		_, err := svc.Join(&GuestJoinRequest{Nickname: strings.Repeat("很", 25), InviteCode: d.InviteCode})
		assertAppErrorCode(t, err, pkg.INVALID_PARAMS)
	})

	t.Run("nickname empty rejected", func(t *testing.T) {
		d := seedGuestDomain(t, db, true)
		_, err := svc.Join(&GuestJoinRequest{Nickname: "", InviteCode: d.InviteCode})
		assertAppErrorCode(t, err, pkg.INVALID_PARAMS)
	})

	t.Run("missing code and uuid rejected", func(t *testing.T) {
		_, err := svc.Join(&GuestJoinRequest{Nickname: "x"})
		assertAppErrorCode(t, err, pkg.INVALID_PARAMS)
	})

	t.Run("invite code not found", func(t *testing.T) {
		_, err := svc.Join(&GuestJoinRequest{Nickname: "x", InviteCode: "no-such-code"})
		assertAppErrorCode(t, err, pkg.NOT_FOUND)
	})

	t.Run("guest limit reached", func(t *testing.T) {
		d := seedGuestDomain(t, db, true)
		d.GuestLimit = 2
		if err := db.Save(d).Error; err != nil {
			t.Fatal(err)
		}
		limited := newGuestService(db, func(string) int { return 2 })
		_, err := limited.Join(&GuestJoinRequest{Nickname: "挤不进去", InviteCode: d.InviteCode})
		assertAppErrorCode(t, err, pkg.RATE_LIMITED)

		ok := newGuestService(db, func(string) int { return 1 })
		if _, err := ok.Join(&GuestJoinRequest{Nickname: "挤得进去", InviteCode: d.InviteCode}); err != nil {
			t.Fatalf("below limit must pass: %v", err)
		}
	})
}

func TestGuestService_BanUnban(t *testing.T) {
	db := setupGuestServiceDB(t)
	svc := newGuestService(db, nil)
	d := seedGuestDomain(t, db, true)

	t.Run("ban guest member", func(t *testing.T) {
		u := seedGuestMember(t, db, d, "捣蛋鬼")
		op := "00000000-0000-0000-0000-000000000002"
		if err := svc.Ban(d.UUID, op, u.UUID, "扰乱秩序", 0); err != nil {
			t.Fatalf("ban: %v", err)
		}
		ban := repository.NewGuestBanRepo(db).FindActive(d.UUID, u.UUID)
		if ban == nil || ban.Reason != "扰乱秩序" || ban.ExpiresAt != nil {
			t.Fatalf("expect permanent ban record: %+v", ban)
		}

		// 二次 ban 幂等：更新 reason 与 expires_at，不产生新记录
		if err := svc.Ban(d.UUID, op, u.UUID, "追加", 24); err != nil {
			t.Fatalf("re-ban: %v", err)
		}
		list, err := svc.ListBans(d.UUID)
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		if len(list) != 1 {
			t.Fatalf("expect 1 ban record, got %d", len(list))
		}
		if list[0].Reason != "追加" || list[0].ExpiresAt == nil {
			t.Fatalf("expect updated reason + expiry: %+v", list[0])
		}
	})

	t.Run("ban non-member rejected", func(t *testing.T) {
		err := svc.Ban(d.UUID, "op", "00000000-0000-0000-0000-000000000099", "x", 0)
		assertAppErrorCode(t, err, pkg.NOT_FOUND)
	})

	t.Run("unban removes record", func(t *testing.T) {
		u := seedGuestMember(t, db, d, "回来再玩")
		if err := svc.Ban(d.UUID, "op", u.UUID, "测试", 0); err != nil {
			t.Fatal(err)
		}
		if err := svc.Unban(d.UUID, u.UUID); err != nil {
			t.Fatalf("unban: %v", err)
		}
		if ban := repository.NewGuestBanRepo(db).FindActive(d.UUID, u.UUID); ban != nil {
			t.Fatalf("expect no active ban: %+v", ban)
		}
	})
}
