package repository

import (
	"testing"

	"GOSpeak/internal/model"
)

func TestDomainRepo_ListPublicKeyword(t *testing.T) {
	db := newDomainTestDB(t)
	repo := NewDomainRepository(db)

	publicAlpha := &model.Domain{Name: "Alpha Server", Description: "第一支战队", OwnerUUID: "owner-1", IsPublic: true}
	publicBeta := &model.Domain{Name: "Beta Server", Description: "第二支战队", OwnerUUID: "owner-1", IsPublic: true}
	privateAlpha := &model.Domain{Name: "Alpha Private", OwnerUUID: "owner-1", IsPublic: false}
	for _, g := range []*model.Domain{publicAlpha, publicBeta, privateAlpha} {
		if err := repo.Create(g); err != nil {
			t.Fatalf("seed domain: %v", err)
		}
	}

	domains, total, err := repo.ListPublic(1, 10, "Alpha")
	if err != nil {
		t.Fatalf("ListPublic keyword error: %v", err)
	}
	if total != 1 || len(domains) != 1 || domains[0].Name != "Alpha Server" {
		t.Fatalf("expected public Alpha Server only, got total=%d names=%v", total, domainNames(domains))
	}
}

func domainNames(domains []model.Domain) []string {
	names := make([]string, 0, len(domains))
	for _, g := range domains {
		names = append(names, g.Name)
	}
	return names
}
