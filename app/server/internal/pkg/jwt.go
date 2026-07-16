package pkg

import (
	"GOSpeak/internal/redis"
	"crypto/rand"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/oklog/ulid/v2"
)

// Claims JWT 载荷。TokenVersion 绑定用户态的 token 版本号，改密/重置后递增使旧 token 失效。
type Claims struct {
	Username     string   `json:"username"`
	DisplayName  string   `json:"display_name"`
	UserUUID     string   `json:"user_uuid"`
	Role         string   `json:"role"`
	TokenVersion uint     `json:"token_version"`
	Permissions  []string `json:"permissions,omitempty"`
	jwt.RegisteredClaims
}

// AccessTokenTTL 用户 access_token 有效期（配合 refresh_token 做无感刷新）。
const AccessTokenTTL = 15 * time.Minute

// RefreshTokenTTL refresh_token 有效期。
const RefreshTokenTTL = 7 * 24 * time.Hour

// GenerateToken 签发 access_token（15m）。tokenVersion 应来自当前用户的 TokenVersion 字段。
func GenerateToken(username, displayName, userUUID, role string, tokenVersion uint) (string, error) {
	claims := Claims{
		Username:     username,
		DisplayName:  displayName,
		UserUUID:     userUUID,
		Role:         role,
		TokenVersion: tokenVersion,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(AccessTokenTTL)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			ID:        newJTI(),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(redis.GetSigningKey())
}

// ParseToken 解析并校验 JWT Token，依次尝试当前密钥和历史密钥。
func ParseToken(tokenStr string) (*Claims, error) {
	keys := redis.GetAllSigningKeys()
	if len(keys) == 0 {
		return nil, jwt.ErrSignatureInvalid
	}

	var lastErr error
	for _, key := range keys {
		token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(token *jwt.Token) (interface{}, error) {
			return key, nil
		})
		if err == nil {
			if claims, ok := token.Claims.(*Claims); ok && token.Valid {
				return claims, nil
			}
		}
		lastErr = err
	}
	return nil, lastErr
}

// GenerateRefreshToken 签发 refresh_token（7d）。tokenVersion 与 access_token 一致。
func GenerateRefreshToken(username, displayName, userUUID, role string, tokenVersion uint) (string, error) {
	claims := Claims{
		Username:     username,
		DisplayName:  displayName,
		UserUUID:     userUUID,
		Role:         role,
		TokenVersion: tokenVersion,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(RefreshTokenTTL)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			ID:        newJTI(),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(redis.GetSigningKey())
}

func IsTokenExpired(claims *Claims) bool {
	return time.Now().Unix() > claims.ExpiresAt.Unix()
}

// newJTI 生成不可预测的 JWT ID，供黑名单/重放防护使用。使用 crypto/rand 而非 math/rand。
func newJTI() string {
	return ulid.MustNew(ulid.Timestamp(time.Now()), rand.Reader).String()
}

// GenerateBotToken 签发 Bot token。如果 isPermanent 为 true，token 永不过期（100年）。
func GenerateBotToken(username, displayName, userUUID, role string, tokenVersion uint, permissions []string, isPermanent bool) (string, error) {
	claims := Claims{
		Username:     username,
		DisplayName:  displayName,
		UserUUID:     userUUID,
		Role:         role,
		TokenVersion: tokenVersion,
		Permissions:  permissions,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			ID:        newJTI(),
		},
	}
	if isPermanent {
		claims.RegisteredClaims.ExpiresAt = jwt.NewNumericDate(time.Date(2125, 1, 1, 0, 0, 0, 0, time.UTC))
	} else {
		claims.RegisteredClaims.ExpiresAt = jwt.NewNumericDate(time.Now().Add(24 * time.Hour))
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(redis.GetSigningKey())
}
