package pkg

import (
	"crypto/subtle"
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

// VerifyPassword 校验密码；旧版本遗留的明文密码也以恒定时间比较兼容，直到迁移完成。
func VerifyPassword(stored, password string) bool {
	if strings.HasPrefix(stored, "$2") {
		return bcrypt.CompareHashAndPassword([]byte(stored), []byte(password)) == nil
	}
	if len(stored) == 0 || len(password) == 0 {
		return stored == password
	}
	return subtle.ConstantTimeCompare([]byte(stored), []byte(password)) == 1
}

// IsHashedPassword 判断存储值是否为 bcrypt 哈希。
func IsHashedPassword(stored string) bool {
	return strings.HasPrefix(stored, "$2")
}
