package service

import (
	"context"
	"strings"
	"testing"

	"GOSpeak/internal/model"
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/repository"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func sanitizeTestName(name string) string {
	return strings.NewReplacer("/", "_", " ", "_").Replace(name)
}

type announcementTestBus struct {
	namespace []string
	rooms     []string
}

func (b *announcementTestBus) PublishNamespace(_ context.Context, event string, _ interface{}) error {
	b.namespace = append(b.namespace, event)
	return nil
}

func (b *announcementTestBus) PublishRoom(_ context.Context, room, event string, _ interface{}) error {
	b.rooms = append(b.rooms, room+":"+event)
	return nil
}

func setupAnnouncementService(t *testing.T) (*AnnouncementService, *gorm.DB, *announcementTestBus) {
	t.Helper()
	dsn := "file:announcement_svc_" + sanitizeTestName(t.Name()) + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.Announcement{}, &model.AnnouncementRead{}, &model.DomainMember{}, &model.User{}); err != nil {
		t.Fatal(err)
	}
	bus := &announcementTestBus{}
	domainRepo := repository.NewDomainRepository(db)
	svc := NewAnnouncementService(repository.NewAnnouncementRepository(db), domainRepo, db)
	svc.SetEventBus(bus)
	svc.SetDomainPermissionChecker(func(domainUUID, userUUID, _ string) bool {
		member, err := domainRepo.GetMember(domainUUID, userUUID)
		if err != nil || member == nil {
			return false
		}
		return member.RoleName == model.DomainRoleOwner || member.RoleName == model.DomainRoleAdmin
	})
	return svc, db, bus
}

func TestAnnouncementService_PlatformPublishBroadcasts(t *testing.T) {
	svc, _, bus := setupAnnouncementService(t)
	publish := true

	dto, err := svc.Create(AnnouncementActor{UserUUID: "admin-uuid", Name: "admin", IsPlatformAdmin: true}, AnnouncementInput{
		ScopeType: model.AnnouncementScopePlatform,
		Title:     "维护公告",
		Content:   "# 维护\n今晚 22:00",
		Level:     model.AnnouncementLevelWarning,
		Publish:   &publish,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if dto.UUID == "" {
		t.Fatal("uuid missing")
	}
	if len(bus.namespace) != 1 || bus.namespace[0] != EventAnnouncementNew {
		t.Fatalf("expected namespace broadcast, got %v", bus.namespace)
	}
}

func TestAnnouncementService_DomainPublishTargetsMembersOnly(t *testing.T) {
	svc, db, bus := setupAnnouncementService(t)
	owner := model.DomainMember{DomainUUID: "dom-1", UserUUID: "owner-uuid", RoleName: model.DomainRoleOwner}
	if err := db.Create(&owner).Error; err != nil {
		t.Fatal(err)
	}
	for _, uuid := range []string{"u-1", "u-2"} {
		if err := db.Create(&model.DomainMember{DomainUUID: "dom-1", UserUUID: uuid}).Error; err != nil {
			t.Fatal(err)
		}
	}
	publish := true

	_, err := svc.Create(AnnouncementActor{UserUUID: "owner-uuid", Name: "owner"}, AnnouncementInput{
		ScopeType: model.AnnouncementScopeDomain,
		ScopeID:   "dom-1",
		Title:     "域公告",
		Content:   "body",
		Publish:   &publish,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if len(bus.namespace) != 0 {
		t.Fatalf("domain announcement must not use namespace broadcast, got %v", bus.namespace)
	}
	if len(bus.rooms) != 3 {
		t.Fatalf("expected 3 per-member deliveries (owner + 2 members), got %v", bus.rooms)
	}
}

func TestAnnouncementService_DraftIsNotDelivered(t *testing.T) {
	svc, _, bus := setupAnnouncementService(t)
	no := false

	if _, err := svc.Create(AnnouncementActor{UserUUID: "admin-uuid", IsPlatformAdmin: true}, AnnouncementInput{
		ScopeType: model.AnnouncementScopePlatform,
		Title:     "草稿",
		Content:   "body",
		Publish:   &no,
	}); err != nil {
		t.Fatalf("create: %v", err)
	}
	if len(bus.namespace) != 0 || len(bus.rooms) != 0 {
		t.Fatal("draft must not be delivered")
	}
}

func TestAnnouncementService_UnreadAndMarkRead(t *testing.T) {
	svc, _, _ := setupAnnouncementService(t)
	publish := true

	dto, err := svc.Create(AnnouncementActor{UserUUID: "admin-uuid", IsPlatformAdmin: true}, AnnouncementInput{
		ScopeType: model.AnnouncementScopePlatform,
		Title:     "公告一",
		Content:   "body",
		Publish:   &publish,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	count, err := svc.UnreadCount("u-1")
	if err != nil {
		t.Fatalf("unread: %v", err)
	}
	if count != 1 {
		t.Fatalf("unread = %d, want 1", count)
	}

	if err := svc.MarkRead("u-1", dto.UUID); err != nil {
		t.Fatalf("mark read: %v", err)
	}
	count, err = svc.UnreadCount("u-1")
	if err != nil {
		t.Fatalf("unread: %v", err)
	}
	if count != 0 {
		t.Fatalf("unread after read = %d, want 0", count)
	}
}

func TestAnnouncementService_MarkReadRejectsInvisible(t *testing.T) {
	svc, _, _ := setupAnnouncementService(t)
	publish := true

	dto, err := svc.Create(AnnouncementActor{UserUUID: "admin-uuid", IsPlatformAdmin: true}, AnnouncementInput{
		ScopeType: model.AnnouncementScopeDomain,
		ScopeID:   "dom-x",
		Title:     "foreign",
		Content:   "body",
		Publish:   &publish,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	err = svc.MarkRead("u-1", dto.UUID)
	if err == nil {
		t.Fatal("expected NOT_FOUND for invisible announcement")
	}
	appErr, ok := err.(*pkg.AppError)
	if !ok || appErr.Code != pkg.NOT_FOUND {
		t.Fatalf("expected NOT_FOUND, got %v", err)
	}
}

func TestAnnouncementService_CanManage(t *testing.T) {
	svc, db, _ := setupAnnouncementService(t)
	if err := db.Create(&model.DomainMember{DomainUUID: "dom-1", UserUUID: "u-1", RoleName: model.DomainRoleAdmin}).Error; err != nil {
		t.Fatal(err)
	}

	if !svc.CanManage(AnnouncementActor{UserUUID: "admin-uuid", IsPlatformAdmin: true}, model.AnnouncementScopePlatform, "") {
		t.Error("platform admin should manage platform announcements")
	}
	if svc.CanManage(AnnouncementActor{UserUUID: "u-1"}, model.AnnouncementScopePlatform, "") {
		t.Error("regular user must not manage platform announcements")
	}
	if !svc.CanManage(AnnouncementActor{UserUUID: "u-1"}, model.AnnouncementScopeDomain, "dom-1") {
		t.Error("domain admin should manage its own domain announcements")
	}
	if svc.CanManage(AnnouncementActor{UserUUID: "u-1"}, model.AnnouncementScopeDomain, "dom-2") {
		t.Error("domain admin must not manage another domain")
	}
}

func TestAnnouncementService_ListMarksReadState(t *testing.T) {
	svc, _, _ := setupAnnouncementService(t)
	publish := true

	first, err := svc.Create(AnnouncementActor{UserUUID: "admin-uuid", IsPlatformAdmin: true}, AnnouncementInput{
		ScopeType: model.AnnouncementScopePlatform,
		Title:     "一",
		Content:   "body",
		Publish:   &publish,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := svc.Create(AnnouncementActor{UserUUID: "admin-uuid", IsPlatformAdmin: true}, AnnouncementInput{
		ScopeType: model.AnnouncementScopePlatform,
		Title:     "二",
		Content:   "body",
		Publish:   &publish,
	}); err != nil {
		t.Fatalf("create: %v", err)
	}

	if err := svc.MarkRead("u-1", first.UUID); err != nil {
		t.Fatalf("mark read: %v", err)
	}

	items, total, err := svc.List("u-1", false, 1, 20)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 2 || len(items) != 2 {
		t.Fatalf("total=%d len=%d", total, len(items))
	}
	readCount := 0
	for _, item := range items {
		if item.IsRead {
			readCount++
		}
	}
	if readCount != 1 {
		t.Fatalf("expected exactly 1 read item, got %d", readCount)
	}
}

func TestAnnouncementService_DeleteRequiresManage(t *testing.T) {
	svc, _, _ := setupAnnouncementService(t)
	publish := true

	dto, err := svc.Create(AnnouncementActor{UserUUID: "admin-uuid", IsPlatformAdmin: true}, AnnouncementInput{
		ScopeType: model.AnnouncementScopePlatform,
		Title:     "待删",
		Content:   "body",
		Publish:   &publish,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if err := svc.Delete(AnnouncementActor{UserUUID: "u-1"}, dto.UUID); err == nil {
		t.Fatal("expected FORBIDDEN for regular user")
	}
	if err := svc.Delete(AnnouncementActor{UserUUID: "admin-uuid", IsPlatformAdmin: true}, dto.UUID); err != nil {
		t.Fatalf("delete: %v", err)
	}

	if _, _, err := svc.List("u-1", false, 1, 20); err != nil {
		t.Fatalf("list: %v", err)
	}
}
