package service

import (
	"fmt"
	"log"
	"strings"

	"github.com/casbin/casbin/v2"
	casbinmodel "github.com/casbin/casbin/v2/model"
	casbinpersist "github.com/casbin/casbin/v2/persist"
)

// Policy subjects are user UUIDs; policy roles are synthetic global names.
// Domain roles include the domain UUID, so identical role names cannot leak
// permissions across domains.
const domainCasbinModel = `
[request_definition]
r = sub, dom, act

[policy_definition]
p = sub, dom, act

[role_definition]
g = _, _

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = g(r.sub, p.sub) && (p.dom == "*" || p.dom == r.dom) && r.act == p.act
`

func domainCasbinModelText() string {
	return strings.TrimSpace(domainCasbinModel)
}

// UseCasbin makes the domain-aware policy engine authoritative for
// HasDomainPermission. The legacy member/role lookup remains as a fallback for
// tests and startup paths that have not opted into Casbin yet.
func (s *DomainService) UseCasbin(adapter casbinpersist.Adapter) error {
	parsedModel, err := casbinmodel.NewModelFromString(domainCasbinModelText())
	if err != nil {
		return fmt.Errorf("load domain casbin model: %w", err)
	}
	enforcer, err := casbin.NewSyncedEnforcer(parsedModel, adapter)
	if err != nil {
		return fmt.Errorf("init domain casbin enforcer: %w", err)
	}
	if err := enforcer.LoadPolicy(); err != nil {
		return fmt.Errorf("load domain casbin policy: %w", err)
	}

	s.memberCacheMu.Lock()
	s.authEnforcer = enforcer
	s.useCasbin = true
	s.memberCacheMu.Unlock()
	return nil
}

// reloadDomainCasbin refreshes policies before an authorization decision. A
// failed refresh fails closed instead of continuing with potentially stale or
// partially loaded rules.
func (s *DomainService) reloadDomainCasbin(enforcer *casbin.SyncedEnforcer) bool {
	if enforcer == nil {
		return false
	}
	if err := enforcer.LoadPolicy(); err != nil {
		log.Printf("[Domain] casbin policy reload failed: %v", err)
		return false
	}
	return true
}
