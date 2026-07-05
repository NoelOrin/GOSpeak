package srs

import (
	"crypto/hmac"
	"crypto/sha256"
	"math/big"
)

const streamNamePrefix = "gs-"

func GenerateStreamName(room, identity string) string {
	h := sha256.Sum256([]byte(room + ":" + identity))
	return streamNamePrefix + base36(h[:])[:12]
}

func GenerateStreamToken(stream, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(stream))
	sum := mac.Sum(nil)
	return base36(sum)[:16]
}

func ValidateStreamToken(stream, token, secret string) bool {
	expected := GenerateStreamToken(stream, secret)
	return hmac.Equal([]byte(expected), []byte(token))
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
