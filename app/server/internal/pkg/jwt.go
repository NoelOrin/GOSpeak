package pkg

import (
	"GOSpeak/internal/authstate"
	"crypto/rand"
	"time"

	"encoding/hex"

	"github.com/golang-jwt/jwt/v5"
	"github.com/oklog/ulid/v2"
)

// Claims JWT 载荷。TokenVersion 绑定用户态的 token 版本号，改密/重置后递增使旧 token 失效。
const (
	AccessTokenType  = "access"
	RefreshTokenType = "refresh"
	BotTokenType     = "bot"
)

type Claims struct {
	Username      string   `json:"username"`
	DisplayName   string   `json:"display_name"`
	UserUUID      string   `json:"user_uuid"`
	Role          string   `json:"role"`
	TokenVersion  uint     `json:"token_version"`
	RefreshFamily string   `json:"refresh_family,omitempty"`
	Permissions   []string `json:"permissions,omitempty"`
	TokenType     string   `json:"token_type,omitempty"`
	jwt.RegisteredClaims
}

// AccessTokenTTL 用户 access_token 有效期（配合 refresh_token 做无感刷新），由装配处按 config 注入。
var AccessTokenTTL = 15 * time.Minute

// RefreshTokenTTL refresh_token 有效期，由装配处按 config 注入。
var RefreshTokenTTL = 7 * 24 * time.Hour

// AccessTokenExpiresIn access token 有效期（秒），供响应体 expires_in 使用。
func AccessTokenExpiresIn() int64 {
	return int64(AccessTokenTTL / time.Second)
}

// GenerateToken 签发 access_token（15m）。tokenVersion 应来自当前用户的 TokenVersion 字段。
func GenerateToken(username, displayName, userUUID, role string, tokenVersion uint) (string, error) {
	claims := Claims{
		Username:     username,
		DisplayName:  displayName,
		UserUUID:     userUUID,
		Role:         role,
		TokenVersion: tokenVersion,
		TokenType:    AccessTokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(AccessTokenTTL)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			ID:        newJTI(),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(authstate.GetSigningKey())
}

// ParseToken 解析并校验 JWT Token，依次尝试当前密钥和历史密钥。
func ParseToken(tokenStr string) (*Claims, error) {
	keys := authstate.GetAllSigningKeys()
	if len(keys) == 0 {
		return nil, jwt.ErrSignatureInvalid
	}

	var lastErr error
	for _, key := range keys {
		token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(token *jwt.Token) (interface{}, error) {
			return key, nil
		}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
		if err == nil {
			if claims, ok := token.Claims.(*Claims); ok && token.Valid {
				return claims, nil
			}
		}
		lastErr = err
	}
	if lastErr == nil {
		lastErr = jwt.ErrSignatureInvalid
	}
	return nil, lastErr
}

// GenerateRefreshFamily 为一次登录生成不可变 family ID。
func GenerateRefreshFamily() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// GenerateRefreshTokenWithFamily 签发 refresh_token（7d），并携带指定 family。
// family 为空时自动生成新 family，用于登录首次签发。
func GenerateRefreshTokenWithFamily(username, displayName, userUUID, role string, tokenVersion uint, family string) (string, error) {
	if family == "" {
		var err error
		family, err = GenerateRefreshFamily()
		if err != nil {
			return "", err
		}
	}
	claims := Claims{
		Username:      username,
		DisplayName:   displayName,
		UserUUID:      userUUID,
		Role:          role,
		TokenVersion:  tokenVersion,
		RefreshFamily: family,
		TokenType:     RefreshTokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(RefreshTokenTTL)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			ID:        newJTI(),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(authstate.GetSigningKey())
}

// GenerateRefreshToken 签发 refresh_token（7d）。tokenVersion 与 access_token 一致。
func GenerateRefreshToken(username, displayName, userUUID, role string, tokenVersion uint) (string, error) {
	return GenerateRefreshTokenWithFamily(username, displayName, userUUID, role, tokenVersion, "")
}

// IsRefreshToken 判断 claims 是否为 refresh token。
func IsRefreshToken(claims *Claims) bool {
	return claims != nil && claims.TokenType == RefreshTokenType
}

func IsTokenExpired(claims *Claims) bool {
	if claims == nil {
		return true
	}
	if claims.ExpiresAt == nil {
		return claims.TokenType != BotTokenType
	}
	return time.Now().Unix() > claims.ExpiresAt.Unix()
}

// newJTI 生成不可预测的 JWT ID，供黑名单/重放防护使用。使用 crypto/rand 而非 math/rand。
func newJTI() string {
	return ulid.MustNew(ulid.Timestamp(time.Now()), rand.Reader).String()
}

// GenerateBotToken 签发 Bot token。如果 isPermanent 为 true，token 不设置 exp。
func GenerateBotToken(username, displayName, userUUID, role string, tokenVersion uint, permissions []string, isPermanent bool) (string, error) {
	claims := Claims{
		Username:     username,
		DisplayName:  displayName,
		UserUUID:     userUUID,
		Role:         role,
		TokenVersion: tokenVersion,
		Permissions:  permissions,
		TokenType:    BotTokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			ID:        newJTI(),
		},
	}
	if isPermanent {
		// Omit exp so the JWT lifetime matches the nullable database expiry.
	} else {
		claims.RegisteredClaims.ExpiresAt = jwt.NewNumericDate(time.Now().Add(24 * time.Hour))
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(authstate.GetSigningKey())
}
