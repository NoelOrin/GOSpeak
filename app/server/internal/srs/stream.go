package srs

import (
	"crypto/sha256"
	"fmt"
	"math/big"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const streamNamePrefix = "gs-"
const streamTokenTTL = 2 * time.Hour

type streamTokenClaims struct {
	Stream string `json:"stream"`
	jwt.RegisteredClaims
}

func GenerateStreamName(room, identity string) string {
	h := sha256.Sum256([]byte(room + ":" + identity))
	return streamNamePrefix + base36(h[:])[:12]
}

func GenerateStreamToken(stream, secret string) (string, error) {
	if secret == "" {
		return "", errSecretRequired
	}
	claims := streamTokenClaims{
		Stream: stream,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(streamTokenTTL)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

func ValidateStreamToken(stream, token, secret string) bool {
	if secret == "" || token == "" || stream == "" {
		return false
	}
	parsed, err := jwt.ParseWithClaims(token, &streamTokenClaims{}, func(t *jwt.Token) (interface{}, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return []byte(secret), nil
	})
	if err != nil || !parsed.Valid {
		return false
	}
	claims, ok := parsed.Claims.(*streamTokenClaims)
	if !ok || claims.Stream == "" {
		return false
	}
	return claims.Stream == stream
}

func base36(b []byte) string {
	n := new(big.Int).SetBytes(b)
	var out []byte
	mod := big.NewInt(36)
	zero := big.NewInt(0)
	for n.Cmp(zero) > 0 {
		rem := new(big.Int).Mod(n, mod)
		n.Div(n, mod)
		out = append([]byte{toBase36(rem.Int64())}, out...)
	}
	if len(out) == 0 {
		return "0"
	}
	return string(out)
}

func toBase36(v int64) byte {
	if v < 10 {
		return byte('0' + v)
	}
	return byte('a' + v - 10)
}
