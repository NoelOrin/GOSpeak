package service

import "testing"

func TestGuildService_ListPublic_Keyword(t *testing.T) {
	svc, db := setupGuildServiceTestDB(t)
	publicAlpha := seedGuildOwner(t, db, "Alpha Server", "owner-1")
	publicAlpha.IsPublic = true
	if err := svc.guildRepo.Update(publicAlpha); err != nil {
		t.Fatalf("update public guild: %v", err)
	}
	publicBeta := seedGuildOwner(t, db, "Beta Server", "owner-2")
	publicBeta.IsPublic = true
	if err := svc.guildRepo.Update(publicBeta); err != nil {
		t.Fatalf("update public guild: %v", err)
	}
	seedGuildOwner(t, db, "Alpha Private", "owner-3")

	guilds, total, err := svc.ListPublic(1, 10, "Alpha")
	if err != nil {
		t.Fatalf("ListPublic error: %v", err)
	}
	if total != 1 || len(guilds) != 1 || guilds[0].Name != "Alpha Server" {
		t.Fatalf("expected public Alpha Server only, got total=%d names=%v", total, guilds)
	}
}
