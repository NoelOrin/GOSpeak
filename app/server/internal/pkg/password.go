package pkg

import (
	"strings"

	"golang.org/x/crypto/bcrypt"
)

// HashPassword 返回 bcrypt 密码哈希。
func HashPassword(password string) (string, error) {
	if password == "" {
		return "", nil
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// VerifyPassword 校验 bcrypt 密码哈希。明文密码兼容路径已退役（2026-11）：
// 存量明文口令无法通过校验，需走重置密码流程。
func VerifyPassword(stored, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(stored), []byte(password)) == nil
}

// IsHashedPassword 判断存储值是否为 bcrypt 哈希。
func IsHashedPassword(stored string) bool {
	return strings.HasPrefix(stored, "$2")
}
