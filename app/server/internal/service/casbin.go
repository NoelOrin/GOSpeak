// Package service 业务逻辑层，协调 repository 和外部服务完成核心业务。
package service

import (
	"fmt"
	"strings"

	"github.com/casbin/casbin/v2/model"
)

const casbinModel = `
[request_definition]
r = sub, act

[policy_definition]
p = sub, act

[role_definition]
g = _, _

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = g(r.sub, p.sub) && r.act == p.act
`

func casbinModelText() string {
	return strings.TrimSpace(casbinModel)
}

func mustCasbinModel() (model.Model, error) {
	return model.NewModelFromString(casbinModelText())
}

func casbinPolicyText(rules [][]string) string {
	lines := make([]string, 0, len(rules))
	for _, rule := range rules {
		if len(rule) == 2 {
			lines = append(lines, fmt.Sprintf("p, %s, %s", rule[0], rule[1]))
		}
	}
	return strings.Join(lines, "\n")
}
