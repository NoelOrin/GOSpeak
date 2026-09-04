package pkg

import "testing"

func TestTokenTypesAreDistinct(t *testing.T) {
	access, err := GenerateToken("alice", "Alice", "uuid-alice", "user", 1)
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}
	accessClaims, err := ParseToken(access)
	if err != nil {
		t.Fatalf("parse access token: %v", err)
	}
	if accessClaims.TokenType != AccessTokenType {
		t.Fatalf("expected access token type %q, got %q", AccessTokenType, accessClaims.TokenType)
	}
	if IsRefreshToken(accessClaims) {
		t.Fatal("access token must not be accepted as refresh token")
	}

	refresh, err := GenerateRefreshToken("alice", "Alice", "uuid-alice", "user", 1)
	if err != nil {
		t.Fatalf("GenerateRefreshToken: %v", err)
	}
	refreshClaims, err := ParseToken(refresh)
	if err != nil {
		t.Fatalf("parse refresh token: %v", err)
	}
	if refreshClaims.TokenType != RefreshTokenType {
		t.Fatalf("expected refresh token type %q, got %q", RefreshTokenType, refreshClaims.TokenType)
	}
	if !IsRefreshToken(refreshClaims) {
		t.Fatal("refresh token must be accepted as refresh token")
	}

	bot, err := GenerateBotToken("bot", "Bot", "uuid-bot", "bot", 1, []string{"signal:kick"}, false)
	if err != nil {
		t.Fatalf("GenerateBotToken: %v", err)
	}
	botClaims, err := ParseToken(bot)
	if err != nil {
		t.Fatalf("parse bot token: %v", err)
	}
	if botClaims.TokenType != BotTokenType {
		t.Fatalf("expected bot token type %q, got %q", BotTokenType, botClaims.TokenType)
	}
	if IsRefreshToken(botClaims) {
		t.Fatal("bot token must not be accepted as refresh token")
	}
}
