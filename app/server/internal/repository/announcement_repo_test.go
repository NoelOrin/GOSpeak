package repository

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"GOSpeak/internal/model"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupAnnouncementDB(t *testing.T) *gorm.DB {
	t.Helper()
	// 每个测试用独立内存库：共享 DSN 会让用例之间互相看到对方的数据。
	dsn := fmt.Sprintf("file:announcement_%s?mode=memory&cache=shared", strings.NewReplacer("/", "_", " ", "_").Replace(t.Name()))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.Announcement{}, &model.AnnouncementRead{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func publishedAnnouncement(scope, scopeID, title string) *model.Announcement {
	now := time.Now()
	return &model.Announcement{
		ScopeType:   scope,
		ScopeID:     scopeID,
		Title:       title,
		Content:     "body",
		Level:       model.AnnouncementLevelInfo,
		PublishedAt: &now,
	}
}

func TestAnnouncementRepo_ListVisibleFiltersScope(t *testing.T) {
	db := setupAnnouncementDB(t)
	repo := NewAnnouncementRepository(db)

	platform := publishedAnnouncement(model.AnnouncementScopePlatform, "", "platform notice")
	domainMine := publishedAnnouncement(model.AnnouncementScopeDomain, "dom-1", "mine")
	domainOther := publishedAnnouncement(model.AnnouncementScopeDomain, "dom-2", "other")
	for _, a := range []*model.Announcement{platform, domainMine, domainOther} {
		if err := repo.Create(a); err != nil {
			t.Fatal(err)
		}
	}

	items, total, err := repo.ListVisible("user-1", []string{"dom-1"}, false, 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 {
		t.Fatalf("total = %d, want 2", total)
	}
	for _, item := range items {
		if item.Title == "other" {
			t.Fatal("announcement from a foreign domain must not be visible")
		}
	}
}

func TestAnnouncementRepo_ListVisibleExcludesDraftAndExpired(t *testing.T) {
	db := setupAnnouncementDB(t)
	repo := NewAnnouncementRepository(db)

	draft := publishedAnnouncement(model.AnnouncementScopePlatform, "", "draft")
	draft.PublishedAt = nil
	past := time.Now().Add(-time.Hour)
	expired := publishedAnnouncement(model.AnnouncementScopePlatform, "", "expired")
	expired.ExpiresAt = &past
	future := time.Now().Add(time.Hour)
	alive := publishedAnnouncement(model.AnnouncementScopePlatform, "", "alive")
	alive.ExpiresAt = &future

	for _, a := range []*model.Announcement{draft, expired, alive} {
		if err := repo.Create(a); err != nil {
			t.Fatal(err)
		}
	}

	items, total, err := repo.ListVisible("user-1", nil, false, 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(items) != 1 || items[0].Title != "alive" {
		t.Fatalf("got total=%d items=%v", total, items)
	}
}

func TestAnnouncementRepo_UnreadCountAndMarkReadIdempotent(t *testing.T) {
	db := setupAnnouncementDB(t)
	repo := NewAnnouncementRepository(db)

	a1 := publishedAnnouncement(model.AnnouncementScopePlatform, "", "one")
	a2 := publishedAnnouncement(model.AnnouncementScopePlatform, "", "two")
	for _, a := range []*model.Announcement{a1, a2} {
		if err := repo.Create(a); err != nil {
			t.Fatal(err)
		}
	}

	count, err := repo.CountUnread("user-1", nil)
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("unread = %d, want 2", count)
	}

	for i := 0; i < 2; i++ {
		if err := repo.MarkRead(a1.UUID, "user-1"); err != nil {
			t.Fatalf("mark read attempt %d: %v", i, err)
		}
	}

	count, err = repo.CountUnread("user-1", nil)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("unread after duplicate mark = %d, want 1", count)
	}
}

func TestAnnouncementRepo_OnlyUnreadFilter(t *testing.T) {
	db := setupAnnouncementDB(t)
	repo := NewAnnouncementRepository(db)

	a1 := publishedAnnouncement(model.AnnouncementScopePlatform, "", "read me")
	a2 := publishedAnnouncement(model.AnnouncementScopePlatform, "", "leave me")
	for _, a := range []*model.Announcement{a1, a2} {
		if err := repo.Create(a); err != nil {
			t.Fatal(err)
		}
	}
	if err := repo.MarkRead(a1.UUID, "user-1"); err != nil {
		t.Fatal(err)
	}

	items, total, err := repo.ListVisible("user-1", nil, true, 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || items[0].Title != "leave me" {
		t.Fatalf("only_unread filter broken: total=%d", total)
	}
}

func TestAnnouncementRepo_IsVisible(t *testing.T) {
	db := setupAnnouncementDB(t)
	repo := NewAnnouncementRepository(db)

	mine := publishedAnnouncement(model.AnnouncementScopeDomain, "dom-1", "mine")
	other := publishedAnnouncement(model.AnnouncementScopeDomain, "dom-2", "other")
	for _, a := range []*model.Announcement{mine, other} {
		if err := repo.Create(a); err != nil {
			t.Fatal(err)
		}
	}

	ok, err := repo.IsVisible(mine.UUID, []string{"dom-1"})
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("own domain announcement must be visible")
	}

	ok, err = repo.IsVisible(other.UUID, []string{"dom-1"})
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("foreign domain announcement must not be visible")
	}
}

func TestAnnouncementRepo_ListAdminScopedForDomainManager(t *testing.T) {
	db := setupAnnouncementDB(t)
	repo := NewAnnouncementRepository(db)

	domA := publishedAnnouncement(model.AnnouncementScopeDomain, "dom-1", "a")
	domB := publishedAnnouncement(model.AnnouncementScopeDomain, "dom-2", "b")
	myDraft := publishedAnnouncement(model.AnnouncementScopePlatform, "", "my draft")
	myDraft.PublishedAt = nil
	myDraft.CreatedByUUID = "u-1"
	for _, a := range []*model.Announcement{domA, domB, myDraft} {
		if err := repo.Create(a); err != nil {
			t.Fatal(err)
		}
	}

	items, total, err := repo.ListAdmin("u-1", []string{"dom-1"}, false, 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 {
		t.Fatalf("total = %d, want 2 (own domain + own draft)", total)
	}
	for _, item := range items {
		if item.Title == "b" {
			t.Fatal("must not see another domain's announcement")
		}
	}

	_, allTotal, err := repo.ListAdmin("u-1", nil, true, 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	if allTotal != 3 {
		t.Fatalf("platform admin total = %d, want 3", allTotal)
	}
}
