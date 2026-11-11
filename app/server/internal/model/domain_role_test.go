package model

import "testing"

func TestAssignableDomainPermissions_IncludesInvitePermissions(t *testing.T) {
	set := AssignableDomainPermissionsSet()
	for _, want := range []string{PermDomainInviteManage, PermDomainMemberReview} {
		if _, ok := set[want]; !ok {
			t.Fatalf("%s must be assignable", want)
		}
	}
}

func TestDefaultDomainRolePermissions_AdminHoldsInvitePermissions(t *testing.T) {
	codes := DefaultDomainRolePermissions[DomainRoleAdmin]
	for _, want := range []string{PermDomainInviteManage, PermDomainMemberReview} {
		found := false
		for _, code := range codes {
			if code == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("domain admin must hold %s", want)
		}
	}
}

func TestDefaultDomainRolePermissions_MemberLacksInvitePermissions(t *testing.T) {
	for _, code := range DefaultDomainRolePermissions[DomainRoleMember] {
		if code == PermDomainInviteManage || code == PermDomainMemberReview {
			t.Fatalf("member must not hold %s", code)
		}
	}
}
