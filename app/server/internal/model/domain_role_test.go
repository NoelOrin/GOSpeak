package model

import "testing"

func TestAssignableDomainPermissionsAreScoped(t *testing.T) {
	allowed := AssignableDomainPermissionsSet()
	platformOnly := []string{
		PermUserRead, PermRoleRead, PermSFUManage, PermBotManage,
		PermStorageRead, PermOAuthRead, PermClusterRead,
		PermEmailConfigRead, PermPluginRead, PermDomainCreate, PermDomainDelete,
	}
	for _, code := range platformOnly {
		if _, ok := allowed[code]; ok {
			t.Errorf("platform-only permission %q must not be assignable to domain roles", code)
		}
	}
}

func TestDefaultDomainRolePermissionsUseAssignableSet(t *testing.T) {
	allowed := AssignableDomainPermissionsSet()
	for role, codes := range DefaultDomainRolePermissions {
		if IsSystemDomainRole(role) == false {
			t.Errorf("role %q must be system", role)
		}
		if role == DomainRoleOwner {
			t.Fatalf("owner permissions must not be stored, got %d codes", len(codes))
		}
		for _, code := range codes {
			if _, ok := allowed[code]; !ok {
				t.Errorf("role %q uses non-assignable permission %q", role, code)
			}
		}
	}
}

func TestIsSystemDomainRole(t *testing.T) {
	for _, role := range []string{DomainRoleOwner, DomainRoleAdmin, DomainRoleMember, DomainRoleGuest} {
		if !IsSystemDomainRole(role) {
			t.Errorf("expected %q to be system", role)
		}
	}
	if IsSystemDomainRole("moderator") {
		t.Error("custom role must not be system")
	}
}
