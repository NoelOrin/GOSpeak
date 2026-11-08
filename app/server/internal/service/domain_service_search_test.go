package service

import "testing"

func TestDomainService_ListPublic_Keyword(t *testing.T) {
	svc, db := setupDomainServiceTestDB(t)
	publicAlpha := seedDomainOwner(t, db, "Alpha Server", "owner-1")
	publicAlpha.IsPublic = true
	if err := svc.domainRepo.Update(publicAlpha); err != nil {
		t.Fatalf("update public domain: %v", err)
	}
	publicBeta := seedDomainOwner(t, db, "Beta Server", "owner-2")
	publicBeta.IsPublic = true
	if err := svc.domainRepo.Update(publicBeta); err != nil {
		t.Fatalf("update public domain: %v", err)
	}
	seedDomainOwner(t, db, "Alpha Private", "owner-3")

	domains, total, err := svc.ListPublic(1, 10, "Alpha")
	if err != nil {
		t.Fatalf("ListPublic error: %v", err)
	}
	if total != 1 || len(domains) != 1 || domains[0].Name != "Alpha Server" {
		t.Fatalf("expected public Alpha Server only, got total=%d names=%v", total, domains)
	}
}
