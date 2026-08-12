package handler

import (
	"testing"

	"github.com/gin-gonic/gin"
)

func TestDomainPermissionGranted_DomainRoomUsesDomainPermission(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(nil)
	c.Set("user_uuid", "member-1")
	c.Set("role", "user")

	domainSvc := fakeDomainPermChecker(true)
	if !domainPermissionGranted(c, "domain-a", "room:create", domainSvc, nil) {
		t.Fatal("expected domain permission to grant access")
	}
	if domainSvc.gotDomainUUID != "domain-a" || domainSvc.gotUserUUID != "member-1" || domainSvc.gotPermCode != "room:create" {
		t.Fatalf("unexpected domain permission args: domain=%q user=%q perm=%q", domainSvc.gotDomainUUID, domainSvc.gotUserUUID, domainSvc.gotPermCode)
	}
	if domainPermissionGranted(c, "domain-a", "room:create", fakeDomainPermChecker(false), fakeGlobalPermChecker(true)) {
		t.Fatal("expected missing domain permission to deny access even with global role present")
	}
}

func TestDomainPermissionGranted_PlatformRoomFallsBackToGlobal(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(nil)
	c.Set("user_uuid", "user-1")
	c.Set("role", "admin")

	permSvc := fakeGlobalPermChecker(true)
	if !domainPermissionGranted(c, "", "room:read", nil, permSvc) {
		t.Fatal("expected global permission to grant platform room access")
	}
	if permSvc.gotRole != "admin" || permSvc.gotPermCode != "room:read" {
		t.Fatalf("unexpected global permission args: role=%q perm=%q", permSvc.gotRole, permSvc.gotPermCode)
	}
	if domainPermissionGranted(c, "", "room:read", nil, fakeGlobalPermChecker(false)) {
		t.Fatal("expected missing global permission to deny")
	}
}

func TestDomainPermissionGranted_NilCheckersFailClosed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(nil)
	c.Set("user_uuid", "member-1")
	c.Set("role", "admin")

	if domainPermissionGranted(c, "domain-a", "room:create", nil, fakeGlobalPermChecker(true)) {
		t.Fatal("expected nil domain checker to deny domain access")
	}
	if domainPermissionGranted(c, "", "room:read", fakeDomainPermChecker(true), nil) {
		t.Fatal("expected nil global checker to deny platform access")
	}
}

type fakeDomainPermSvc struct {
	allow          bool
	gotDomainUUID  string
	gotUserUUID    string
	gotPermCode    string
}

func (f *fakeDomainPermSvc) HasDomainPermission(domainUUID, userUUID, permCode string) bool {
	f.gotDomainUUID = domainUUID
	f.gotUserUUID = userUUID
	f.gotPermCode = permCode
	return f.allow
}

func fakeDomainPermChecker(allow bool) *fakeDomainPermSvc { return &fakeDomainPermSvc{allow: allow} }

type fakeGlobalPermSvc struct {
	allow       bool
	gotRole     string
	gotPermCode string
}

func (f *fakeGlobalPermSvc) HasPermission(role, permCode string) bool {
	f.gotRole = role
	f.gotPermCode = permCode
	return f.allow
}

func fakeGlobalPermChecker(allow bool) *fakeGlobalPermSvc { return &fakeGlobalPermSvc{allow: allow} }
