package server

import (
	"testing"

	"GOSpeak/internal/pkg"
	"GOSpeak/internal/policy"
	"GOSpeak/internal/ws"
)

type fakeWSClient struct {
	id     string
	claims *pkg.Claims
}

func (c *fakeWSClient) ID() string                                        { return c.id }
func (c *fakeWSClient) Claims() *pkg.Claims                               { return c.claims }
func (c *fakeWSClient) Send(v interface{}) bool                           { return true }
func (c *fakeWSClient) SendACK(id, event string, data interface{})        {}
func (c *fakeWSClient) SendErrorACK(id, event string, code int, m string) {}
func (c *fakeWSClient) Close()                                            {}

// 回归「策略契约 ↔ 传输通道」：/ws 绕过 Gin 的 BanCheck，
// 逐事件判决必须在每条上行消息上重新执行，不能只在建连时判一次。
func TestNewWSPolicyGuard_BanRoleRejectedOnEveryEvent(t *testing.T) {
	guard := newWSPolicyGuard(policy.NewEngine(), func(clientID string) string { return "dom-1" })
	client := &fakeWSClient{
		id: "conn-1",
		claims: &pkg.Claims{
			UserUUID:  "banned-1",
			Username:  "banned",
			Role:      "ban",
			TokenType: pkg.AccessTokenType,
		},
	}

	for _, event := range []string{"message:send", "private:send", "typing"} {
		d := guard(client, ws.Message{Event: event})
		if d.Code != pkg.USER_BANNED || !d.Terminate {
			t.Errorf("event %q for ban role must be USER_BANNED + terminate, got %+v", event, d)
		}
	}
}

func TestNewWSPolicyGuard_NormalUserUnaffected(t *testing.T) {
	guard := newWSPolicyGuard(policy.NewEngine(), func(string) string { return "" })
	client := &fakeWSClient{id: "conn-1", claims: &pkg.Claims{UserUUID: "user-1", Role: "user"}}

	for _, event := range []string{"private:send", "message:send", "typing"} {
		if d := guard(client, ws.Message{Event: event}); d.Code != pkg.SUCCESS {
			t.Errorf("event %q must pass for normal user, got %+v", event, d)
		}
	}
}

func TestNewWSPolicyGuard_MissingClaimsRejected(t *testing.T) {
	guard := newWSPolicyGuard(policy.NewEngine(), func(string) string { return "" })
	if d := guard(&fakeWSClient{id: "conn-1"}, ws.Message{Event: "typing"}); d.Code != pkg.TOKEN_NOT_EXIST {
		t.Fatalf("connection without claims must be rejected, got %+v", d)
	}
}
