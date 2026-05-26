package pkg

import (
	"go_rtc/internal/redis"
	"math/rand"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/oklog/ulid/v2"
)

type Claims struct {
	Username string `json:"username"`
	UserUUID string `json:"user_uuid"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

func GenerateToken(username, userUUID, role string) (string, error) {
	claims := Claims{
		Username: username,
		UserUUID: userUUID,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			ID:        newJTI(),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(redis.GetOrRotateSigningKey())
}

func ParseToken(tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		return redis.GetOrRotateSigningKey(), nil
	})
	if err != nil {
		return nil, err
	}
	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}
	return nil, jwt.ErrSignatureInvalid
}

func GenerateRefreshToken(username, userUUID, role string) (string, error) {
	claims := Claims{
		Username: username,
		UserUUID: userUUID,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(7 * 24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			ID:        newJTI(),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(redis.GetOrRotateSigningKey())
}

func IsTokenExpired(claims *Claims) bool {
	return time.Now().Unix() > claims.ExpiresAt.Unix()
}

func newJTI() string {
	entropy := rand.New(rand.NewSource(time.Now().UnixNano()))
	return ulid.MustNew(ulid.Timestamp(time.Now()), entropy).String()
}
