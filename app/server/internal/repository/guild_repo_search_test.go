package repository

import (
	"testing"

	"GOSpeak/internal/model"
)

func TestGuildRepo_ListPublicKeyword(t *testing.T) {
	db := newGuildTestDB(t)
	repo := NewGuildRepository(db)

	publicAlpha := &model.Guild{Name: "Alpha Server", Description: "第一支战队", OwnerUUID: "owner-1", IsPublic: true}
	publicBeta := &model.Guild{Name: "Beta Server", Description: "第二支战队", OwnerUUID: "owner-1", IsPublic: true}
	privateAlpha := &model.Guild{Name: "Alpha Private", OwnerUUID: "owner-1", IsPublic: false}
	for _, g := range []*model.Guild{publicAlpha, publicBeta, privateAlpha} {
		if err := repo.Create(g); err != nil {
			t.Fatalf("seed guild: %v", err)
		}
	}

	guilds, total, err := repo.ListPublic(1, 10, "Alpha")
	if err != nil {
		t.Fatalf("ListPublic keyword error: %v", err)
	}
	if total != 1 || len(guilds) != 1 || guilds[0].Name != "Alpha Server" {
		t.Fatalf("expected public Alpha Server only, got total=%d names=%v", total, guildNames(guilds))
	}
}

func guildNames(guilds []model.Guild) []string {
	names := make([]string, 0, len(guilds))
	for _, g := range guilds {
		names = append(names, g.Name)
	}
	return names
}
