package policy

import (
	"testing"

	"GOSpeak/internal/pkg"
)

// ban 角色必须在 roleCache 未初始化时也能拦住（旧 BanCheck 在此场景 fail-open）。
func TestCheckBan_LiteralFallbackWithoutRoleCache(t *testing.T) {
	d := CheckBan(Subject{Role: "ban"})
	if d.Allowed || d.Code != pkg.USER_BANNED || !d.Terminate {
		t.Fatalf("ban role must be rejected, got %+v", d)
	}
	if d := CheckBan(Subject{Role: "user"}); !d.Allowed {
		t.Fatalf("normal role must pass, got %+v", d)
	}
	if d := CheckBan(Subject{}); !d.Allowed {
		t.Fatalf("empty role must pass, got %+v", d)
	}
}

// 多角色串里只要含 ban 就必须拒绝，大小写与空白不敏感。
func TestCheckBan_MultiRoleString(t *testing.T) {
	for _, role := range []string{"ban", "user,ban", "BAN", " user , Ban "} {
		d := CheckBan(Subject{Role: role})
		if d.Allowed || d.Code != pkg.USER_BANNED {
			t.Errorf("role %q must be rejected, got %+v", role, d)
		}
	}
	for _, role := range []string{"user", "admin", "banned", ""} {
		if d := CheckBan(Subject{Role: role}); !d.Allowed {
			t.Errorf("role %q must pass, got %+v", role, d)
		}
	}
}

// Evaluate 与 CheckBan 必须给出同一份判决，保证 HTTP/WS 两个通道不产生策略分叉。
func TestEvaluate_MatchesCheckBan(t *testing.T) {
	e := NewEngine()
	cases := []Subject{
		{Role: "ban"},
		{Role: "user"},
		{Role: "user,ban"},
		{},
	}
	for _, sub := range cases {
		want := CheckBan(sub)
		got := e.Evaluate(sub, ActionWSConnect, "")
		if got != want {
			t.Errorf("Evaluate(%+v) = %+v, want %+v", sub, got, want)
		}
	}
}

// 建连阶段也必须拦住 ban 角色：/ws 绕过 Gin 中间件链，握手策略是唯一兜底。
func TestEvaluate_ConnectActionRejectsBanRole(t *testing.T) {
	e := NewEngine()
	if d := e.Evaluate(Subject{UserUUID: "banned-1", Role: "ban"}, ActionWSConnect, ""); d.Allowed {
		t.Fatal("banned role must not connect")
	}
	if d := e.Evaluate(Subject{UserUUID: "user-1", Role: "user"}, ActionWSConnect, ""); !d.Allowed {
		t.Fatalf("normal user must connect, got %+v", d)
	}
}

func TestSubjectFromClaims(t *testing.T) {
	if got := SubjectFromClaims(nil); got != (Subject{}) {
		t.Errorf("nil claims must yield empty subject, got %+v", got)
	}
	c := &pkg.Claims{UserUUID: "u1", Username: "alice", Role: "admin"}
	want := Subject{UserUUID: "u1", Username: "alice", Role: "admin"}
	if got := SubjectFromClaims(c); got != want {
		t.Errorf("SubjectFromClaims = %+v, want %+v", got, want)
	}
}
