// Package policy 定义与传输通道无关的访问策略。
//
// HTTP 中间件（Gin）与 WebSocket 分发层都必须调用同一份 Evaluate，
// 避免「同一身份在 HTTP 被拒、换到 /ws 就放行」这类策略契约分叉。
// 本包只依赖 model/pkg，不依赖 gin、ws 或 signal，可被任意传输层复用。
package policy

import (
	"strings"

	"GOSpeak/internal/model"
	"GOSpeak/internal/pkg"
)

// Action 是一次访问的动作标识：
//   - HTTP 通道传请求路径（以 "/" 开头，例如 "/api/v1/user/list"）
//   - WebSocket 通道传上行事件名（例如 "message:send"）
//   - 建连动作使用 ActionWSConnect
type Action string

// ActionWSConnect 是 WebSocket 握手阶段的策略动作。
const ActionWSConnect Action = "ws:connect"

// Subject 是已鉴权身份的快照，由 JWT claims 或 gin.Context 构造。
type Subject struct {
	UserUUID string
	Username string
	Role     string
}

// SubjectFromClaims 把 JWT claims 转换为策略输入。
func SubjectFromClaims(c *pkg.Claims) Subject {
	if c == nil {
		return Subject{}
	}
	return Subject{
		UserUUID: c.UserUUID,
		Username: c.Username,
		Role:     c.Role,
	}
}

// Decision 是策略判决。Allowed=false 时 Code/Message 可直接用于任意传输通道的响应。
// Terminate 表示命中不可逆状态（封禁），WS 侧应主动断连以同时停止下行广播。
type Decision struct {
	Allowed   bool
	Code      pkg.ErrCode
	Message   string
	Terminate bool
}

// Engine 持有策略判决所需的业务依赖。
// 当前策略只做身份级封禁判决（角色 ban），不需要外部依赖，
// 保留 Engine 是为了让调用方与未来可能引入的域级策略解耦。
// 注意：域级策略尚未实现，见 Evaluate 的说明。
type Engine struct{}

// NewEngine 构造策略引擎。
func NewEngine() *Engine {
	return &Engine{}
}

func allow() Decision {
	return Decision{Allowed: true, Code: pkg.SUCCESS}
}

func deny(code pkg.ErrCode, msg string, terminate bool) Decision {
	return Decision{Allowed: false, Code: code, Message: msg, Terminate: terminate}
}

// CheckBan 只做角色封禁判决，供 BanCheck 等不需要访客依赖的调用点复用。
func CheckBan(sub Subject) Decision {
	if hasBanRole(sub.Role) {
		return deny(pkg.USER_BANNED, "user has been banned", true)
	}
	return allow()
}

// Evaluate 对 subject 做一次访问判决，供 HTTP 与 WS 两个通道共用。
//
// 当前只有身份级封禁一条规则（role == ban），action 与 domainUUID 是预留参数，
// 尚无任何域级规则参与判决：调用方不得假设域级强制已生效，也不得把
// payload 里的 domain_uuid 当作已校验的域上下文。
// 保留完整签名是为了让两个通道共用同一个调用点，将来引入域级策略时无需再改调用方。
func (e *Engine) Evaluate(sub Subject, _ Action, _ string) Decision {
	return CheckBan(sub)
}

// hasBanRole 在 model.HasBanRole（依赖启动期 roleCache）之外补一层字面量判定，
// 避免未走 bootstrap 的嵌入/测试路径因缓存为空而 fail-open 放行 ban 角色。
// User.Role 是单列角色名，不解析逗号分隔的多角色串。
func hasBanRole(role string) bool {
	if role == "" {
		return false
	}
	if model.HasBanRole(role) {
		return true
	}
	return strings.EqualFold(strings.TrimSpace(role), "ban")
}
