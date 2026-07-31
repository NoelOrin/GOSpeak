package model

import (
	"testing"

	"GOSpeak/internal/permcode"
)

func TestDefaultPermissionsContainGuildCodes(t *testing.T) {
	codes := make(map[string]struct{}, len(DefaultPermissions))
	for _, p := range DefaultPermissions {
		codes[p.Code] = struct{}{}
	}
	for _, code := range []string{
		permcode.PermGuildCreate,
		permcode.PermGuildRead,
		permcode.PermGuildManage,
		permcode.PermGuildDelete,
		permcode.PermGuildInvite,
		permcode.PermGuildKick,
		permcode.PermGuildRoleManage,
	} {
		if _, ok := codes[code]; !ok {
			t.Errorf("DefaultPermissions missing %q", code)
		}
	}
}

func TestDefaultRolePermissionsContainGuildCodes(t *testing.T) {
	admin := make(map[string]struct{}, len(DefaultRolePermissions["admin"]))
	for _, code := range DefaultRolePermissions["admin"] {
		admin[code] = struct{}{}
	}
	for _, code := range []string{
		permcode.PermGuildCreate,
		permcode.PermGuildRead,
		permcode.PermGuildManage,
		permcode.PermGuildDelete,
		permcode.PermGuildInvite,
		permcode.PermGuildKick,
		permcode.PermGuildRoleManage,
	} {
		if _, ok := admin[code]; !ok {
			t.Errorf("admin role missing %q", code)
		}
	}

	user := make(map[string]struct{}, len(DefaultRolePermissions["user"]))
	for _, code := range DefaultRolePermissions["user"] {
		user[code] = struct{}{}
	}
	if _, ok := user[permcode.PermGuildCreate]; !ok {
		t.Error("user role missing guild:create")
	}
}
