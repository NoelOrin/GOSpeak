package model

import (
	"testing"

	"GOSpeak/internal/permcode"
)

func TestDefaultPermissionsContainDomainCodes(t *testing.T) {
	codes := make(map[string]struct{}, len(DefaultPermissions))
	for _, p := range DefaultPermissions {
		codes[p.Code] = struct{}{}
	}
	for _, code := range []string{
		permcode.PermDomainCreate,
		permcode.PermDomainRead,
		permcode.PermDomainManage,
		permcode.PermDomainDelete,
		permcode.PermDomainInvite,
		permcode.PermDomainKick,
		permcode.PermDomainRoleManage,
	} {
		if _, ok := codes[code]; !ok {
			t.Errorf("DefaultPermissions missing %q", code)
		}
	}
}

func TestDefaultRolePermissionsContainDomainCodes(t *testing.T) {
	admin := make(map[string]struct{}, len(DefaultRolePermissions["admin"]))
	for _, code := range DefaultRolePermissions["admin"] {
		admin[code] = struct{}{}
	}
	for _, code := range []string{
		permcode.PermDomainCreate,
		permcode.PermDomainRead,
		permcode.PermDomainManage,
		permcode.PermDomainDelete,
		permcode.PermDomainInvite,
		permcode.PermDomainKick,
		permcode.PermDomainRoleManage,
	} {
		if _, ok := admin[code]; !ok {
			t.Errorf("admin role missing %q", code)
		}
	}

	user := make(map[string]struct{}, len(DefaultRolePermissions["user"]))
	for _, code := range DefaultRolePermissions["user"] {
		user[code] = struct{}{}
	}
	if _, ok := user[permcode.PermDomainCreate]; !ok {
		t.Error("user role missing domain:create")
	}
}
