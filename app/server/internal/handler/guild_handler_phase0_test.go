package handler

import (
	"testing"

	"GOSpeak/internal/model"
)

func TestGuildHandler_Update_NonOwnerForbidden(t *testing.T) {
	db, guildSvc := setupGuildHandlerTestDB(t)
	router := setupGuildHandlerRouter(t, guildSvc)

	g := &model.Guild{Name: "Test", OwnerUUID: "owner-1"}
	if err := db.Create(g).Error; err != nil {
		t.Fatalf("seed guild: %v", err)
	}
	db.Create(&model.GuildMember{GuildUUID: g.UUID, UserUUID: "member-1", RoleName: "member"})

	w := postGuildJSON(t, router, "/api/v1/guild/update", `{"uuid":"`+g.UUID+`","name":"Hacked"}`, map[string]string{"X-User-UUID": "member-1"})
	resp := parseGuildResp(t, w.Body.String())
	if code := intCode(resp["code"]); code != 1013 {
		t.Fatalf("expected code 1013, got %d", code)
	}
}

func TestGuildHandler_Update_OwnerSuccess(t *testing.T) {
	db, guildSvc := setupGuildHandlerTestDB(t)
	router := setupGuildHandlerRouter(t, guildSvc)

	g := &model.Guild{Name: "Test", OwnerUUID: "owner-1"}
	if err := db.Create(g).Error; err != nil {
		t.Fatalf("seed guild: %v", err)
	}

	w := postGuildJSON(t, router, "/api/v1/guild/update", `{"uuid":"`+g.UUID+`","name":"Updated"}`, map[string]string{"X-User-UUID": "owner-1"})
	resp := parseGuildResp(t, w.Body.String())
	if code := intCode(resp["code"]); code != 0 {
		t.Fatalf("expected code 0, got %d: %s", code, resp["msg"])
	}
}

func TestGuildHandler_Delete_CallsHook(t *testing.T) {
	db, guildSvc := setupGuildHandlerTestDB(t)
	h := NewGuildHandler(guildSvc, nil)
	var deleted string
	h.SetOnGuildDelete(func(uuid string) { deleted = uuid })

	router := setupGuildHandlerRouter(t, guildSvc)
	router.POST("/api/v1/guild/delete-with-hook", h.Delete)

	g := &model.Guild{Name: "Test", OwnerUUID: "owner-1"}
	if err := db.Create(g).Error; err != nil {
		t.Fatalf("seed guild: %v", err)
	}

	w := postGuildJSON(t, router, "/api/v1/guild/delete-with-hook", `{"uuid":"`+g.UUID+`"}`, map[string]string{"X-User-UUID": "owner-1"})
	resp := parseGuildResp(t, w.Body.String())
	if code := intCode(resp["code"]); code != 0 {
		t.Fatalf("expected code 0, got %d: %s", code, resp["msg"])
	}
	if deleted != g.UUID {
		t.Fatalf("expected hook called with %q, got %q", g.UUID, deleted)
	}
}

func TestGuildHandler_Kick_AdminSuccess(t *testing.T) {
	db, guildSvc := setupGuildHandlerTestDB(t)
	router := setupGuildHandlerRouter(t, guildSvc)

	g := &model.Guild{Name: "Test", OwnerUUID: "owner-1"}
	if err := db.Create(g).Error; err != nil {
		t.Fatalf("seed guild: %v", err)
	}
	db.Create(&model.GuildMember{GuildUUID: g.UUID, UserUUID: "admin-1", RoleName: "admin"})
	db.Create(&model.GuildMember{GuildUUID: g.UUID, UserUUID: "member-2", RoleName: "member"})

	w := postGuildJSON(t, router, "/api/v1/guild/kick", `{"guild_uuid":"`+g.UUID+`","user_uuid":"member-2"}`, map[string]string{"X-User-UUID": "admin-1"})
	resp := parseGuildResp(t, w.Body.String())
	if code := intCode(resp["code"]); code != 0 {
		t.Fatalf("expected code 0, got %d: %s", code, resp["msg"])
	}
}
