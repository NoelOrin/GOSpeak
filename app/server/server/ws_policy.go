package server

import (
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/policy"
	"GOSpeak/internal/signal"
	"GOSpeak/internal/ws"
)

// installWSPolicy 把传输无关的 policy.Engine 装配到 /ws 的三个策略执行点：
//  1. 握手阶段（ConnectPolicy）——根 http.Handler 直连 Upgrader，补回 Gin 的 BanCheck；
//  2. 每条上行消息（HandlerRegistry.PolicyGuard）——封禁不失效 JWT，必须实时判决；
//  3. signal 层用 DB 解析出的权威域做二次判决，防止客户端伪造 domain_uuid。
//
// HTTP 侧由 middleware.BanCheck 调用同一个 Engine，两个通道不再有策略分叉。
func installWSPolicy(engine *policy.Engine, registry *ws.HandlerRegistry, upgraderCfg *ws.UpgraderConfig, hub *signal.Hub) {
	upgraderCfg.ConnectPolicy = func(claims *pkg.Claims) (pkg.ErrCode, string) {
		d := engine.Evaluate(policy.SubjectFromClaims(claims), policy.ActionWSConnect, "")
		return d.Code, d.Message
	}

	resolveDomain := func(clientID string) string { return "" }
	if hub != nil {
		resolveDomain = hub.ClientDomain
		hub.SetAccessPolicy(func(sub policy.Subject, action policy.Action, domainUUID string) policy.Decision {
			return engine.Evaluate(sub, action, domainUUID)
		})
	}
	registry.SetPolicyGuard(newWSPolicyGuard(engine, resolveDomain))
}

// newWSPolicyGuard 构造逐事件策略适配器。resolveDomain 必须返回服务端登记过的
// 可信域，绝不解析客户端 payload 里的 domain_uuid。单独抽出便于回归测试。
func newWSPolicyGuard(engine *policy.Engine, resolveDomain func(clientID string) string) ws.PolicyGuard {
	return func(c ws.ClientMessenger, msg ws.Message) ws.PolicyDecision {
		if c == nil || c.Claims() == nil {
			return ws.PolicyDecision{Code: pkg.TOKEN_NOT_EXIST, Message: "unauthorized"}
		}
		domainUUID := ""
		if resolveDomain != nil {
			domainUUID = resolveDomain(c.ID())
		}
		d := engine.Evaluate(policy.SubjectFromClaims(c.Claims()), policy.Action(msg.Event), domainUUID)
		return ws.PolicyDecision{
			Code:      d.Code,
			Message:   d.Message,
			Terminate: d.Terminate,
		}
	}
}
