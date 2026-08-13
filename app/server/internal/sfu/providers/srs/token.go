package srs

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"GOSpeak/internal/pkg"
)

type tokenClaims struct {
	Room     string `json:"room"`
	Identity string `json:"identity"`
	jwt.RegisteredClaims
}

var errSecretRequired = pkg.NewAppError(pkg.SFU_NOT_CONFIGURED, "SRS_SECRET is not configured: set SRS_SECRET to a non-empty value")

func GenerateToken(room, identity, secret string) (string, error) {
	if secret == "" {
		return "", errSecretRequired
	}
	claims := tokenClaims{
		Room:     room,
		Identity: identity,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

func ParseToken(tokenStr, secret string) (room, identity string, err error) {
	if secret == "" {
		return "", "", errSecretRequired
	}
	token, err := jwt.ParseWithClaims(tokenStr, &tokenClaims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(secret), nil
	})
	if err != nil {
		return "", "", fmt.Errorf("parse srs token: %w", err)
	}
	claims, ok := token.Claims.(*tokenClaims)
	if !ok || !token.Valid {
		return "", "", fmt.Errorf("invalid srs token claims")
	}
	if claims.Room == "" || claims.Identity == "" {
		return "", "", fmt.Errorf("srs token missing room or identity")
	}
	return claims.Room, claims.Identity, nil
}
