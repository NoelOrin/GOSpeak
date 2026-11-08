package repository

import (
	"testing"

	"GOSpeak/internal/model"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func newDomainTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Domain{}, &model.DomainMember{}, &model.Room{}, &model.User{}, &model.DomainRole{}, &model.DomainRolePermission{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestDomainRepo_Create(t *testing.T) {
	db := newDomainTestDB(t)
	repo := NewDomainRepository(db)

	domain := &model.Domain{Name: "Test Domain", OwnerUUID: "owner-uuid-1"}
	if err := repo.Create(domain); err != nil {
		t.Fatalf("Create error: %v", err)
	}
	if domain.UUID == "" {
		t.Fatal("expected UUID to be set after create")
	}
	if domain.InviteCode == "" {
		t.Fatal("expected InviteCode to be set after create")
	}

	var saved model.Domain
	if err := db.First(&saved, "uuid = ?", domain.UUID).Error; err != nil {
		t.Fatalf("domain not found in DB: %v", err)
	}
	if saved.Name != "Test Domain" {
		t.Fatalf("expected name 'Test Domain', got %q", saved.Name)
	}
}

func TestDomainRepo_GetByUUID(t *testing.T) {
	db := newDomainTestDB(t)
	repo := NewDomainRepository(db)

	domain := &model.Domain{Name: "Test", OwnerUUID: "owner-1"}
	if err := repo.Create(domain); err != nil {
		t.Fatalf("Create error: %v", err)
	}

	got, err := repo.GetByUUID(domain.UUID)
	if err != nil {
		t.Fatalf("GetByUUID error: %v", err)
	}
	if got.Name != "Test" {
		t.Fatalf("expected name 'Test', got %q", got.Name)
	}
}

func TestDomainRepo_GetByInviteCode(t *testing.T) {
	db := newDomainTestDB(t)
	repo := NewDomainRepository(db)

	domain := &model.Domain{Name: "Test", OwnerUUID: "owner-1"}
	if err := repo.Create(domain); err != nil {
		t.Fatalf("Create error: %v", err)
	}

	got, err := repo.GetByInviteCode(domain.InviteCode)
	if err != nil {
		t.Fatalf("GetByInviteCode error: %v", err)
	}
	if got.Name != "Test" {
		t.Fatalf("expected name 'Test', got %q", got.Name)
	}
}

func TestDomainRepo_List(t *testing.T) {
	db := newDomainTestDB(t)
	repo := NewDomainRepository(db)

	// explicit invite codes to avoid collision in rapid creation
	for i, name := range []string{"Domain-A", "Domain-B", "Domain-C", "Domain-D", "Domain-E"} {
		prefix := string(rune('A' + i))
		code := prefix + prefix + "ABCDEF"
		g := &model.Domain{
			Name:       name,
			OwnerUUID:  "owner-1",
			InviteCode: code,
		}
		if err := repo.Create(g); err != nil {
			t.Fatalf("Create error: %v", err)
		}
	}

	domains, total, err := repo.List(1, 3)
	if err != nil {
		t.Fatalf("List error: %v", err)
	}
	if len(domains) != 3 {
		t.Fatalf("expected 3 domains, got %d", len(domains))
	}
	if total != 5 {
		t.Fatalf("expected total=5, got %d", total)
	}

	// page 2
	domains2, _, err := repo.List(2, 3)
	if err != nil {
		t.Fatalf("List page2 error: %v", err)
	}
	if len(domains2) != 2 {
		t.Fatalf("expected 2 domains on page2, got %d", len(domains2))
	}
}

func TestDomainRepo_ListPublic(t *testing.T) {
	db := newDomainTestDB(t)
	repo := NewDomainRepository(db)

	for i := 0; i < 3; i++ {
		g := &model.Domain{
			Name:      "Public-" + string(rune('A'+i)),
			OwnerUUID: "owner-1",
			IsPublic:  true,
		}
		if err := repo.Create(g); err != nil {
			t.Fatalf("Create error: %v", err)
		}
	}
	// private domain
	g := &model.Domain{Name: "Private", OwnerUUID: "owner-1", IsPublic: false}
	if err := repo.Create(g); err != nil {
		t.Fatalf("Create error: %v", err)
	}

	domains, total, err := repo.ListPublic(1, 10, "")
	if err != nil {
		t.Fatalf("ListPublic error: %v", err)
	}
	if total != 3 {
		t.Fatalf("expected total=3, got %d", total)
	}
	if len(domains) != 3 {
		t.Fatalf("expected 3 domains, got %d", len(domains))
	}
}

func TestDomainRepo_AddMember(t *testing.T) {
	db := newDomainTestDB(t)
	repo := NewDomainRepository(db)

	domain := &model.Domain{Name: "Test", OwnerUUID: "owner-1"}
	if err := repo.Create(domain); err != nil {
		t.Fatalf("Create error: %v", err)
	}

	member := &model.DomainMember{
		DomainUUID: domain.UUID,
		UserUUID:   "user-1",
		RoleName:   "member",
	}
	if err := repo.AddMember(member); err != nil {
		t.Fatalf("AddMember error: %v", err)
	}

	var saved model.DomainMember
	if err := db.Where("domain_uuid = ? AND user_uuid = ?", domain.UUID, "user-1").First(&saved).Error; err != nil {
		t.Fatalf("member not found: %v", err)
	}
	if saved.RoleName != "member" {
		t.Fatalf("expected role 'member', got %q", saved.RoleName)
	}
}

func TestDomainRepo_RemoveMember(t *testing.T) {
	db := newDomainTestDB(t)
	repo := NewDomainRepository(db)

	domain := &model.Domain{Name: "Test", OwnerUUID: "owner-1"}
	if err := repo.Create(domain); err != nil {
		t.Fatalf("Create error: %v", err)
	}

	member := &model.DomainMember{DomainUUID: domain.UUID, UserUUID: "user-1"}
	if err := repo.AddMember(member); err != nil {
		t.Fatalf("AddMember error: %v", err)
	}

	if err := repo.RemoveMember(domain.UUID, "user-1"); err != nil {
		t.Fatalf("RemoveMember error: %v", err)
	}

	var count int64
	db.Model(&model.DomainMember{}).Where("domain_uuid = ? AND user_uuid = ?", domain.UUID, "user-1").Count(&count)
	if count != 0 {
		t.Fatal("expected no rows after remove")
	}
}

func TestDomainRepo_GetMember(t *testing.T) {
	db := newDomainTestDB(t)
	repo := NewDomainRepository(db)

	domain := &model.Domain{Name: "Test", OwnerUUID: "owner-1"}
	if err := repo.Create(domain); err != nil {
		t.Fatalf("Create error: %v", err)
	}

	member := &model.DomainMember{DomainUUID: domain.UUID, UserUUID: "user-1", RoleName: "admin"}
	if err := repo.AddMember(member); err != nil {
		t.Fatalf("AddMember error: %v", err)
	}

	got, err := repo.GetMember(domain.UUID, "user-1")
	if err != nil {
		t.Fatalf("GetMember error: %v", err)
	}
	if got.RoleName != "admin" {
		t.Fatalf("expected role 'admin', got %q", got.RoleName)
	}

	_, err = repo.GetMember(domain.UUID, "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent member")
	}
}

func TestDomainRepo_ListMembers(t *testing.T) {
	db := newDomainTestDB(t)
	repo := NewDomainRepository(db)

	domain := &model.Domain{Name: "Test", OwnerUUID: "owner-1"}
	if err := repo.Create(domain); err != nil {
		t.Fatalf("Create error: %v", err)
	}

	for i := 0; i < 3; i++ {
		uid := string(rune('A' + i))
		member := &model.DomainMember{DomainUUID: domain.UUID, UserUUID: uid, RoleName: "member"}
		if err := repo.AddMember(member); err != nil {
			t.Fatalf("AddMember error: %v", err)
		}
	}

	for i := 0; i < 3; i++ {
		uid := string(rune('A' + i))
		user := &model.User{UUID: uid, Name: "user-" + uid, DisplayName: "User " + uid}
		if err := db.Create(user).Error; err != nil {
			t.Fatalf("create user %s: %v", uid, err)
		}
	}

	members, err := repo.ListMembers(domain.UUID)
	if err != nil {
		t.Fatalf("ListMembers error: %v", err)
	}
	if len(members) != 3 {
		t.Fatalf("expected 3 members, got %d", len(members))
	}
	if members[0].Name != "user-A" || members[0].DisplayName != "User A" {
		t.Fatalf("expected member user name, got %q / %q", members[0].Name, members[0].DisplayName)
	}
}

func TestDomainRepo_ListUserDomains(t *testing.T) {
	db := newDomainTestDB(t)
	repo := NewDomainRepository(db)

	g1 := &model.Domain{Name: "G1", OwnerUUID: "o1"}
	if err := repo.Create(g1); err != nil {
		t.Fatalf("Create error: %v", err)
	}
	g2 := &model.Domain{Name: "G2", OwnerUUID: "o2"}
	if err := repo.Create(g2); err != nil {
		t.Fatalf("Create error: %v", err)
	}

	repo.AddMember(&model.DomainMember{DomainUUID: g1.UUID, UserUUID: "user-x"})
	repo.AddMember(&model.DomainMember{DomainUUID: g2.UUID, UserUUID: "user-x"})

	uuids, err := repo.ListUserDomains("user-x")
	if err != nil {
		t.Fatalf("ListUserDomains error: %v", err)
	}
	if len(uuids) != 2 {
		t.Fatalf("expected 2 domain UUIDs, got %d", len(uuids))
	}
}

func TestDomainRepo_CountMembers(t *testing.T) {
	db := newDomainTestDB(t)
	repo := NewDomainRepository(db)

	domain := &model.Domain{Name: "Test", OwnerUUID: "owner-1"}
	if err := repo.Create(domain); err != nil {
		t.Fatalf("Create error: %v", err)
	}

	for i := 0; i < 4; i++ {
		uid := string(rune('A' + i))
		repo.AddMember(&model.DomainMember{DomainUUID: domain.UUID, UserUUID: uid})
	}

	count, err := repo.CountMembers(domain.UUID)
	if err != nil {
		t.Fatalf("CountMembers error: %v", err)
	}
	if count != 4 {
		t.Fatalf("expected 4 members, got %d", count)
	}
}

func TestDomainRepo_Delete(t *testing.T) {
	db := newDomainTestDB(t)
	repo := NewDomainRepository(db)

	domain := &model.Domain{Name: "Test", OwnerUUID: "owner-1"}
	if err := repo.Create(domain); err != nil {
		t.Fatalf("Create error: %v", err)
	}

	if err := repo.Delete(domain.UUID); err != nil {
		t.Fatalf("Delete error: %v", err)
	}

	var count int64
	db.Model(&model.Domain{}).Where("uuid = ?", domain.UUID).Count(&count)
	if count != 0 {
		t.Fatal("expected no rows after delete")
	}
}

func TestDomainRepo_Delete_NotFound(t *testing.T) {
	db := newDomainTestDB(t)
	repo := NewDomainRepository(db)

	// Deleting a non-existent UUID should not error (GORM returns nil for zero rows)
	if err := repo.Delete("nonexistent-uuid"); err != nil {
		t.Fatalf("Delete nonexistent error: %v", err)
	}
}

func TestDomainRepo_Delete_CleansMembersAndRooms(t *testing.T) {
	db := newDomainTestDB(t)
	repo := NewDomainRepository(db)

	domain := &model.Domain{Name: "Test", OwnerUUID: "owner-1"}
	if err := repo.Create(domain); err != nil {
		t.Fatalf("Create error: %v", err)
	}
	if err := repo.AddMember(&model.DomainMember{DomainUUID: domain.UUID, UserUUID: "member-1"}); err != nil {
		t.Fatalf("AddMember error: %v", err)
	}
	if err := db.Create(&model.Room{Name: "room-1", DomainUUID: domain.UUID}).Error; err != nil {
		t.Fatalf("Create room error: %v", err)
	}

	if err := repo.Delete(domain.UUID); err != nil {
		t.Fatalf("Delete error: %v", err)
	}

	var count int64
	db.Model(&model.Domain{}).Where("uuid = ?", domain.UUID).Count(&count)
	if count != 0 {
		t.Fatalf("expected domain removed, count=%d", count)
	}
	db.Model(&model.DomainMember{}).Where("domain_uuid = ?", domain.UUID).Count(&count)
	if count != 0 {
		t.Fatalf("expected members removed, count=%d", count)
	}
	db.Model(&model.Room{}).Where("domain_uuid = ?", domain.UUID).Count(&count)
	if count != 0 {
		t.Fatalf("expected rooms removed, count=%d", count)
	}
}

func TestCreateWithOwner_SeedsDefaultDomainRoles(t *testing.T) {
	db := newDomainTestDB(t)
	if err := db.AutoMigrate(&model.DomainRole{}, &model.DomainRolePermission{}); err != nil {
		t.Fatalf("migrate roles: %v", err)
	}
	repo := NewDomainRepository(db)
	domain := &model.Domain{Name: "Roles", OwnerUUID: "owner-1"}
	owner := &model.DomainMember{DomainUUID: "", UserUUID: "owner-1", RoleName: model.DomainRoleOwner}
	if err := repo.CreateWithOwner(domain, owner); err != nil {
		t.Fatalf("CreateWithOwner: %v", err)
	}

	roleRepo := NewDomainRoleRepository(db)
	roles, err := roleRepo.ListRoles(domain.UUID)
	if err != nil {
		t.Fatalf("ListRoles: %v", err)
	}
	if len(roles) != 4 {
		t.Fatalf("expected 4 seeded roles, got %d", len(roles))
	}
}
