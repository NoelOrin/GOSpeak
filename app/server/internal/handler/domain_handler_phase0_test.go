package handler

import (
	"testing"

	"GOSpeak/internal/model"
)

func TestDomainHandler_Update_NonOwnerForbidden(t *testing.T) {
	db, domainSvc := setupDomainHandlerTestDB(t)
	router := setupDomainHandlerRouter(t, domainSvc)

	g := &model.Domain{Name: "Test", OwnerUUID: "owner-1"}
	if err := db.Create(g).Error; err != nil {
		t.Fatalf("seed domain: %v", err)
	}
	db.Create(&model.DomainMember{DomainUUID: g.UUID, UserUUID: "member-1", RoleName: "member"})

	w := postDomainJSON(t, router, "/api/v1/domain/update", `{"domain_uuid":"`+g.UUID+`","name":"Hacked"}`, map[string]string{"X-User-UUID": "member-1"})
	resp := parseDomainResp(t, w.Body.String())
	if code := intCode(resp["code"]); code != 1013 {
		t.Fatalf("expected code 1013, got %d", code)
	}
}

func TestDomainHandler_Update_OwnerSuccess(t *testing.T) {
	db, domainSvc := setupDomainHandlerTestDB(t)
	router := setupDomainHandlerRouter(t, domainSvc)

	g := &model.Domain{Name: "Test", OwnerUUID: "owner-1"}
	if err := db.Create(g).Error; err != nil {
		t.Fatalf("seed domain: %v", err)
	}

	w := postDomainJSON(t, router, "/api/v1/domain/update", `{"domain_uuid":"`+g.UUID+`","name":"Updated"}`, map[string]string{"X-User-UUID": "owner-1"})
	resp := parseDomainResp(t, w.Body.String())
	if code := intCode(resp["code"]); code != 0 {
		t.Fatalf("expected code 0, got %d: %s", code, resp["msg"])
	}
}

func TestDomainHandler_Delete_CallsHook(t *testing.T) {
	db, domainSvc := setupDomainHandlerTestDB(t)
	h := NewDomainHandler(domainSvc, nil)
	var deleted string
	h.SetOnDomainDelete(func(uuid string) { deleted = uuid })

	router := setupDomainHandlerRouter(t, domainSvc)
	router.POST("/api/v1/domain/delete-with-hook", h.Delete)

	g := &model.Domain{Name: "Test", OwnerUUID: "owner-1"}
	if err := db.Create(g).Error; err != nil {
		t.Fatalf("seed domain: %v", err)
	}

	w := postDomainJSON(t, router, "/api/v1/domain/delete-with-hook", `{"domain_uuid":"`+g.UUID+`"}`, map[string]string{"X-User-UUID": "owner-1"})
	resp := parseDomainResp(t, w.Body.String())
	if code := intCode(resp["code"]); code != 0 {
		t.Fatalf("expected code 0, got %d: %s", code, resp["msg"])
	}
	if deleted != g.UUID {
		t.Fatalf("expected hook called with %q, got %q", g.UUID, deleted)
	}
}

func TestDomainHandler_Kick_AdminSuccess(t *testing.T) {
	db, domainSvc := setupDomainHandlerTestDB(t)
	router := setupDomainHandlerRouter(t, domainSvc)

	g := &model.Domain{Name: "Test", OwnerUUID: "owner-1"}
	if err := db.Create(g).Error; err != nil {
		t.Fatalf("seed domain: %v", err)
	}
	db.Create(&model.DomainMember{DomainUUID: g.UUID, UserUUID: "admin-1", RoleName: "admin"})
	db.Create(&model.DomainMember{DomainUUID: g.UUID, UserUUID: "member-2", RoleName: "member"})

	w := postDomainJSON(t, router, "/api/v1/domain/kick", `{"domain_uuid":"`+g.UUID+`","user_uuid":"member-2"}`, map[string]string{"X-User-UUID": "admin-1"})
	resp := parseDomainResp(t, w.Body.String())
	if code := intCode(resp["code"]); code != 0 {
		t.Fatalf("expected code 0, got %d: %s", code, resp["msg"])
	}
}
