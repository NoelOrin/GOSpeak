package service

import (
	"testing"

	"GOSpeak/internal/model"
	"GOSpeak/internal/permcode"
	"GOSpeak/internal/repository"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func newDomainCasbinTestDB(t *testing.T) (*gorm.DB, *model.Domain, *model.Domain, *DomainService) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&model.User{},
		&model.Domain{},
		&model.DomainMember{},
		&model.DomainRole{},
		&model.DomainRolePermission{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	domainA := &model.Domain{Name: "A", OwnerUUID: "owner-a"}
	domainB := &model.Domain{Name: "B", OwnerUUID: "owner-b"}
	for _, domain := range []*model.Domain{domainA, domainB} {
		if err := db.Create(domain).Error; err != nil {
			t.Fatalf("create domain: %v", err)
		}
		if err := repository.SeedDefaultDomainRoles(db, domain.UUID); err != nil {
			t.Fatalf("seed roles: %v", err)
		}
	}
	users := []model.User{
		{UUID: "platform-admin", Name: "platform-admin", Role: "admin", Status: model.UserStatusActive},
		{UUID: "bot-admin", Name: "bot-admin", Role: "admin", Status: model.UserStatusActive, IsBot: true},
	}
	for i := range users {
		if err := db.Create(&users[i]).Error; err != nil {
			t.Fatalf("create user: %v", err)
		}
	}
	members := []model.DomainMember{
		{DomainUUID: domainA.UUID, UserUUID: "moderator", RoleName: "editor"},
		{DomainUUID: domainA.UUID, UserUUID: "guest", RoleName: model.DomainRoleGuest},
		{DomainUUID: domainB.UUID, UserUUID: "moderator", RoleName: model.DomainRoleGuest},
	}
	for i := range members {
		if err := db.Create(&members[i]).Error; err != nil {
			t.Fatalf("create member: %v", err)
		}
	}
	role := &model.DomainRole{DomainUUID: domainA.UUID, Name: "editor"}
	if err := repository.NewDomainRoleRepository(db).CreateRoleWithPermissions(
		role,
		[]string{permcode.PermRoomUpdate},
	); err != nil {
		t.Fatalf("create editor role: %v", err)
	}

	domainSvc := NewDomainService(repository.NewDomainRepository(db), repository.NewDomainRoleRepository(db))
	if err := domainSvc.UseCasbin(repository.NewDomainCasbinAdapter(db)); err != nil {
		t.Fatalf("use casbin: %v", err)
	}
	return db, domainA, domainB, domainSvc
}

func requireDomainPermission(t *testing.T, svc *DomainService, domainUUID, userUUID, code string, want bool) {
	t.Helper()
	if got := svc.HasDomainPermission(domainUUID, userUUID, code); got != want {
		t.Fatalf("HasDomainPermission(%s, %s, %s) = %v, want %v", domainUUID, userUUID, code, got, want)
	}
}

func TestDomainCasbin_PlatformAdminAndOwner(t *testing.T) {
	_, domainA, domainB, svc := newDomainCasbinTestDB(t)

	for _, code := range []string{
		permcode.PermRoomCreate,
		permcode.PermRoomRead,
		permcode.PermRoomUpdate,
		permcode.PermRoomDelete,
	} {
		requireDomainPermission(t, svc, domainA.UUID, "platform-admin", code, true)
		requireDomainPermission(t, svc, domainB.UUID, "platform-admin", code, true)
	}
	requireDomainPermission(t, svc, domainA.UUID, "platform-admin", permcode.PermMessageSend, false)
	requireDomainPermission(t, svc, domainA.UUID, "bot-admin", permcode.PermRoomDelete, false)

	for _, code := range model.AssignableDomainPermissions {
		requireDomainPermission(t, svc, domainA.UUID, "owner-a", code, true)
	}
	requireDomainPermission(t, svc, domainB.UUID, "owner-a", permcode.PermRoomDelete, false)
}

func TestDomainCasbin_CustomRolesStayScoped(t *testing.T) {
	_, domainA, domainB, svc := newDomainCasbinTestDB(t)

	requireDomainPermission(t, svc, domainA.UUID, "moderator", permcode.PermRoomUpdate, true)
	requireDomainPermission(t, svc, domainA.UUID, "moderator", permcode.PermRoomDelete, false)
	requireDomainPermission(t, svc, domainB.UUID, "moderator", permcode.PermRoomUpdate, false)
	requireDomainPermission(t, svc, domainA.UUID, "guest", permcode.PermRoomRead, true)
	requireDomainPermission(t, svc, domainA.UUID, "outsider", permcode.PermRoomRead, false)
}
